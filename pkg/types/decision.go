package types

import "time"

// Decision records an orchestration decision made during prompt execution.
type Decision struct {
	// ID uniquely identifies this decision.
	ID string `json:"id"`
	// Timestamp is when the decision was made.
	Timestamp time.Time `json:"timestamp"`
	// PhaseID is the phase in which the decision occurred.
	PhaseID PhaseID `json:"phase_id"`
	// PromptNum is the prompt number within the phase.
	PromptNum int `json:"prompt_num"`
	// AgentID is the agent that made or triggered the decision.
	AgentID AgentID `json:"agent_id"`
	// Context describes the situation that prompted the decision.
	Context string `json:"context"`
	// Rationale explains why this action was chosen.
	Rationale string `json:"rationale"`
	// Action is the concrete step taken.
	Action string `json:"action"`
	// Outcome records the result of the action, if known.
	Outcome string `json:"outcome,omitempty"`
}
