package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/roygabriel/crux/internal/agent"
	"github.com/roygabriel/crux/internal/config"
	"github.com/roygabriel/crux/internal/memory/journal"
	"github.com/roygabriel/crux/internal/memory/session"
	"github.com/roygabriel/crux/internal/memory/worknotes"
	"github.com/roygabriel/crux/internal/phase"
	"github.com/roygabriel/crux/internal/tmux"
	"github.com/roygabriel/crux/pkg/types"
)

const defaultPollInterval = 2 * time.Second

// SecurityGate checks permission before executing an action.
type SecurityGate interface {
	Gate(agentID types.AgentID, perm types.Permission, action string, target string, phaseID types.PhaseID, promptNum int) error
}

// Orchestrator runs the main control loop: polls agents, updates world state,
// detects completion, runs gates, advances phases, and assigns work.
type Orchestrator struct {
	cfg          *config.Config
	worldState   *WorldState
	assigner     *Assigner
	rag          *DecisionRAG
	registry     *agent.Registry
	engine       *phase.Engine
	completion   *phase.CompletionHandler
	contextBld   *phase.ContextBuilder
	tracker      *phase.Tracker
	watcher      *tmux.Watcher
	messenger    *agent.Messenger
	sessionMgr   *session.Manager
	workNotes    *worknotes.Manager
	journal      *journal.Journal
	logger       *slog.Logger
	pollInterval time.Duration

	// tmuxSM manages the top-level tmux session where agent panes live.
	tmuxSM *tmux.SessionManager

	// tmuxSessionName is the name of the tmux session created for this run.
	tmuxSessionName string

	// security gates agent actions against their permission tier.
	security SecurityGate

	// preTickHook is called at the start of each tick before processing (e.g. command draining).
	preTickHook func(ctx context.Context)

	// tickHook is called at the end of each tick for external consumers (e.g. TUI).
	tickHook func()

	// conflicts detects and resolves file conflicts between parallel agents.
	conflicts *ConflictDetector

	// paneContent stores the latest captured content per agent, guarded by mu.
	mu          sync.Mutex
	paneContent map[types.AgentID]string

	// prevStatus tracks previous agent status for transition detection.
	prevStatus map[types.AgentID]types.AgentStatus

	// session holds the active session context.
	session *session.SessionContext
}

// New creates an Orchestrator with all dependencies wired.
func New(
	cfg *config.Config,
	registry *agent.Registry,
	engine *phase.Engine,
	completion *phase.CompletionHandler,
	contextBld *phase.ContextBuilder,
	tracker *phase.Tracker,
	watcher *tmux.Watcher,
	messenger *agent.Messenger,
	sessionMgr *session.Manager,
	workNotes *worknotes.Manager,
	j *journal.Journal,
	logger *slog.Logger,
) *Orchestrator {
	if logger == nil {
		logger = slog.Default()
	}
	ws := NewWorldState("")
	var recorder DecisionRecorder
	if j != nil {
		recorder = j
	} else {
		recorder = noopRecorder{}
	}
	conflicts := NewConflictDetector(
		engine,
		NewExecGitDiffer(cfg.Project.Root),
		registry,
		ws,
		workNotes,
		recorder,
		30*time.Second,
		logger,
	)
	return &Orchestrator{
		cfg:          cfg,
		worldState:   ws,
		assigner:     NewAssigner(registry, engine, ws, logger),
		rag:          NewDecisionRAG(j, logger),
		conflicts:    conflicts,
		registry:     registry,
		engine:       engine,
		completion:   completion,
		contextBld:   contextBld,
		tracker:      tracker,
		watcher:      watcher,
		messenger:    messenger,
		sessionMgr:   sessionMgr,
		workNotes:    workNotes,
		journal:      j,
		logger:       logger,
		pollInterval: defaultPollInterval,
		paneContent:  make(map[types.AgentID]string),
		prevStatus:   make(map[types.AgentID]types.AgentStatus),
	}
}

// SetSecurityGate configures the security gate for action checks.
func (o *Orchestrator) SetSecurityGate(g SecurityGate) {
	o.security = g
}

// SetTmuxSessionManager sets the tmux session manager used to create
// the top-level session that hosts agent panes.
func (o *Orchestrator) SetTmuxSessionManager(sm *tmux.SessionManager) {
	o.tmuxSM = sm
}

// SetTickHook registers a function called at the end of each tick iteration.
func (o *Orchestrator) SetTickHook(fn func()) {
	o.tickHook = fn
}

