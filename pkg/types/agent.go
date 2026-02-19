package types

// AgentID uniquely identifies an agent instance.
type AgentID string

// String returns the string representation of an AgentID.
func (id AgentID) String() string { return string(id) }

// AgentRole defines the functional role an agent plays in the orchestration.
type AgentRole string

const (
	// RoleOrchestrator is the top-level control agent.
	RoleOrchestrator AgentRole = "orchestrator"
	// RolePlanner coordinates phase planning and dependency ordering.
	RolePlanner AgentRole = "planner"
	// RoleProjectManager coordinates task assignment and phase progression.
	RoleProjectManager AgentRole = "project-manager"
	// RoleSoftwareEngineer executes implementation prompts.
	RoleSoftwareEngineer AgentRole = "software-engineer"
	// RoleSystemsEngineer handles infrastructure and tooling prompts.
	RoleSystemsEngineer AgentRole = "systems-engineer"
	// RoleCodeReviewer performs code review and quality checks.
	RoleCodeReviewer AgentRole = "code-reviewer"

	// RoleEngineer is a legacy alias for RoleSoftwareEngineer.
	// Kept for backward compatibility with existing config files.
	RoleEngineer AgentRole = "engineer"
	// RoleReviewer is a legacy alias for RoleCodeReviewer.
	// Kept for backward compatibility with existing config files.
	RoleReviewer AgentRole = "reviewer"
)

// NormalizeAgentRole maps legacy role names to their current equivalents.
// Unknown roles pass through unchanged.
func NormalizeAgentRole(role AgentRole) AgentRole {
	switch role {
	case RoleEngineer:
		return RoleSoftwareEngineer
	case RoleReviewer:
		return RoleCodeReviewer
	default:
		return role
	}
}

// String returns the string representation of an AgentRole.
func (r AgentRole) String() string { return string(r) }

// AgentStatus represents the current operational state of an agent.
type AgentStatus string

const (
	// StatusIdle means the agent is ready to accept work.
	StatusIdle AgentStatus = "idle"
	// StatusBusy means the agent is actively processing a task.
	StatusBusy AgentStatus = "busy"
	// StatusError means the agent encountered an unrecoverable error.
	StatusError AgentStatus = "error"
	// StatusRateLimited means the agent hit an API rate limit.
	StatusRateLimited AgentStatus = "rate-limited"
	// StatusPrompted means the agent is waiting at an interactive prompt.
	StatusPrompted AgentStatus = "prompted"
	// StatusStopped means the agent has been explicitly stopped.
	StatusStopped AgentStatus = "stopped"
	// StatusVerifying means gate/evidence verification is running.
	StatusVerifying AgentStatus = "verifying"
	// StatusReviewing means reviewer evaluation is running.
	StatusReviewing AgentStatus = "reviewing"
	// StatusQuarantined means prompt execution has been terminally quarantined.
	StatusQuarantined AgentStatus = "quarantined"
)

// String returns the string representation of an AgentStatus.
func (s AgentStatus) String() string { return string(s) }

// Agent represents a running agent instance managed by the orchestrator.
type Agent struct {
	// ID is the unique identifier for this agent instance.
	ID AgentID `json:"id"`
	// Name is the human-readable label for the agent.
	Name string `json:"name"`
	// Plugin is the adapter used to drive this agent (claude, codex, gemini, generic).
	Plugin string `json:"plugin"`
	// Role is the functional role this agent plays.
	Role AgentRole `json:"role"`
	// Status is the current operational state.
	Status AgentStatus `json:"status"`
	// Permission is the security tier granted to this agent.
	Permission Permission `json:"permission"`
	// CurrentTask is the description of the task the agent is working on, if any.
	CurrentTask string `json:"current_task,omitempty"`
	// PaneID is the tmux pane identifier where the agent runs.
	PaneID string `json:"pane_id,omitempty"`
	// SessionID is the orchestrator session this agent belongs to.
	SessionID string `json:"session_id,omitempty"`
}
