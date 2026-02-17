// Package orchestrator implements the control loop that coordinates agents,
// phases, and decisions for the crux multi-agent orchestrator.
package orchestrator

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/roygabriel/crux/pkg/types"
)

// AgentState captures the observable state of an agent for world state tracking.
type AgentState struct {
	// Status is the agent's current operational status.
	Status types.AgentStatus `json:"status"`
	// PromptDisplay is a short label describing the active prompt.
	PromptDisplay string `json:"prompt"`
	// Task describes the work the agent is performing.
	Task string `json:"task"`
	// LastDecision is the most recent decision the agent made.
	LastDecision string `json:"last_decision,omitempty"`
	// LastActive is the last time the agent reported activity.
	LastActive time.Time `json:"last_active"`
	// PhaseID is the phase currently assigned to this agent.
	PhaseID types.PhaseID `json:"phase_id,omitempty"`
	// AssignedAt is when the agent was assigned its current work.
	AssignedAt time.Time `json:"assigned_at,omitempty"`
}

// WorldState is a thread-safe snapshot of the orchestration state.
// It is used for status display and compact JSON injection into prompts.
type WorldState struct {
	mu             sync.RWMutex
	SessionID      string                       `json:"session_id"`
	Phase          types.PhaseID                `json:"phase"`
	PhaseName      string                       `json:"phase_name"`
	Agents         map[types.AgentID]AgentState `json:"agents"`
	GatesPassed    []string                     `json:"gates_passed"`
	GatesPending   []string                     `json:"gates_pending"`
	DecisionsToday int                          `json:"decisions_today,omitempty"`
	OpenQuestions  int                          `json:"open_questions,omitempty"`
	UpdatedAt      time.Time                    `json:"updated_at"`
}

// NewWorldState creates an initialized WorldState for the given session.
func NewWorldState(sessionID string) *WorldState {
	return &WorldState{
		SessionID: sessionID,
		Agents:    make(map[types.AgentID]AgentState),
		UpdatedAt: time.Now().UTC(),
	}
}

// UpdateAgent sets the state for an agent. It is safe for concurrent use.
func (w *WorldState) UpdateAgent(id types.AgentID, state AgentState) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.Agents[id] = state
	w.UpdatedAt = time.Now().UTC()
}

// UpdatePhase sets the current phase. It is safe for concurrent use.
func (w *WorldState) UpdatePhase(phase types.PhaseID, name string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.Phase = phase
	w.PhaseName = name
	w.UpdatedAt = time.Now().UTC()
}

// GetAgent returns the state for a specific agent. It is safe for concurrent use.
func (w *WorldState) GetAgent(id types.AgentID) (AgentState, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	state, ok := w.Agents[id]
	return state, ok
}

// compactAgent is the minimal agent representation for prompt injection.
type compactAgent struct {
	Status string `json:"status"`
	Prompt string `json:"prompt,omitempty"`
}

// compactState is a stripped-down WorldState for LLM prompt injection.
type compactState struct {
	Phase        string                  `json:"phase,omitempty"`
	PhaseName    string                  `json:"phase_name,omitempty"`
	Agents       map[string]compactAgent `json:"agents,omitempty"`
	GatesPassed  int                     `json:"gates_passed,omitempty"`
	GatesPending int                     `json:"gates_pending,omitempty"`
	Decisions    int                     `json:"decisions_today,omitempty"`
	Questions    int                     `json:"open_questions,omitempty"`
}

// Compact returns a minimal JSON representation suitable for LLM prompt
// injection. It strips fields the LLM doesn't need and collapses gate
// lists to counts. Returns "{}" on marshal error.
func (w *WorldState) Compact() string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	agents := make(map[string]compactAgent, len(w.Agents))
	for id, a := range w.Agents {
		agents[string(id)] = compactAgent{
			Status: string(a.Status),
			Prompt: a.PromptDisplay,
		}
	}

	cs := compactState{
		Phase:        string(w.Phase),
		PhaseName:    w.PhaseName,
		Agents:       agents,
		GatesPassed:  len(w.GatesPassed),
		GatesPending: len(w.GatesPending),
		Decisions:    w.DecisionsToday,
		Questions:    w.OpenQuestions,
	}

	data, err := json.Marshal(cs)
	if err != nil {
		return "{}"
	}
	return string(data)
}