// SetPreTickHook registers a function called at the start of each tick,
// before agents are polled. Used to drain the TUI command bus.
func (o *Orchestrator) SetPreTickHook(fn func(ctx context.Context)) {
	o.preTickHook = fn
}

// WorldState returns the orchestrator's world state for external read access.
func (o *Orchestrator) WorldState() *WorldState {
	return o.worldState
}

// Run executes the main orchestration loop. It blocks until the context is
// cancelled or all phases are complete.
func (o *Orchestrator) Run(ctx context.Context) error {
	// Resume or start session.
	sc, err := o.sessionMgr.ResumeLatest()
	if err != nil {
		o.logger.Info("no existing session found, starting new one")
		sc, err = o.sessionMgr.Start()
		if err != nil {
			return fmt.Errorf("start session: %w", err)
		}
	}
	o.session = sc
	o.worldState.SessionID = sc.ID

	// Load phase engine.
	if err := o.engine.LoadAll(); err != nil {
		return fmt.Errorf("load phases: %w", err)
	}

	// Update world state with current phase.
	if spec := o.engine.CurrentPhase(); spec != nil {
		o.worldState.UpdatePhase(spec.ID, spec.Name)
	}

	// Create the tmux session that will host agent panes.
	if o.tmuxSM != nil {
		o.tmuxSessionName = "crux-" + sc.ID
		// Kill any stale session from a prior crash to avoid a Create failure.
		if exists, _ := o.tmuxSM.Exists(ctx, o.tmuxSessionName); exists {
			o.logger.Warn("killing stale tmux session", "session", o.tmuxSessionName)
			if err := o.tmuxSM.Kill(ctx, o.tmuxSessionName); err != nil {
				return fmt.Errorf("kill stale tmux session: %w", err)
			}
		}
		if err := o.tmuxSM.Create(ctx, o.tmuxSessionName); err != nil {
			return fmt.Errorf("create tmux session: %w", err)
		}
		o.logger.Info("created tmux session", "session", o.tmuxSessionName)
	}

	// Spawn configured agents.
	if err := o.spawnAgents(ctx); err != nil {
		return fmt.Errorf("spawn agents: %w", err)
	}

	// Start pane watchers for each agent.
	o.startWatchers(ctx)

	// Start file conflict monitoring.
	conflictCh := o.conflicts.MonitorRuntime(ctx)
	go func() {
		for event := range conflictCh {
			if err := o.conflicts.HandleConflict(ctx, event); err != nil {
				o.logger.Error("conflict handling failed", "error", err)
			}
		}
	}()

	// Main loop.
	ticker := time.NewTicker(o.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			o.logger.Info("context cancelled, shutting down")
			return o.shutdown(ctx)
		case <-ticker.C:
			if err := o.tick(ctx); err != nil {
				o.logger.Error("tick error", "error", err)
			}
		}
	}
}

// Stop performs a graceful shutdown: saves session, stops watchers, kills agents.
func (o *Orchestrator) Stop(ctx context.Context) error {
	return o.shutdown(ctx)
}

// tick runs one iteration of the control loop.
func (o *Orchestrator) tick(ctx context.Context) error {
	if o.preTickHook != nil {
		o.preTickHook(ctx)
	}

	agents := o.registry.List()

	for _, inst := range agents {
		id := inst.Agent.ID
		content := o.latestContent(id)
		if content == "" {
			continue
		}

		newStatus := o.detectStatus(inst, content)
		prev := o.prevStatus[id]

		if newStatus != prev {
			o.handleTransition(ctx, inst, prev, newStatus, content)
			o.prevStatus[id] = newStatus
		}
	}

	// Check for file conflicts before assigning.
	assignmentAllowed := true
	if nextPrompt := o.engine.CurrentPrompt(); nextPrompt != nil {
		if spec := o.engine.CurrentPhase(); spec != nil {
			// Check against all active phases from busy agents.
			for _, inst := range agents {
				if inst.Agent.Status == types.StatusBusy {
					agentState, ok := o.worldState.GetAgent(inst.Agent.ID)
					if ok && agentState.PhaseID != "" && agentState.PhaseID != spec.ID {
						if err := o.conflicts.CheckBeforeAssign(agentState.PhaseID, spec.ID); err != nil {
							o.logger.Warn("skipping assignment due to conflict", "error", err)
							assignmentAllowed = false
							break
						}
					}
				}
			}
		}
	}

	// Assign idle agents to pending prompts.
	if assignmentAllowed {
		if err := o.assigner.AssignNext(ctx); err != nil && err != ErrNoAvailableAgent {
			o.logger.Warn("assignment error", "error", err)
		} else if err == nil {
			// Track the assignment for conflict detection.
			if spec := o.engine.CurrentPhase(); spec != nil {
				// Find the agent that was just assigned (became busy).
				for _, inst := range o.registry.List() {
					if inst.Agent.Status == types.StatusBusy {
						agentState, ok := o.worldState.GetAgent(inst.Agent.ID)
						if ok && agentState.PhaseID == spec.ID {
							files := append(spec.FilesNew, spec.FilesModified...)
							o.conflicts.TrackAssignment(inst.Agent.ID, spec.ID, files)
						}
					}
				}
			}
		}
	}

	// Periodically save session.
	o.saveSession()

	// Check if all phases are complete.
	if o.engine.CurrentPhase() == nil {
		o.logger.Info("all phases complete")
	}

	// Notify external consumers (e.g. TUI) that a tick completed.
	if o.tickHook != nil {
		o.tickHook()
	}

	return nil
}

