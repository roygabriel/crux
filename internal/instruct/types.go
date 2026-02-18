// Package instruct generates per-agent instruction files from a universal
// template. It aggregates context from every Crux subsystem into a single
// InstructionData struct, renders it through text/template with [[ ]]
// delimiters, and enforces per-agent token budgets by dropping low-priority
// sections first.
package instruct

import "time"

// AgentCLI identifies the CLI tool used by an agent.
type AgentCLI string

const (
	// CLIClaude is the Claude Code CLI.
	CLIClaude AgentCLI = "claude"
	// CLICodex is the OpenAI Codex CLI.
	CLICodex AgentCLI = "codex"
	// CLIGemini is the Google Gemini CLI.
	CLIGemini AgentCLI = "gemini"
	// CLICopilot is the GitHub Copilot CLI.
	CLICopilot AgentCLI = "copilot"
)

// String returns the string representation of an AgentCLI.
func (c AgentCLI) String() string { return string(c) }

// RoleName identifies a functional role in the orchestration.
type RoleName string

const (
	// RolePlanner coordinates phase planning and dependency ordering.
	RolePlanner RoleName = "planner"
	// RoleProjectManager coordinates task assignment and phase progression.
	RoleProjectManager RoleName = "project-manager"
	// RoleSoftwareEngineer executes implementation prompts.
	RoleSoftwareEngineer RoleName = "software-engineer"
	// RoleSystemsEngineer handles infrastructure and tooling prompts.
	RoleSystemsEngineer RoleName = "systems-engineer"
	// RoleCodeReviewer performs code review and quality checks.
	RoleCodeReviewer RoleName = "code-reviewer"
)

// String returns the string representation of a RoleName.
func (r RoleName) String() string { return string(r) }

// SectionPriority determines the order in which sections are dropped when
// the rendered output exceeds the token budget. Lower values are higher
// priority and are dropped last.
type SectionPriority int

const (
	// PriorityCritical sections are always included regardless of budget.
	PriorityCritical SectionPriority = 0
	// PriorityHigh sections are dropped only after Low and Medium.
	PriorityHigh SectionPriority = 1
	// PriorityMedium sections are dropped after Low but before High.
	PriorityMedium SectionPriority = 2
	// PriorityLow sections are dropped first when budget is exceeded.
	PriorityLow SectionPriority = 3
)

// String returns the string representation of a SectionPriority.
func (p SectionPriority) String() string {
	switch p {
	case PriorityCritical:
		return "critical"
	case PriorityHigh:
		return "high"
	case PriorityMedium:
		return "medium"
	case PriorityLow:
		return "low"
	default:
		return "unknown"
	}
}

// InstructionData is the root data structure passed to the universal template.
// It aggregates context from every Crux subsystem.
type InstructionData struct {
	// Project holds project-level identification and configuration.
	Project ProjectContext `json:"project"`
	// Phase holds the current phase execution state.
	Phase PhaseContext `json:"phase"`
	// Agent holds the identity of the agent being instructed.
	Agent AgentContext `json:"agent"`
	// Role holds the role definition for this agent.
	Role RoleContext `json:"role"`
	// Prefs holds compiled engineering preference instructions.
	Prefs PreferenceInstructions `json:"prefs"`
	// Memory holds memory bank summaries for context injection.
	Memory MemoryContext `json:"memory"`
	// MCP holds MCP server and tool availability.
	MCP MCPContext `json:"mcp"`
	// Skills holds skill availability for this agent.
	Skills SkillsContext `json:"skills"`
	// Team holds information about other agents in the session.
	Team TeamContext `json:"team"`
	// Custom holds user-provided custom sections keyed by heading.
	Custom map[string]string `json:"custom,omitempty"`
	// GeneratedAt is the timestamp when instructions were rendered.
	GeneratedAt time.Time `json:"generated_at"`
	// CruxVersion is the version of the Crux binary.
	CruxVersion string `json:"crux_version"`
}

// ProjectContext holds project-level identification and configuration.
type ProjectContext struct {
	// Name is the human-readable project name.
	Name string `json:"name"`
	// Description is a short project description.
	Description string `json:"description,omitempty"`
	// Language is the primary programming language.
	Language string `json:"language"`
	// Frameworks lists frameworks and major libraries in use.
	Frameworks []string `json:"frameworks,omitempty"`
	// RepoRoot is the absolute path to the project root.
	RepoRoot string `json:"repo_root"`
	// KeyConcerns lists cross-cutting concerns (e.g., security, performance).
	KeyConcerns []string `json:"key_concerns,omitempty"`
}

