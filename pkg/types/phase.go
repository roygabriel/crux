package types

// PhaseID uniquely identifies a project phase.
type PhaseID string

// String returns the string representation of a PhaseID.
func (id PhaseID) String() string { return string(id) }

// PhaseStatus represents the lifecycle state of a phase.
type PhaseStatus string

const (
	// PhasePlanned means the phase is defined but not yet started.
	PhasePlanned PhaseStatus = "planned"
	// PhaseActive means the phase is currently being executed.
	PhaseActive PhaseStatus = "active"
	// PhaseBlocked means the phase cannot proceed due to unmet dependencies.
	PhaseBlocked PhaseStatus = "blocked"
	// PhaseComplete means the phase has passed all exit criteria.
	PhaseComplete PhaseStatus = "complete"
)

// String returns the string representation of a PhaseStatus.
func (s PhaseStatus) String() string { return string(s) }

// Phase represents a discrete unit of project work containing one or more prompts.
type Phase struct {
	// ID uniquely identifies this phase.
	ID PhaseID `json:"id"`
	// Name is the human-readable title.
	Name string `json:"name"`
	// Status is the current lifecycle state.
	Status PhaseStatus `json:"status"`
	// DependsOn lists phase IDs that must complete before this phase can start.
	DependsOn []PhaseID `json:"depends_on,omitempty"`
	// TotalPrompts is the number of prompts in this phase.
	TotalPrompts int `json:"total_prompts"`
	// CurrentPrompt is the index of the prompt currently being executed.
	CurrentPrompt int `json:"current_prompt"`
}