// detectStatus inspects pane content using the agent's plugin to determine status.
func (o *Orchestrator) detectStatus(inst *agent.AgentInstance, content string) types.AgentStatus {
	if _, isLimited := inst.Plugin.DetectRateLimit(content); isLimited {
		return types.StatusRateLimited
	}
	if _, isErr := inst.Plugin.DetectError(content); isErr {
		return types.StatusError
	}
	if inst.Plugin.DetectBusy(content) {
		return types.StatusBusy
	}
	if inst.Plugin.DetectReady(content) {
		return types.StatusIdle
	}
	return inst.Agent.Status
}

// handleTransition acts on a detected agent state change.
func (o *Orchestrator) handleTransition(
	ctx context.Context,
	inst *agent.AgentInstance,
	prev, curr types.AgentStatus,
	content string,
) {
	id := inst.Agent.ID
	o.logger.Info("agent status transition",
		"agent_id", id,
		"from", prev,
		"to", curr,
	)

	if err := o.registry.UpdateStatus(id, curr); err != nil {
		o.logger.Warn("failed to update agent status in registry", "agent_id", id, "error", err)
	}

	// Preserve PhaseID and AssignedAt from the current agent state.
	existing, _ := o.worldState.GetAgent(id)

	switch curr {
	case types.StatusIdle:
		if prev == types.StatusBusy {
			// Agent just finished — parse output and handle completion.
			o.handleCompletion(ctx, inst, content)
		}
		o.worldState.UpdateAgent(id, AgentState{
			Status:     types.StatusIdle,
			LastActive: time.Now().UTC(),
		})

	case types.StatusError:
		errMsg, _ := inst.Plugin.DetectError(content)
		o.logger.Error("agent error detected", "agent_id", id, "error", errMsg)
		o.worldState.UpdateAgent(id, AgentState{
			Status:       types.StatusError,
			LastDecision: errMsg,
			LastActive:   time.Now().UTC(),
			PhaseID:      existing.PhaseID,
			AssignedAt:   existing.AssignedAt,
		})

	case types.StatusRateLimited:
		retryAfter, _ := inst.Plugin.DetectRateLimit(content)
		o.logger.Warn("agent rate limited", "agent_id", id, "retry_after", retryAfter)
		o.worldState.UpdateAgent(id, AgentState{
			Status:     types.StatusRateLimited,
			LastActive: time.Now().UTC(),
			PhaseID:    existing.PhaseID,
			AssignedAt: existing.AssignedAt,
		})

	case types.StatusBusy:
		o.worldState.UpdateAgent(id, AgentState{
			Status:     types.StatusBusy,
			LastActive: time.Now().UTC(),
			PhaseID:    existing.PhaseID,
			AssignedAt: existing.AssignedAt,
		})
	}
}

