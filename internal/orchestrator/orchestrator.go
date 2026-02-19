package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/roygabriel/crux/internal/agent"
	"github.com/roygabriel/crux/internal/config"
	"github.com/roygabriel/crux/internal/instruct"
	"github.com/roygabriel/crux/internal/memory/journal"
	"github.com/roygabriel/crux/internal/memory/session"
	"github.com/roygabriel/crux/internal/memory/worknotes"
	"github.com/roygabriel/crux/internal/phase"
	"github.com/roygabriel/crux/internal/tmux"
	"github.com/roygabriel/crux/pkg/types"
)

const defaultPollInterval = 2 * time.Second
const defaultReadyTimeout = 20 * time.Second
const noAvailableAgentLogInterval = 10 * time.Second
const dispatchBackoffBase = 2 * time.Second
const dispatchBackoffMax = 60 * time.Second
const dispatchFailureThreshold = 2
const dispatchRepeatThreshold = 3
const cooldownLogInterval = 8 * time.Second
const dispatchAckTimeout = 12 * time.Second
const recoveryGuardInterval = 8 * time.Second

var errDispatchCoolingDown = errors.New("dispatch cooling down")

type promptKey struct {
	phaseID   types.PhaseID
	promptNum int
}

type dispatchFingerprint struct {
	phaseID    types.PhaseID
	promptNum  int
	promptHash string
	filesHash  string
}

type dispatchAckState struct {
	key          promptKey
	baselineHash string
	deadline     time.Time
	sentAt       time.Time
}

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

	// distributor generates and writes per-agent instruction files.
	distributor *instruct.Distributor

	// instructOrch builds the orchestrator's own system prompt.
	instructOrch *instruct.OrchestratorPromptBuilder

	// conflicts detects and resolves file conflicts between parallel agents.
	conflicts *ConflictDetector

	// paneContent stores the latest captured content per agent, guarded by mu.
	// orchestratorPrompt holds the latest rendered orchestrator system prompt.
	// agentReady tracks whether each agent has shown a ready prompt since spawn.
	mu                 sync.Mutex
	paneContent        map[types.AgentID]string
	firstContentAt     map[types.AgentID]time.Time
	agentReady         map[types.AgentID]bool
	fallbackReadyLog   map[types.AgentID]bool
	orchestratorPrompt string

	// prevStatus tracks previous agent status for transition detection.
	prevStatus map[types.AgentID]types.AgentStatus

	// lastDispatchTime records when a prompt was last dispatched to each agent.
	// Used to suppress premature Busy→Idle transitions during the grace period.
	lastDispatchTime map[types.AgentID]time.Time
	dispatchGrace    time.Duration
	readyTimeout     time.Duration
	lastNoAvailLogAt time.Time

	// Dispatch stability tracking prevents tight re-dispatch loops when an
	// agent repeatedly receives the same prompt without advancing progress.
	lastDispatchFingerprint map[types.AgentID]dispatchFingerprint
	lastDispatchPaneHash    map[types.AgentID]string
	repeatDispatchCount     map[types.AgentID]int
	promptFailCount         map[promptKey]int
	promptCooldownUntil     map[promptKey]time.Time
	promptCooldownLogAt     map[promptKey]time.Time
	pendingDispatchAck      map[types.AgentID]dispatchAckState
	agentRecoveryUntil      map[types.AgentID]time.Time
	lastRecoveryAt          map[types.AgentID]time.Time

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
	assigner := NewAssigner(registry, engine, ws, logger)
	readyTimeout := defaultReadyTimeout
	if cfg != nil {
		if parsed, err := time.ParseDuration(strings.TrimSpace(cfg.Context.ReadyTimeout)); err == nil && parsed > 0 {
			readyTimeout = parsed
		}
	}

	o := &Orchestrator{
		cfg:                     cfg,
		worldState:              ws,
		assigner:                assigner,
		rag:                     NewDecisionRAG(j, logger),
		conflicts:               conflicts,
		registry:                registry,
		engine:                  engine,
		completion:              completion,
		contextBld:              contextBld,
		tracker:                 tracker,
		watcher:                 watcher,
		messenger:               messenger,
		sessionMgr:              sessionMgr,
		workNotes:               workNotes,
		journal:                 j,
		logger:                  logger,
		pollInterval:            defaultPollInterval,
		paneContent:             make(map[types.AgentID]string),
		firstContentAt:          make(map[types.AgentID]time.Time),
		agentReady:              make(map[types.AgentID]bool),
		fallbackReadyLog:        make(map[types.AgentID]bool),
		prevStatus:              make(map[types.AgentID]types.AgentStatus),
		lastDispatchTime:        make(map[types.AgentID]time.Time),
		dispatchGrace:           5 * time.Second,
		readyTimeout:            readyTimeout,
		lastDispatchFingerprint: make(map[types.AgentID]dispatchFingerprint),
		lastDispatchPaneHash:    make(map[types.AgentID]string),
		repeatDispatchCount:     make(map[types.AgentID]int),
		promptFailCount:         make(map[promptKey]int),
		promptCooldownUntil:     make(map[promptKey]time.Time),
		promptCooldownLogAt:     make(map[promptKey]time.Time),
		pendingDispatchAck:      make(map[types.AgentID]dispatchAckState),
		agentRecoveryUntil:      make(map[types.AgentID]time.Time),
		lastRecoveryAt:          make(map[types.AgentID]time.Time),
	}
	assigner.SetReadyGate(o.isAgentReadyForDispatch)
	return o
}

