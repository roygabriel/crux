package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/roygabriel/crux/internal/agent"
	"github.com/roygabriel/crux/internal/phase"
	"github.com/roygabriel/crux/internal/plugin"
	"github.com/roygabriel/crux/pkg/types"
)

// ErrNoAvailableAgent is returned when no idle agent can accept work.
var ErrNoAvailableAgent = errors.New("no available agent")

// AgentLister provides agent information for assignment decisions.
type AgentLister interface {
	List() []*agent.AgentInstance
	Get(id types.AgentID) (*agent.AgentInstance, error)
	UpdateStatus(id types.AgentID, status types.AgentStatus) error
}

// PromptProvider exposes phase engine state for assignment.
type PromptProvider interface {
	CurrentPhase() *phase.PhaseSpec
	CurrentPrompt() *phase.PromptContract
}

// Assigner matches idle agents to pending prompts and updates world state.
type Assigner struct {
	agents     AgentLister
	prompts    PromptProvider
	worldState *WorldState
	logger     *slog.Logger
}

// NewAssigner creates an Assigner with the given dependencies.
func NewAssigner(agents AgentLister, prompts PromptProvider, worldState *WorldState, logger *slog.Logger) *Assigner {
	if logger == nil {
		logger = slog.Default()
	}
	return &Assigner{
		agents:     agents,
		prompts:    prompts,
		worldState: worldState,
		logger:     logger,
	}
}

// AssignNext finds the next pending prompt and assigns it to an idle agent.
// Returns nil if there is no prompt to assign. Returns ErrNoAvailableAgent
// if a prompt exists but no agent is idle.
func (a *Assigner) AssignNext(ctx context.Context) error {
	prompt := a.prompts.CurrentPrompt()
	if prompt == nil {
		return nil
	}

	spec := a.prompts.CurrentPhase()
	if spec == nil {
		return nil
	}

	idle := a.idleAgents()
	if len(idle) == 0 {
		return ErrNoAvailableAgent
	}

	// Pick the best agent: prefer capability match, fall back to first idle.
	selected := a.selectAgent(idle, prompt)

	if err := a.agents.UpdateStatus(selected.Agent.ID, types.StatusBusy); err != nil {
		return fmt.Errorf("assign next: update status: %w", err)
	}

	now := time.Now().UTC()
	a.worldState.UpdateAgent(selected.Agent.ID, AgentState{
		Status:        types.StatusBusy,
		PromptDisplay: fmt.Sprintf("Phase %s P%d", spec.ID, prompt.PromptNumber),
		Task:          prompt.Task,
		LastActive:    selected.LaunchedAt,
		PhaseID:       spec.ID,
		AssignedAt:    now,
	})

	a.logger.Info("assigned prompt to agent",
		"agent_id", selected.Agent.ID,
		"phase", spec.ID,
		"prompt", prompt.PromptNumber,
	)
	return nil
}

// AssignToAgent explicitly assigns a specific prompt to a named agent.
func (a *Assigner) AssignToAgent(ctx context.Context, agentID types.AgentID, phaseID types.PhaseID, promptNum int) error {
	inst, err := a.agents.Get(agentID)
	if err != nil {
		return fmt.Errorf("assign to agent: %w", err)
	}

	if err := a.agents.UpdateStatus(agentID, types.StatusBusy); err != nil {
		return fmt.Errorf("assign to agent: update status: %w", err)
	}

	a.worldState.UpdateAgent(agentID, AgentState{
		Status:        types.StatusBusy,
		PromptDisplay: fmt.Sprintf("Phase %s P%d", phaseID, promptNum),
		LastActive:    inst.LaunchedAt,
		PhaseID:       phaseID,
		AssignedAt:    time.Now().UTC(),
	})

	a.logger.Info("explicitly assigned prompt to agent",
		"agent_id", agentID,
		"phase", phaseID,
		"prompt", promptNum,
	)
	return nil
}

// idleAgents returns agents with StatusIdle.
func (a *Assigner) idleAgents() []*agent.AgentInstance {
	all := a.agents.List()
	var idle []*agent.AgentInstance
	for _, inst := range all {
		if inst.Agent.Status == types.StatusIdle {
			idle = append(idle, inst)
		}
	}
	return idle
}

// selectAgent picks the best idle agent for the given prompt.
// It prefers codex for boilerplate (code-gen only) and claude for complex tasks.
// Falls back to the first idle agent.
func (a *Assigner) selectAgent(idle []*agent.AgentInstance, prompt *phase.PromptContract) *agent.AgentInstance {
	isComplex := len(prompt.Items) > 3 || len(prompt.Constraints) > 2

	for _, inst := range idle {
		caps := inst.Plugin.Capabilities()
		hasCodeGen := containsCap(caps, plugin.CapCodeGen)
		hasShell := containsCap(caps, plugin.CapShellExec)

		if isComplex && hasCodeGen && hasShell {
			return inst
		}
		if !isComplex && hasCodeGen {
			return inst
		}
	}

	return idle[0]
}

// containsCap checks if a capability slice contains the given capability.
func containsCap(caps []plugin.Capability, target plugin.Capability) bool {
	for _, c := range caps {
		if c == target {
			return true
		}
	}
	return false
}
