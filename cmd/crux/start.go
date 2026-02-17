package main

import (
	"context"
	"fmt"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/roygabriel/crux/internal/agent"
	"github.com/roygabriel/crux/internal/config"
	"github.com/roygabriel/crux/internal/memory/bank"
	"github.com/roygabriel/crux/internal/memory/journal"
	"github.com/roygabriel/crux/internal/memory/session"
	"github.com/roygabriel/crux/internal/memory/store"
	"github.com/roygabriel/crux/internal/memory/worknotes"
	"github.com/roygabriel/crux/internal/orchestrator"
	"github.com/roygabriel/crux/internal/phase"
	"github.com/roygabriel/crux/internal/plugin"
	"github.com/roygabriel/crux/internal/pluginloader"
	"github.com/roygabriel/crux/internal/security"
	"github.com/roygabriel/crux/internal/tmux"
	"github.com/roygabriel/crux/pkg/types"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the orchestration loop",
	RunE: func(cmd *cobra.Command, args []string) error {
		log := setupLogger()

		log.Info("loading configuration", "path", cfgFile)
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		log.Info("starting orchestration", "project", cfg.Project.Name)

		// SQLite store.
		st, err := store.NewStore(cfg.Memory.SQLitePath)
		if err != nil {
			return fmt.Errorf("open store: %w", err)
		}
		defer st.Close()
		if err := st.Migrate(); err != nil {
			return fmt.Errorf("migrate store: %w", err)
		}

		// Decision journal (vector index is optional).
		j := journal.NewJournal(st, nil)

		// Tmux managers.
		tmuxCmd, err := tmux.NewRealCommander(log)
		if err != nil {
			return fmt.Errorf("init tmux: %w", err)
		}
		sm := tmux.NewSessionManager(tmuxCmd, log)
		pm := tmux.NewPaneManager(tmuxCmd, log)

		// Plugin registry.
		pluginReg := plugin.NewRegistry()
		if err := pluginloader.LoadPlugins(cfg, pluginReg); err != nil {
			return fmt.Errorf("load plugins: %w", err)
		}

		// Agent registry and messenger.
		registry := agent.NewRegistry(sm, pm, pluginReg, log)
		messenger := agent.NewMessenger(pm, registry, log)

		// Tmux watcher.
		watcher := tmux.NewWatcher(pm, 2*time.Second, log)

		// Phase engine.
		gateRunner := phase.NewGateRunner(cfg.Project.Root, 30*time.Second, log)
		engine, err := phase.NewEngine(cfg.Phases.SpecDir, gateRunner, j, log)
		if err != nil {
			return fmt.Errorf("create phase engine: %w", err)
		}

		// Phase completion handler.
		notesDir := filepath.Join(cfg.Project.StateDir, "notes")
		notesMgr := worknotes.NewManager(notesDir, log)
		completion := phase.NewCompletionHandler(engine, gateRunner, j, notesMgr, log)

		// Context builder with optional summarizer and budget enforcer.
		bankDir := filepath.Join(cfg.Project.StateDir, "memory-bank")
		memBank := bank.NewBank(bankDir, log)
		contextBld := phase.NewContextBuilder(j, notesMgr, memBank, log)

		// Progressive summarizer.
		summarizer := orchestrator.NewSummarizer(notesMgr, j, cfg.Context.Summary, log)
		contextBld.SetSummarizer(summarizer)

		// Context budget enforcer.
		budget := orchestrator.BudgetFromConfig(cfg.Context, log)
		contextBld.SetEnforcer(budget)

		// Tracker.
		tracker := phase.NewTracker(engine, log)

		// Session manager.
		sessDir := filepath.Join(cfg.Project.StateDir, "sessions")
		sessionMgr := session.NewManager(sessDir, st, log)

		// Security middleware.
		sandbox, err := security.NewSandbox(cfg.Project.Root, cfg.Security.AllowedPaths, cfg.Security.DeniedPaths, log)
		if err != nil {
			return fmt.Errorf("create sandbox: %w", err)
		}
		enforcer := security.NewEnforcer(sandbox, log)
		auditLogger, err := security.NewAuditLogger(filepath.Join(cfg.Project.StateDir, "audit.log"))
		if err != nil {
			return fmt.Errorf("create audit logger: %w", err)
		}
		defer auditLogger.Close()
		secMiddleware := security.NewSecurityMiddleware(enforcer, auditLogger, log)

		rateLimiter := security.NewRateLimiter(cfg.Security.MaxCmdsPerMin, cfg.Security.MaxFilesPerSession, log)
		secMiddleware.SetRateLimiter(rateLimiter)

		messenger.SetMessageGate(secMiddleware)

		// Build orchestrator.
		orch := orchestrator.New(
			cfg, registry, engine, completion, contextBld, tracker,
			watcher, messenger, sessionMgr, notesMgr, j, log,
		)
		orch.SetSecurityGate(&securityAdapter{mw: secMiddleware})

		// Signal handling: SIGINT/SIGTERM cancel the context.
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		if err := orch.Run(ctx); err != nil {
			return fmt.Errorf("orchestrator: %w", err)
		}

		return orch.Stop(context.Background())
	},
}

// securityAdapter adapts *security.SecurityMiddleware to orchestrator.SecurityGate.
type securityAdapter struct {
	mw *security.SecurityMiddleware
}

func (a *securityAdapter) Gate(agentID types.AgentID, perm types.Permission, action, target string, phaseID types.PhaseID, promptNum int) error {
	return a.mw.GateString(agentID, perm, action, target, phaseID, promptNum)
}
