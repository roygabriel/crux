package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/roygabriel/crux/internal/agent"
	"github.com/roygabriel/crux/internal/config"
	"github.com/roygabriel/crux/internal/instruct"
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

var (
	headlessFlag    bool
	noInstructFlag  bool
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the orchestration loop",
	RunE: func(cmd *cobra.Command, args []string) error {
		var log *slog.Logger
		var logBridge *tui.LogBridge

		if headlessFlag {
			log = setupLogger()
		} else {
			logBridge = tui.NewLogBridge(64)
			log = slog.New(logBridge)
			slog.SetDefault(log)
		}

		log.Info("loading configuration", "path", cfgFile)
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if err := validateGitRepo(cfg.Project.Root); err != nil {
			return err
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

		// Load phases early so we can fail before spawning agents or starting the TUI.
		if err := engine.LoadAll(); err != nil {
			return fmt.Errorf("load phases: %w", err)
		}
		if len(engine.PhaseOrder()) == 0 {
			return fmt.Errorf("no phase specifications found in %s\n\nRun 'crux plan' to generate phase specs, or add PHASE*.md files manually.", cfg.Phases.SpecDir)
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

		// Generate or refresh instruction files unless --no-instruct is set.
		if !noInstructFlag {
			dist := buildDistributor(cfg, log)
			generated, refreshed, instrErr := ensureInstructionFiles(context.Background(), dist, cfg, log)
			if instrErr != nil {
				log.Warn("instruction generation failed", "error", instrErr)
			} else if generated > 0 {
				log.Info("generated instruction files", "agents", generated)
			} else if refreshed > 0 {
				log.Info("refreshed instruction files", "agents", refreshed)
			} else {
				log.Info("instruction files up to date")
			}
		}

		// Build orchestrator.
		orch := orchestrator.New(
			cfg, registry, engine, completion, contextBld, tracker,
			watcher, messenger, sessionMgr, notesMgr, j, log,
		)
		orch.SetSecurityGate(&securityAdapter{mw: secMiddleware})
		orch.SetTmuxSessionManager(sm)

		// Signal handling: SIGINT/SIGTERM cancel the context.
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		if headlessFlag {
			// Headless mode: run orchestrator without TUI.
			if err := orch.Run(ctx); err != nil {
				return fmt.Errorf("orchestrator: %w", err)
			}
			return orch.Stop(context.Background())
		}

		return runWithTUI(ctx, stop, orch, registry, engine, messenger, rateLimiter, tracker, auditLogger, j, notesMgr, logBridge, log)
	},
}

func init() {
	startCmd.Flags().BoolVar(&headlessFlag, "headless", false, "Run without the terminal dashboard (for CI/scripting)")
	startCmd.Flags().BoolVar(&noInstructFlag, "no-instruct", false, "Skip instruction file generation on start")
}

// validateGitRepo checks that root is inside a git work tree with at least
// one commit. It returns a user-friendly error when either condition fails.
func validateGitRepo(root string) error {
	check := exec.Command("git", "-C", root, "rev-parse", "--is-inside-work-tree")
	if err := check.Run(); err != nil {
		return fmt.Errorf("project root is not a git repository\n\nRun 'git init && git add -A && git commit -m \"initial\"' in %s", root)
	}
	head := exec.Command("git", "-C", root, "rev-parse", "--verify", "HEAD")
	if err := head.Run(); err != nil {
		return fmt.Errorf("git repository has no commits\n\nCreate an initial commit before running crux start")
	}
	return nil
}

// runWithTUI starts the orchestrator in a background goroutine and runs the
// bubbletea TUI in the foreground. Either exiting cancels the other.
func runWithTUI(
	ctx context.Context,
	stop context.CancelFunc,
	orch *orchestrator.Orchestrator,
	registry *agent.Registry,
	engine *phase.Engine,
	messenger *agent.Messenger,
	rateLimiter *security.RateLimiter,
	tracker *phase.Tracker,
	auditLogger *security.AuditLogger,
	j *journal.Journal,
	notesMgr *worknotes.Manager,
	logBridge *tui.LogBridge,
	logger *slog.Logger,
) error {
	bridge := tui.NewStateBridge(1)

	commandBus := tui.NewCommandBus(16, logger)
	worldState := orch.WorldState()

	// Route audit entries to the TUI log panel.
	auditLogger.SetHook(func(entry security.AuditEntry) {
		logBridge.Send(tui.AuditToLogEntry(
			entry.Timestamp,
			string(entry.AgentID),
			string(entry.Action),
			entry.Target,
			entry.Allowed,
			entry.Reason,
		))
	})

	// Pre-tick hook: drain TUI commands and apply them to live components.
	orch.SetPreTickHook(func(ctx context.Context) {
		for {
			select {
			case cmd := <-commandBus.Receive():
				switch cmd.Type {
				case tui.CmdPauseAgent:
					existing, _ := worldState.GetAgent(cmd.AgentID)
					if err := registry.UpdateStatus(cmd.AgentID, types.StatusStopped); err != nil {
						logger.Warn("pause command failed", "agent_id", cmd.AgentID, "error", err)
					}
					worldState.UpdateAgent(cmd.AgentID, orchestrator.AgentState{
						Status:        types.StatusStopped,
						PromptDisplay: existing.PromptDisplay,
						Task:          existing.Task,
						PhaseID:       existing.PhaseID,
						AssignedAt:    existing.AssignedAt,
						LastActive:    time.Now().UTC(),
					})

				case tui.CmdResumeAgent:
					existing, _ := worldState.GetAgent(cmd.AgentID)
					if err := registry.UpdateStatus(cmd.AgentID, types.StatusIdle); err != nil {
						logger.Warn("resume command failed", "agent_id", cmd.AgentID, "error", err)
					}
					worldState.UpdateAgent(cmd.AgentID, orchestrator.AgentState{
						Status:        types.StatusIdle,
						PromptDisplay: existing.PromptDisplay,
						Task:          existing.Task,
						PhaseID:       existing.PhaseID,
						AssignedAt:    existing.AssignedAt,
						LastActive:    time.Now().UTC(),
					})

				case tui.CmdKillAgent:
					if err := registry.Kill(ctx, cmd.AgentID); err != nil {
						logger.Warn("kill command failed", "agent_id", cmd.AgentID, "error", err)
					}
					worldState.RemoveAgent(cmd.AgentID)

				case tui.CmdForceAdvance:
					if err := engine.ForceAdvance(ctx, cmd.PhaseID); err != nil {
						logger.Warn("force advance failed", "phase_id", cmd.PhaseID, "error", err)
					}

				case tui.CmdSendMessage:
					msg := types.Message{
						From:      "operator",
						To:        cmd.AgentID,
						Type:      types.MessageTask,
						Priority:  types.PriorityHigh,
						Payload:   cmd.Text,
						Timestamp: time.Now().UTC(),
					}
					if err := messenger.Send(ctx, cmd.AgentID, msg); err != nil {
						logger.Warn("send message failed", "agent_id", cmd.AgentID, "error", err)
					}

				case tui.CmdShutdown:
					logger.Info("shutdown command received from TUI")
					stop()
				}
			default:
				return
			}
		}
	})

	// Set tick hook: builds a StateUpdate from live components and pushes it.
	orch.SetTickHook(func() {
		snap := worldState.Snapshot()
		agents := registry.List()

		snapshots := make([]tui.AgentSnapshot, 0, len(agents))
		for _, inst := range agents {
			as, _ := worldState.GetAgent(inst.Agent.ID)
			cmds, files := rateLimiter.Stats(inst.Agent.ID)

			// Fetch recent decisions for this agent.
			var decisions []string
			if decs, err := j.ByAgent(context.Background(), inst.Agent.ID); err == nil {
				start := 0
				if len(decs) > 5 {
					start = len(decs) - 5
				}
				for _, d := range decs[start:] {
					decisions = append(decisions, fmt.Sprintf("%s — %s", d.Action, d.Rationale))
				}
			}

			// Fetch work notes summary for the current phase.
			var workNotesInfo string
			if notes, err := notesMgr.Read(string(snap.Phase)); err == nil {
				workNotesInfo = fmt.Sprintf("Status: %s", notes.Status)
				if len(notes.SessionLog) > 0 {
					last := notes.SessionLog[len(notes.SessionLog)-1]
					if last.Next != "" {
						workNotesInfo += fmt.Sprintf("\nNext: %s", last.Next)
					}
				}
			}

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
				Permission:     string(inst.Agent.Permission),
				Decisions:      decisions,
				WorkNotesInfo:  workNotesInfo,
				PaneContent:    orch.LatestContent(inst.Agent.ID),
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
	model := tui.NewModel(bridge, logBridge, commandBus)
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

// ensureInstructionFiles generates missing instruction files or refreshes
// stale ones. Returns the number of agents whose files were generated from
// scratch and the number whose files were refreshed.
func ensureInstructionFiles(ctx context.Context, dist *instruct.Distributor, cfg *config.Config, log *slog.Logger) (generated, refreshed int, err error) {
	if instructionFilesMissing(cfg) {
		if err := dist.GenerateAll(ctx); err != nil {
			return 0, 0, err
		}
		return len(cfg.Agents), 0, nil
	}

	// All primary files exist — check for staleness per agent.
	for _, id := range sortedAgentIDs(cfg) {
		files, _, previewErr := dist.PreviewForAgent(ctx, id)
		if previewErr != nil {
			log.Warn("failed to preview agent instructions", "agent_id", id, "error", previewErr)
			continue
		}

		for _, f := range files {
			existing, readErr := os.ReadFile(f.Path)
			if readErr != nil {
				// File missing for this agent — regenerate.
				if genErr := dist.GenerateForAgent(ctx, id); genErr != nil {
					log.Warn("failed to regenerate agent instructions", "agent_id", id, "error", genErr)
				} else {
					refreshed++
				}
				break
			}
			if string(existing) != f.Content {
				if genErr := dist.GenerateForAgent(ctx, id); genErr != nil {
					log.Warn("failed to regenerate agent instructions", "agent_id", id, "error", genErr)
				} else {
					refreshed++
				}
				break
			}
		}
	}

	return 0, refreshed, nil
}

// instructionFilesMissing returns true if any configured agent is missing
// its primary instruction file on disk.
func instructionFilesMissing(cfg *config.Config) bool {
	for _, id := range sortedAgentIDs(cfg) {
		agentCfg := cfg.Agents[id]
		adapter, err := instruct.AdapterForCLI(instruct.AgentCLI(agentCfg.Plugin))
		if err != nil {
			continue
		}
		probe := &instruct.RenderResult{Content: "probe"}
		files, err := adapter.PrepareFiles(probe, cfg.Project.Root, nil)
		if err != nil || len(files) == 0 {
			continue
		}
		if _, statErr := os.Stat(files[0].Path); os.IsNotExist(statErr) {
			return true
		}
	}
	return false
}