// PhaseContext holds the current phase execution state.
type PhaseContext struct {
	// CurrentID is the phase identifier (e.g., "14A").
	CurrentID string `json:"current_id"`
	// CurrentName is the human-readable phase title.
	CurrentName string `json:"current_name"`
	// Progress describes prompt progress (e.g., "Prompt 2/4").
	Progress string `json:"progress"`
	// Dependencies lists phase IDs this phase depends on.
	Dependencies []string `json:"dependencies,omitempty"`
	// ExitCriteria lists the verification gates for phase completion.
	ExitCriteria []string `json:"exit_criteria,omitempty"`
	// FilesInScope lists files the agent may create or modify.
	FilesInScope []string `json:"files_in_scope,omitempty"`
	// FilesReadOnly lists files the agent should read but not modify.
	FilesReadOnly []string `json:"files_read_only,omitempty"`
}

// AgentContext holds the identity of the agent being instructed.
type AgentContext struct {
	// ID is the unique agent identifier.
	ID string `json:"id"`
	// CLI is the CLI tool this agent uses.
	CLI AgentCLI `json:"cli"`
	// Model is the LLM model identifier.
	Model string `json:"model,omitempty"`
	// Permissions is the security tier (readonly, standard, elevated, autonomous).
	Permissions string `json:"permissions"`
	// Branch is the git branch assigned to this agent.
	Branch string `json:"branch,omitempty"`
}

// RoleContext holds the full role definition for an agent.
type RoleContext struct {
	// Name is the role identifier.
	Name RoleName `json:"name"`
	// Title is the human-readable role title.
	Title string `json:"title"`
	// Identity is the "You are a..." statement.
	Identity string `json:"identity"`
	// Responsibilities lists what this role is expected to do.
	Responsibilities []string `json:"responsibilities,omitempty"`
	// Constraints lists hard rules this role must follow.
	Constraints []string `json:"constraints,omitempty"`
	// Communication lists communication protocol rules.
	Communication []string `json:"communication,omitempty"`
	// ReviewFocus lists review priorities (for code-reviewer role).
	ReviewFocus []string `json:"review_focus,omitempty"`
	// PlanningRules lists planning directives (for planner role).
	PlanningRules []string `json:"planning_rules,omitempty"`
}

// PreferenceInstructions holds compiled engineering preference text.
type PreferenceInstructions struct {
	// Testing describes testing standards and expectations.
	Testing string `json:"testing,omitempty"`
	// ErrorHandling describes error handling patterns.
	ErrorHandling string `json:"error_handling,omitempty"`
	// Organization describes code organization preferences.
	Organization string `json:"organization,omitempty"`
	// Naming describes naming conventions.
	Naming string `json:"naming,omitempty"`
	// Abstraction describes abstraction and DRY preferences.
	Abstraction string `json:"abstraction,omitempty"`
	// Documentation describes documentation standards.
	Documentation string `json:"documentation,omitempty"`
	// Formatting describes formatting rules.
	Formatting string `json:"formatting,omitempty"`
	// Dependencies describes dependency management rules.
	Dependencies string `json:"dependencies,omitempty"`
	// Architecture describes architectural preferences.
	Architecture string `json:"architecture,omitempty"`
}

// MemoryContext holds memory bank summaries for context injection.
type MemoryContext struct {
	// ProjectBrief is the project brief content.
	ProjectBrief string `json:"project_brief,omitempty"`
	// ActiveContext is the current active context summary.
	ActiveContext string `json:"active_context,omitempty"`
	// TechContext is the technical context summary.
	TechContext string `json:"tech_context,omitempty"`
	// SystemPatterns describes established system patterns.
	SystemPatterns string `json:"system_patterns,omitempty"`
	// RecentDecisions lists recent decision summaries.
	RecentDecisions []string `json:"recent_decisions,omitempty"`
}

// MCPContext holds MCP server and tool availability.
type MCPContext struct {
	// Summary is a compact one-line MCP status.
	Summary string `json:"summary,omitempty"`
	// AgentTools lists tool names available to this specific agent.
	AgentTools []string `json:"agent_tools,omitempty"`
	// AllTools maps agent IDs to their available tool names.
	AllTools map[string][]string `json:"all_tools,omitempty"`
}

// SkillsContext holds skill availability for an agent.
type SkillsContext struct {
	// Available lists all registered skill names.
	Available []string `json:"available,omitempty"`
	// AgentSkills lists skill names available to this specific agent.
	AgentSkills []string `json:"agent_skills,omitempty"`
}

// TeamContext holds information about other agents in the session.
type TeamContext struct {
	// Agents lists the other agents participating in this session.
	Agents []TeamMember `json:"agents,omitempty"`
}

// TeamMember describes another agent in the session.
type TeamMember struct {
	// ID is the agent identifier.
	ID string `json:"id"`
	// Role is the agent's functional role.
	Role RoleName `json:"role"`
	// CLI is the CLI tool the agent uses.
	CLI AgentCLI `json:"cli"`
	// Status is the agent's current operational status.
	Status string `json:"status"`
}
