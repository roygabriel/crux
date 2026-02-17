package main

import (
	"context"
	"fmt"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
	"github.com/roygabriel/crux/internal/tui"
	"github.com/roygabriel/crux/pkg/types"
	"github.com/spf13/cobra"
)

var tuiFlag bool

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

		gitGuard := security.NewGitGuard(cfg.Project.Root, log)
		secMiddleware.SetGitGuard(gitGuard)

		secretsScanner := security.NewSecretsScanner(log)
		secMiddleware.SetSecretsScanner(secretsScanner)

		secretsPath := filepath.Join(cfg.Project.StateDir, "secrets.env")
		secretsMgr := security.NewSecretsManager(secretsPath, log)
		if err := secretsMgr.Load(); err != nil {
			log.Warn("secrets manager load failed", "error", err)
		}
		secMiddleware.SetSecretsManager(secretsMgr)

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

		if tuiFlag {
			return runWithTUI(ctx, stop, orch, registry, rateLimiter, tracker)
		}

		// Headless mode.
		if err := orch.Run(ctx); err != nil {
			return fmt.Errorf("orchestrator: %w", err)
		}

		return orch.Stop(context.Background())
	},
}

func init() {
	startCmd.Flags().BoolVar(&tuiFlag, "tui", false, "Launch terminal dashboard")
}

// runWithTUI starts the orchestrator in a background goroutine and runs the
// bubbletea TUI in the foreground. Either exiting cancels the other.
func runWithTUI(
	ctx context.Context,
	stop context.CancelFunc,
	orch *orchestrator.Orchestrator,
	registry *agent.Registry,
	rateLimiter *security.RateLimiter,
	tracker *phase.Tracker,
) error {
	bridge := tui.NewStateBridge(1)
	worldState := orch.WorldState()

	// Set tick hook: builds a StateUpdate from live components and pushes it.
	orch.SetTickHook(func() {
		snap := worldState.Snapshot()
		agents := registry.List()

		snapshots := make([]tui.AgentSnapshot, 0, len(agents))
		for _, inst := range agents {
			as, _ := worldState.GetAgent(inst.Agent.ID)
			cmds, files := rateLimiter.Stats(inst.Agent.ID)

			snapshots = append(snapshots, tui.AgentSnapshot{
				ID:             inst.Agent.ID,
				Name:           inst.Agent.Name,
				Plugin:         inst.Agent.Plugin,
				Role:           inst.Agent.Role,
				Status:         inst.Agent.Status,
				PromptDisplay:  as.PromptDisplay,
				Task:           as.Task,
				CommandsPerMin: cmds,
				FilesSession:   files,
			})
		}

		bridge.Push(tui.StateUpdate{
			Phase:        snap.Phase,
			PhaseName:    snap.PhaseName,
			Agents:       snapshots,
			Progress:     tracker.OverallProgress(),
			GatesPassed:  len(snap.GatesPassed),
			GatesPending: len(snap.GatesPending),
			Timestamp:    snap.UpdatedAt,
		})
	})

	// Run orchestrator in background.
	orchErr := make(chan error, 1)
	go func() {
		orchErr <- orch.Run(ctx)
	}()

	// Run TUI in foreground (blocks).
	model := tui.NewModel(bridge)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		stop()
		return fmt.Errorf("tui: %w", err)
	}

	// TUI exited — shut down orchestrator.
	stop()
	if err := <-orchErr; err != nil {
		// Context cancellation is expected on TUI exit.
		if ctx.Err() == nil {
			return fmt.Errorf("orchestrator: %w", err)
		}
	}
	return orch.Stop(context.Background())
}

// securityAdapter adapts *security.SecurityMiddleware to orchestrator.SecurityGate.
type securityAdapter struct {
	mw *security.SecurityMiddleware
}

func (a *securityAdapter) Gate(agentID types.AgentID, perm types.Permission, action, target string, phaseID types.PhaseID, promptNum int) error {
	return a.mw.GateString(agentID, perm, action, target, phaseID, promptNum)
}