// handleCompletion processes a newly-ready agent's output.
func (o *Orchestrator) handleCompletion(ctx context.Context, inst *agent.AgentInstance, content string) {
	id := inst.Agent.ID

	// Untrack from conflict detection.
	o.conflicts.UntrackAssignment(id)

	output, err := inst.Plugin.ParseOutput(content)
	if err != nil {
		o.logger.Warn("failed to parse agent output", "agent_id", id, "error", err)
		return
	}

	spec := o.engine.CurrentPhase()
	if spec == nil {
		return
	}

	prompt := o.engine.CurrentPrompt()
	if prompt == nil {
		return
	}

	// Gate verification commands and file writes before running completion.
	if o.security != nil {
		for _, gate := range spec.ExitCriteria {
			if gate.Command != "" {
				if err := o.security.Gate(id, inst.Agent.Permission, "shell_exec", gate.Command, spec.ID, prompt.PromptNumber); err != nil {
					o.logger.Warn("security gate denied verification command",
						"agent_id", id, "command", gate.Command, "error", err)
					return
				}
			}
		}
		for _, f := range append(spec.FilesNew, spec.FilesModified...) {
			if err := o.security.Gate(id, inst.Agent.Permission, "file_write", f, spec.ID, prompt.PromptNumber); err != nil {
				o.logger.Warn("security gate denied file write",
					"agent_id", id, "file", f, "error", err)
				return
			}
		}
	}

	result, err := o.completion.HandleCompletion(ctx, spec.ID, prompt.PromptNumber, output)
	if err != nil {
		o.logger.Error("completion handling failed", "agent_id", id, "error", err)
		return
	}

	if result.Passed {
		o.logger.Info("prompt completed successfully",
			"agent_id", id,
			"phase", spec.ID,
			"prompt", prompt.PromptNumber,
		)
		// Update world state with new phase if changed.
		if newSpec := o.engine.CurrentPhase(); newSpec != nil {
			o.worldState.UpdatePhase(newSpec.ID, newSpec.Name)
		}
	} else {
		o.logger.Warn("prompt gates failed",
			"agent_id", id,
			"phase", spec.ID,
			"prompt", prompt.PromptNumber,
		)
	}
}

// spawnAgents creates tmux panes and launches agents per config.
func (o *Orchestrator) spawnAgents(ctx context.Context) error {
	// Use the tmux session name if available, fall back to session ID.
	sessionID := o.session.ID
	if o.tmuxSessionName != "" {
		sessionID = o.tmuxSessionName
	}

	for name, agentCfg := range o.cfg.Agents {
		a := types.Agent{
			ID:         types.AgentID(name),
			Name:       name,
			Plugin:     agentCfg.Plugin,
			Role:       types.AgentRole(agentCfg.Role),
			Permission: types.Permission(agentCfg.Permission),
			SessionID:  sessionID,
		}
		if err := o.registry.Spawn(ctx, a); err != nil {
			return fmt.Errorf("spawn agent %q: %w", name, err)
		}
		o.prevStatus[a.ID] = types.StatusIdle
	}
	return nil
}

// startWatchers creates pane watchers for all registered agents.
func (o *Orchestrator) startWatchers(ctx context.Context) {
	for _, inst := range o.registry.List() {
		id := inst.Agent.ID
		paneID := inst.Agent.PaneID
		o.watcher.Watch(ctx, paneID, func(content string) {
			o.mu.Lock()
			o.paneContent[id] = content
			o.mu.Unlock()
		})
	}
}

// latestContent returns the most recently captured pane content for an agent.
func (o *Orchestrator) latestContent(id types.AgentID) string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.paneContent[id]
}

// saveSession persists the current session state to disk.
func (o *Orchestrator) saveSession() {
	if o.session == nil {
		return
	}

	spec := o.engine.CurrentPhase()
	if spec != nil {
		o.session.CurrentPhase = string(spec.ID)
	}

	prompt := o.engine.CurrentPrompt()
	if prompt != nil {
		o.session.PromptProgress = prompt.PromptNumber
	}

	if err := o.sessionMgr.Save(o.session); err != nil {
		o.logger.Warn("failed to save session", "error", err)
	}
}

// shutdown saves session, stops watchers, kills all agents, and destroys
// the tmux session.
func (o *Orchestrator) shutdown(ctx context.Context) error {
	o.saveSession()
	o.watcher.Stop()

	for _, inst := range o.registry.List() {
		if err := o.registry.Kill(ctx, inst.Agent.ID); err != nil {
			o.logger.Warn("failed to kill agent during shutdown", "agent_id", inst.Agent.ID, "error", err)
		}
	}

	// Destroy the tmux session created for this run.
	if o.tmuxSM != nil && o.tmuxSessionName != "" {
		if err := o.tmuxSM.Kill(ctx, o.tmuxSessionName); err != nil {
			o.logger.Warn("failed to kill tmux session during shutdown", "session", o.tmuxSessionName, "error", err)
		}
	}

	o.logger.Info("orchestrator shutdown complete")
	return nil
}