// SetSecurityGate configures the security gate for action checks.
func (o *Orchestrator) SetSecurityGate(g SecurityGate) {
	o.security = g
}

// SetDistributor configures the instruction file distributor.
func (o *Orchestrator) SetDistributor(d *instruct.Distributor) {
	o.distributor = d
}

// SetOrchestratorPromptBuilder configures the builder used to generate the
// orchestrator's own system prompt.
func (o *Orchestrator) SetOrchestratorPromptBuilder(b *instruct.OrchestratorPromptBuilder) {
	o.instructOrch = b
}

// OrchestratorPrompt returns the latest rendered orchestrator system prompt.
// Thread-safe.
func (o *Orchestrator) OrchestratorPrompt() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.orchestratorPrompt
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

	// Generate instruction files for all agents before spawning.
	if o.distributor != nil {
		if err := o.distributor.GenerateAll(ctx); err != nil {
			o.logger.Warn("instruction generation failed", "error", err)
		}
	}

	// Build the orchestrator's own system prompt.
	o.buildOrchestratorPrompt(ctx)

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
			func() {
				defer func() {
					if r := recover(); r != nil {
						o.logger.Error("panic in conflict handler",
							"recover", r)
					}
				}()
				if err := o.conflicts.HandleConflict(ctx, event); err != nil {
					o.logger.Error("conflict handling failed", "error", err)
				}
			}()
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
		if recovered := o.checkDispatchAck(ctx, inst, content); recovered {
			o.prevStatus[id] = types.StatusIdle
			continue
		}
		if content == "" {
			continue
		}

		newStatus := o.detectStatus(inst, content)
		prev := o.prevStatus[id]

		if newStatus == types.StatusIdle && inst.Plugin.DetectReady(content) {
			o.markAgentReady(id)
		}

		if newStatus != prev {
			// Suppress premature Busy→Idle transitions within the dispatch grace period.
			if prev == types.StatusBusy && newStatus == types.StatusIdle {
				if dt, ok := o.lastDispatchTime[id]; ok && time.Since(dt) < o.dispatchGrace {
					continue
				}
			}
			o.handleTransition(ctx, inst, prev, newStatus, content)
			o.prevStatus[id] = newStatus
		}
	}

	// Check for prompt cooldown and file conflicts before assigning.
	assignmentAllowed := true
	promptCoolingDown := false
	if nextPrompt := o.engine.CurrentPrompt(); nextPrompt != nil {
		if spec := o.engine.CurrentPhase(); spec != nil {
			key := makePromptKey(spec.ID, nextPrompt.PromptNumber)
			if until, cooling := o.promptCooldown(key); cooling {
				promptCoolingDown = true
				o.logPromptCooldown(key, until)
			}

			if promptCoolingDown {
				assignmentAllowed = false
			}

			// Check against all active phases from busy agents.
			if assignmentAllowed {
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
	}

	// Assign idle agents to pending prompts and dispatch the rendered prompt.
	if assignmentAllowed {
		agentID, err := o.assigner.AssignNext(ctx)
		if err != nil && err != ErrNoAvailableAgent {
			o.logger.Warn("assignment error", "error", err)
		} else if err == ErrNoAvailableAgent {
			o.logNoAvailableAgents(agents)
		} else if err == nil && agentID != "" {
			if dispErr := o.dispatchPrompt(ctx, agentID); dispErr != nil {
				if !errors.Is(dispErr, errDispatchCoolingDown) {
					o.logger.Warn("dispatch error", "agent_id", agentID, "error", dispErr)
				}
			} else {
				// Track the assignment for conflict detection.
				if spec := o.engine.CurrentPhase(); spec != nil {
					agentState, ok := o.worldState.GetAgent(agentID)
					if ok && agentState.PhaseID == spec.ID {
						files := append(spec.FilesNew, spec.FilesModified...)
						o.conflicts.TrackAssignment(agentID, spec.ID, files)
					}
				}
			}
		}
	} else if promptCoolingDown {
		// Prompt backoff is active; skip assignment until cooldown expires.
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
	if _, isPrompt := inst.Plugin.DetectPrompt(content); isPrompt {
		return types.StatusPrompted
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
	if curr != types.StatusBusy {
		o.clearDispatchAck(id)
	}

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
		// Regenerate this agent's instructions — error context may require
		// a different assignment or updated guidance.
		o.regenerateAgentInstructions(ctx, id)

	case types.StatusRateLimited:
		retryAfter, _ := inst.Plugin.DetectRateLimit(content)
		o.logger.Warn("agent rate limited", "agent_id", id, "retry_after", retryAfter)
		o.worldState.UpdateAgent(id, AgentState{
			Status:     types.StatusRateLimited,
			LastActive: time.Now().UTC(),
			PhaseID:    existing.PhaseID,
			AssignedAt: existing.AssignedAt,
		})

	case types.StatusPrompted:
		o.handlePromptResponse(ctx, inst, content)
		o.worldState.UpdateAgent(id, AgentState{
			Status:     types.StatusPrompted,
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

	spec := o.engine.CurrentPhase()
	if spec == nil {
		return
	}

	prompt := o.engine.CurrentPrompt()
	if prompt == nil {
		return
	}

	output, err := inst.Plugin.ParseOutput(content)
	if err != nil {
		o.logger.Warn("failed to parse agent output", "agent_id", id, "error", err)
		o.clearDispatchAck(id)
		o.clearDispatchTracking(id)
		o.notePromptFailure(spec.ID, prompt.PromptNumber, "parse output failed")
		return
	}

	// Gate verification commands and file writes before running completion.
	if o.security != nil {
		for _, gate := range spec.ExitCriteria {
			if gate.Command != "" {
				if err := o.security.Gate(id, inst.Agent.Permission, "shell_exec", gate.Command, spec.ID, prompt.PromptNumber); err != nil {
					o.logger.Warn("security gate denied verification command",
						"agent_id", id, "command", gate.Command, "error", err)
					o.clearDispatchAck(id)
					o.clearDispatchTracking(id)
					o.notePromptFailure(spec.ID, prompt.PromptNumber, "security denied verification command")
					return
				}
			}
		}
		for _, f := range append(spec.FilesNew, spec.FilesModified...) {
			if err := o.security.Gate(id, inst.Agent.Permission, "file_write", f, spec.ID, prompt.PromptNumber); err != nil {
				o.logger.Warn("security gate denied file write",
					"agent_id", id, "file", f, "error", err)
				o.clearDispatchAck(id)
				o.clearDispatchTracking(id)
				o.notePromptFailure(spec.ID, prompt.PromptNumber, "security denied file write")
				return
			}
		}
	}

	result, err := o.completion.HandleCompletion(ctx, spec.ID, prompt.PromptNumber, output)
	if err != nil {
		o.logger.Error("completion handling failed", "agent_id", id, "error", err)
		o.clearDispatchAck(id)
		o.clearDispatchTracking(id)
		o.notePromptFailure(spec.ID, prompt.PromptNumber, "completion handling failed")
		return
	}

	if result.Passed {
		o.logger.Info("prompt completed successfully",
			"agent_id", id,
			"phase", spec.ID,
			"prompt", prompt.PromptNumber,
		)
		o.clearDispatchAck(id)
		o.clearPromptFailure(spec.ID, prompt.PromptNumber)
		o.clearDispatchTracking(id)
		// Update world state with new phase if changed.
		if newSpec := o.engine.CurrentPhase(); newSpec != nil {
			o.worldState.UpdatePhase(newSpec.ID, newSpec.Name)
		}
		// Phase context changed; regenerate instructions for all agents.
		o.regenerateInstructions(ctx)
	} else {
		o.logger.Warn("prompt gates failed",
			"agent_id", id,
			"phase", spec.ID,
			"prompt", prompt.PromptNumber,
		)
		o.clearDispatchAck(id)
		o.clearDispatchTracking(id)
		o.notePromptFailure(spec.ID, prompt.PromptNumber, "verification gates failed")
	}
}

// handlePromptResponse detects the interactive prompt type from pane content
// and sends the appropriate key sequence to auto-accept it.
func (o *Orchestrator) handlePromptResponse(ctx context.Context, inst *agent.AgentInstance, content string) {
	resp, ok := inst.Plugin.DetectPrompt(content)
	if !ok {
		return
	}

	o.logger.Info("auto-accepting interactive prompt",
		"agent_id", inst.Agent.ID,
		"prompt_type", resp.Description,
	)

	if err := o.messenger.SendRawKeys(ctx, inst.Agent.ID, resp.Keys...); err != nil {
		o.logger.Error("failed to auto-accept prompt",
			"agent_id", inst.Agent.ID,
			"error", err,
		)
	}
}

// regenerateInstructions re-renders instructions for all agents after a
// phase advancement. Agents that support mid-session reload (Gemini, Codex)
// are reloaded immediately. Restart-based agents (Claude, Copilot) are
// only flagged; the orchestrator handles restart at session boundaries.
func (o *Orchestrator) regenerateInstructions(ctx context.Context) {
	if o.distributor != nil {
		for _, inst := range o.registry.List() {
			agentID := string(inst.Agent.ID)
			changed, err := o.distributor.RegenerateIfStale(ctx, agentID)
			if err != nil {
				o.logger.Warn("instruction regeneration failed",
					"agent_id", agentID, "error", err)
				continue
			}
			if changed && o.distributor.NeedsReload(agentID) {
				if err := o.distributor.ReloadAgent(ctx, agentID); err != nil {
					o.logger.Warn("agent reload failed",
						"agent_id", agentID, "error", err)
				}
			}
		}
	}

	// Rebuild the orchestrator prompt — phase context has changed.
	o.buildOrchestratorPrompt(ctx)
}

// buildOrchestratorPrompt renders the orchestrator system prompt from the
// current world state and stores it for retrieval via OrchestratorPrompt().
func (o *Orchestrator) buildOrchestratorPrompt(ctx context.Context) {
	if o.instructOrch == nil {
		return
	}

	content, err := o.instructOrch.BuildWithWorldState(ctx, o.worldState.Compact())
	if err != nil {
		o.logger.Warn("orchestrator prompt build failed", "error", err)
		return
	}

	o.mu.Lock()
	o.orchestratorPrompt = content
	o.mu.Unlock()

	o.logger.Info("orchestrator prompt built",
		"tokens", instruct.EstimateTokens(content),
	)
	o.logInstructionEvent(ctx, "build_orchestrator_prompt",
		fmt.Sprintf("rendered orchestrator prompt (%d tokens)", instruct.EstimateTokens(content)))
}

// regenerateAgentInstructions re-renders the instruction file for a single
// agent and reloads it if the agent supports mid-session reload.
func (o *Orchestrator) regenerateAgentInstructions(ctx context.Context, agentID types.AgentID) {
	if o.distributor == nil {
		return
	}

	id := string(agentID)
	changed, err := o.distributor.RegenerateIfStale(ctx, id)
	if err != nil {
		o.logger.Warn("agent instruction regeneration failed",
			"agent_id", id, "error", err)
		return
	}
	if changed && o.distributor.NeedsReload(id) {
		if err := o.distributor.ReloadAgent(ctx, id); err != nil {
			o.logger.Warn("agent reload failed",
				"agent_id", id, "error", err)
		}
	}
	o.logInstructionEvent(ctx, "regenerate_agent_instructions",
		fmt.Sprintf("agent=%s changed=%t", id, changed))
}

// logInstructionEvent records an instruction-related decision to the journal.
func (o *Orchestrator) logInstructionEvent(ctx context.Context, action, detail string) {
	if o.journal == nil {
		return
	}

	var phaseID types.PhaseID
	var promptNum int
	if spec := o.engine.CurrentPhase(); spec != nil {
		phaseID = spec.ID
	}
	if prompt := o.engine.CurrentPrompt(); prompt != nil {
		promptNum = prompt.PromptNumber
	}

	_ = o.journal.Record(ctx, types.Decision{
		AgentID:   "orchestrator",
		PhaseID:   phaseID,
		PromptNum: promptNum,
		Context:   "instruction regeneration",
		Action:    action,
		Rationale: detail,
	})
}

// dispatchPrompt renders the current prompt and sends it to the agent's tmux pane.
// On failure it reverts the agent back to Idle.
func (o *Orchestrator) dispatchPrompt(ctx context.Context, agentID types.AgentID) error {
	prompt := o.engine.CurrentPrompt()
	if prompt == nil {
		o.revertAssignment(agentID, fmt.Errorf("no current prompt"))
		return fmt.Errorf("dispatch prompt: no current prompt")
	}

	spec := o.engine.CurrentPhase()
	if spec == nil {
		o.revertAssignment(agentID, fmt.Errorf("no current phase"))
		return fmt.Errorf("dispatch prompt: no current phase")
	}
	key := makePromptKey(spec.ID, prompt.PromptNumber)
	if until, cooling := o.promptCooldown(key); cooling {
		o.logPromptCooldown(key, until)
		o.revertAssignment(agentID, errDispatchCoolingDown)
		return fmt.Errorf("dispatch prompt: %w", errDispatchCoolingDown)
	}

	inst, err := o.registry.Get(agentID)
	if err != nil {
		o.revertAssignment(agentID, err)
		return fmt.Errorf("dispatch prompt: get agent: %w", err)
	}

	promptData, err := o.contextBld.BuildForPrompt(ctx, *prompt, *spec, string(inst.Agent.Role), string(inst.Agent.Permission))
	if err != nil {
		o.revertAssignment(agentID, err)
		return fmt.Errorf("dispatch prompt: build context: %w", err)
	}

	rendered, err := phase.RenderPrompt(promptData)
	if err != nil {
		o.revertAssignment(agentID, err)
		return fmt.Errorf("dispatch prompt: render: %w", err)
	}
	paneHash := hashString(o.latestContent(agentID))
	fp := buildDispatchFingerprint(spec, prompt, rendered)
	repeat := o.bumpDispatchRepeat(agentID, fp, paneHash)
	if repeat > dispatchRepeatThreshold {
		o.notePromptFailure(spec.ID, prompt.PromptNumber,
			fmt.Sprintf("repeated identical dispatch x%d", repeat))
		o.clearDispatchTracking(agentID)
		o.clearDispatchAck(agentID)
		if recErr := o.recoverAgent(ctx, agentID, fmt.Sprintf("repeat-dispatch-%d", repeat)); recErr != nil {
			o.logger.Warn("agent recovery failed", "agent_id", agentID, "error", recErr)
		}
		o.revertAssignment(agentID, errDispatchCoolingDown)
		return fmt.Errorf("dispatch prompt: %w", errDispatchCoolingDown)
	}

	msg := types.Message{
		ID:        fmt.Sprintf("dispatch-%d", time.Now().UnixNano()),
		From:      "orchestrator",
		To:        agentID,
		Type:      types.MessageTask,
		Payload:   rendered,
		Timestamp: time.Now().UTC(),
	}

	if err := o.messenger.Send(ctx, agentID, msg); err != nil {
		o.revertAssignment(agentID, err)
		return fmt.Errorf("dispatch prompt: send: %w", err)
	}

	o.noteDispatchAck(agentID, key, paneHash)
	o.lastDispatchTime[agentID] = time.Now()
	o.prevStatus[agentID] = types.StatusBusy

	o.logger.Info("dispatched prompt to agent",
		"agent_id", agentID,
		"phase", spec.ID,
		"prompt", prompt.PromptNumber,
	)
	return nil
}

// revertAssignment rolls an agent back from Busy to Idle when prompt dispatch fails.
func (o *Orchestrator) revertAssignment(agentID types.AgentID, cause error) {
	o.logger.Warn("reverting assignment",
		"agent_id", agentID,
		"cause", cause,
	)
	o.clearDispatchAck(agentID)
	if err := o.registry.UpdateStatus(agentID, types.StatusIdle); err != nil {
		o.logger.Error("revert assignment: update registry", "agent_id", agentID, "error", err)
	}
	existing, _ := o.worldState.GetAgent(agentID)
	o.worldState.UpdateAgent(agentID, AgentState{
		Status:        types.StatusIdle,
		PromptDisplay: existing.PromptDisplay,
		Task:          existing.Task,
		PhaseID:       existing.PhaseID,
		AssignedAt:    existing.AssignedAt,
		LastActive:    time.Now().UTC(),
	})
	o.prevStatus[agentID] = types.StatusIdle
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
		o.setAgentReady(a.ID, false)
		o.mu.Lock()
		o.firstContentAt[a.ID] = time.Time{}
		o.fallbackReadyLog[a.ID] = false
		o.mu.Unlock()
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
			if strings.TrimSpace(content) != "" {
				if o.firstContentAt[id].IsZero() {
					o.firstContentAt[id] = time.Now().UTC()
				}
			}
			o.mu.Unlock()
		})
	}
}

// IsAgentReady reports whether an agent has shown a ready prompt since spawn.
func (o *Orchestrator) IsAgentReady(id types.AgentID) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.agentReady[id]
}

func (o *Orchestrator) isAgentReadyForDispatch(id types.AgentID) bool {
	if until, ok := o.agentRecoveryUntil[id]; ok {
		if time.Now().Before(until) {
			return false
		}
		delete(o.agentRecoveryUntil, id)
	}

	if o.IsAgentReady(id) {
		return true
	}

	o.mu.Lock()
	content := o.paneContent[id]
	firstSeen := o.firstContentAt[id]
	o.mu.Unlock()

	if strings.TrimSpace(content) == "" || firstSeen.IsZero() {
		return false
	}
	if time.Since(firstSeen) < o.readyTimeout {
		return false
	}

	inst, err := o.registry.Get(id)
	if err != nil {
		return false
	}
	if _, limited := inst.Plugin.DetectRateLimit(content); limited {
		return false
	}
	if _, isErr := inst.Plugin.DetectError(content); isErr {
		return false
	}
	if _, isPrompt := inst.Plugin.DetectPrompt(content); isPrompt {
		return false
	}
	if inst.Plugin.DetectBusy(content) {
		return false
	}

	o.logFallbackReadiness(id)
	return true
}

func (o *Orchestrator) setAgentReady(id types.AgentID, ready bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.agentReady[id] = ready
}

func (o *Orchestrator) markAgentReady(id types.AgentID) {
	changed := false
	o.mu.Lock()
	if !o.agentReady[id] {
		o.agentReady[id] = true
		changed = true
	}
	o.mu.Unlock()
	if !changed {
		return
	}
	o.logger.Info("agent ready for dispatch",
		"agent_id", id,
	)
}

func (o *Orchestrator) logNoAvailableAgents(agents []*agent.AgentInstance) {
	now := time.Now()
	if !o.lastNoAvailLogAt.IsZero() && now.Sub(o.lastNoAvailLogAt) < noAvailableAgentLogInterval {
		return
	}
	o.lastNoAvailLogAt = now

	var idle, idleReady, idleWaiting, busy, prompted, failed, limited, stopped int
	for _, inst := range agents {
		switch inst.Agent.Status {
		case types.StatusIdle:
			idle++
			if o.isAgentReadyForDispatch(inst.Agent.ID) {
				idleReady++
			} else {
				idleWaiting++
			}
		case types.StatusBusy:
			busy++
		case types.StatusPrompted:
			prompted++
		case types.StatusError:
			failed++
		case types.StatusRateLimited:
			limited++
		case types.StatusStopped:
			stopped++
		}
	}

	o.logger.Info("assignment blocked: no dispatchable idle agents",
		"total_agents", len(agents),
		"idle", idle,
		"idle_ready", idleReady,
		"idle_waiting_readiness", idleWaiting,
		"busy", busy,
		"prompted", prompted,
		"error", failed,
		"rate_limited", limited,
		"stopped", stopped,
	)
}

func (o *Orchestrator) logFallbackReadiness(id types.AgentID) {
	o.mu.Lock()
	if o.fallbackReadyLog[id] {
		o.mu.Unlock()
		return
	}
	o.fallbackReadyLog[id] = true
	firstSeen := o.firstContentAt[id]
	o.mu.Unlock()

	o.logger.Info("agent dispatch-ready via startup timeout fallback",
		"agent_id", id,
		"waited", time.Since(firstSeen).Round(time.Second),
	)
}

func makePromptKey(phaseID types.PhaseID, promptNum int) promptKey {
	return promptKey{phaseID: phaseID, promptNum: promptNum}
}

func hashString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func hashStrings(ss []string) string {
	cp := append([]string(nil), ss...)
	sort.Strings(cp)
	return hashString(strings.Join(cp, "\n"))
}

func buildDispatchFingerprint(spec *phase.PhaseSpec, prompt *phase.PromptContract, rendered string) dispatchFingerprint {
	files := append([]string(nil), spec.FilesNew...)
	files = append(files, spec.FilesModified...)
	return dispatchFingerprint{
		phaseID:    spec.ID,
		promptNum:  prompt.PromptNumber,
		promptHash: hashString(rendered),
		filesHash:  hashStrings(files),
	}
}

func (o *Orchestrator) ensureDispatchMaps() {
	if o.lastDispatchFingerprint == nil {
		o.lastDispatchFingerprint = make(map[types.AgentID]dispatchFingerprint)
	}
	if o.lastDispatchPaneHash == nil {
		o.lastDispatchPaneHash = make(map[types.AgentID]string)
	}
	if o.repeatDispatchCount == nil {
		o.repeatDispatchCount = make(map[types.AgentID]int)
	}
	if o.promptFailCount == nil {
		o.promptFailCount = make(map[promptKey]int)
	}
	if o.promptCooldownUntil == nil {
		o.promptCooldownUntil = make(map[promptKey]time.Time)
	}
	if o.promptCooldownLogAt == nil {
		o.promptCooldownLogAt = make(map[promptKey]time.Time)
	}
	if o.pendingDispatchAck == nil {
		o.pendingDispatchAck = make(map[types.AgentID]dispatchAckState)
	}
	if o.agentRecoveryUntil == nil {
		o.agentRecoveryUntil = make(map[types.AgentID]time.Time)
	}
	if o.lastRecoveryAt == nil {
		o.lastRecoveryAt = make(map[types.AgentID]time.Time)
	}
}

func (o *Orchestrator) clearDispatchTracking(agentID types.AgentID) {
	o.ensureDispatchMaps()
	delete(o.lastDispatchFingerprint, agentID)
	delete(o.lastDispatchPaneHash, agentID)
	delete(o.repeatDispatchCount, agentID)
}

func (o *Orchestrator) bumpDispatchRepeat(agentID types.AgentID, fp dispatchFingerprint, paneHash string) int {
	o.ensureDispatchMaps()
	prev, ok := o.lastDispatchFingerprint[agentID]
	prevPaneHash := o.lastDispatchPaneHash[agentID]
	if ok && prev == fp && prevPaneHash == paneHash {
		o.repeatDispatchCount[agentID]++
	} else {
		o.lastDispatchFingerprint[agentID] = fp
		o.lastDispatchPaneHash[agentID] = paneHash
		o.repeatDispatchCount[agentID] = 1
	}
	return o.repeatDispatchCount[agentID]
}

func (o *Orchestrator) promptCooldown(key promptKey) (time.Time, bool) {
	o.ensureDispatchMaps()
	until, ok := o.promptCooldownUntil[key]
	if !ok {
		return time.Time{}, false
	}
	if time.Now().After(until) {
		delete(o.promptFailCount, key)
		delete(o.promptCooldownUntil, key)
		delete(o.promptCooldownLogAt, key)
		return time.Time{}, false
	}
	return until, true
}

func (o *Orchestrator) noteDispatchAck(agentID types.AgentID, key promptKey, baselineHash string) {
	o.ensureDispatchMaps()
	now := time.Now()
	o.pendingDispatchAck[agentID] = dispatchAckState{
		key:          key,
		baselineHash: baselineHash,
		sentAt:       now,
		deadline:     now.Add(dispatchAckTimeout),
	}
}

func (o *Orchestrator) clearDispatchAck(agentID types.AgentID) {
	o.ensureDispatchMaps()
	delete(o.pendingDispatchAck, agentID)
}

func (o *Orchestrator) checkDispatchAck(ctx context.Context, inst *agent.AgentInstance, content string) bool {
	o.ensureDispatchMaps()
	ack, ok := o.pendingDispatchAck[inst.Agent.ID]
	if !ok {
		return false
	}

	currentHash := hashString(content)
	if currentHash != ack.baselineHash {
		delete(o.pendingDispatchAck, inst.Agent.ID)
		return false
	}

	if time.Now().Before(ack.deadline) {
		return false
	}

	o.logger.Warn("dispatch ack timeout",
		"agent_id", inst.Agent.ID,
		"phase", ack.key.phaseID,
		"prompt", ack.key.promptNum,
		"waited", time.Since(ack.sentAt).Round(time.Second),
	)
	o.notePromptFailure(ack.key.phaseID, ack.key.promptNum, "dispatch ack timeout")
	o.clearDispatchTracking(inst.Agent.ID)
	o.clearDispatchAck(inst.Agent.ID)
	if err := o.recoverAgent(ctx, inst.Agent.ID, "dispatch-ack-timeout"); err != nil {
		o.logger.Warn("agent recovery failed after ack timeout",
			"agent_id", inst.Agent.ID,
			"error", err,
		)
	}
	o.revertAssignment(inst.Agent.ID, fmt.Errorf("dispatch ack timeout"))
	return true
}

func (o *Orchestrator) recoverAgent(ctx context.Context, agentID types.AgentID, reason string) error {
	o.ensureDispatchMaps()
	now := time.Now()
	if last, ok := o.lastRecoveryAt[agentID]; ok && now.Sub(last) < recoveryGuardInterval {
		return nil
	}

	if err := o.registry.Restart(ctx, agentID); err != nil {
		return err
	}

	o.lastRecoveryAt[agentID] = now
	hold := o.readyTimeout
	if hold < 5*time.Second {
		hold = 5 * time.Second
	}
	if hold > 45*time.Second {
		hold = 45 * time.Second
	}
	o.agentRecoveryUntil[agentID] = now.Add(hold)

	o.mu.Lock()
	o.paneContent[agentID] = ""
	o.firstContentAt[agentID] = time.Time{}
	o.agentReady[agentID] = false
	o.fallbackReadyLog[agentID] = false
	o.mu.Unlock()

	_ = o.registry.UpdateStatus(agentID, types.StatusIdle)

	existing, _ := o.worldState.GetAgent(agentID)
	o.worldState.UpdateAgent(agentID, AgentState{
		Status:        types.StatusIdle,
		PromptDisplay: existing.PromptDisplay,
		Task:          existing.Task,
		PhaseID:       existing.PhaseID,
		AssignedAt:    existing.AssignedAt,
		LastActive:    time.Now().UTC(),
	})
	o.prevStatus[agentID] = types.StatusIdle

	o.logger.Warn("agent recovered via in-pane restart",
		"agent_id", agentID,
		"reason", reason,
		"hold_for", hold.Round(time.Second),
	)
	return nil
}

func (o *Orchestrator) cooldownDuration(failureCount int) time.Duration {
	retries := failureCount - dispatchFailureThreshold + 1
	if retries < 1 {
		retries = 1
	}
	backoff := dispatchBackoffBase
	for i := 1; i < retries; i++ {
		backoff *= 2
		if backoff >= dispatchBackoffMax {
			return dispatchBackoffMax
		}
	}
	return backoff
}

func (o *Orchestrator) notePromptFailure(phaseID types.PhaseID, promptNum int, reason string) {
	o.ensureDispatchMaps()
	key := makePromptKey(phaseID, promptNum)
	o.promptFailCount[key]++
	count := o.promptFailCount[key]
	if count < dispatchFailureThreshold {
		return
	}

	until := time.Now().Add(o.cooldownDuration(count))
	if current, ok := o.promptCooldownUntil[key]; ok && current.After(until) {
		until = current
	}
	o.promptCooldownUntil[key] = until
	delete(o.promptCooldownLogAt, key)

	o.logger.Warn("prompt dispatch backoff activated",
		"phase", phaseID,
		"prompt", promptNum,
		"failures", count,
		"cooldown", until.Sub(time.Now()).Round(time.Second),
		"reason", reason,
	)
}

func (o *Orchestrator) clearPromptFailure(phaseID types.PhaseID, promptNum int) {
	o.ensureDispatchMaps()
	key := makePromptKey(phaseID, promptNum)
	delete(o.promptFailCount, key)
	delete(o.promptCooldownUntil, key)
	delete(o.promptCooldownLogAt, key)
}

func (o *Orchestrator) logPromptCooldown(key promptKey, until time.Time) {
	o.ensureDispatchMaps()
	now := time.Now()
	if last, ok := o.promptCooldownLogAt[key]; ok && now.Sub(last) < cooldownLogInterval {
		return
	}
	o.promptCooldownLogAt[key] = now
	o.logger.Info("assignment backoff active",
		"phase", key.phaseID,
		"prompt", key.promptNum,
		"cooldown_remaining", until.Sub(now).Round(time.Second),
		"failures", o.promptFailCount[key],
	)
}

// latestContent returns the most recently captured pane content for an agent.
func (o *Orchestrator) latestContent(id types.AgentID) string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.paneContent[id]
}

// LatestContent returns the most recently captured pane content for an agent.
// This is the exported variant for use by the TUI.
func (o *Orchestrator) LatestContent(id types.AgentID) string {
	return o.latestContent(id)
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

	snap := o.worldState.Snapshot()
	if o.session.Agents == nil {
		o.session.Agents = make(map[string]session.AgentState)
	}
	for id, st := range snap.Agents {
		task := st.Task
		if task == "" {
			task = st.PromptDisplay
		}
		o.session.Agents[string(id)] = session.AgentState{
			Status:      string(st.Status),
			CurrentTask: task,
			UpdatedAt:   st.LastActive,
		}
	}
	for id := range o.session.Agents {
		if _, ok := snap.Agents[types.AgentID(id)]; !ok {
			delete(o.session.Agents, id)
		}
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
