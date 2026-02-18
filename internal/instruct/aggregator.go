package instruct

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// MemoryBankReader reads memory bank files for context injection.
type MemoryBankReader interface {
	Read(filename string) (string, error)
	Summary() (string, error)
}

// PhaseProvider provides current phase state.
type PhaseProvider interface {
	CurrentPhaseID() string
	CurrentPhaseName() string
	CurrentProgress() string
	CurrentDependencies() []string
	CurrentExitCriteria() []string
	CurrentFilesInScope() []string
	CurrentFilesReadOnly() []string
}

// AgentLister provides agent information.
type AgentLister interface {
	ListAgents() []AgentInfo
	GetAgent(id string) (AgentInfo, bool)
}

// AgentInfo holds agent metadata returned by an AgentLister.
type AgentInfo struct {
	// ID is the agent identifier.
	ID string `json:"id"`
	// Plugin is the CLI tool name (claude, codex, gemini).
	Plugin string `json:"plugin"`
	// Role is the functional role.
	Role string `json:"role"`
	// Permission is the security tier.
	Permission string `json:"permission"`
	// Model is the LLM model identifier.
	Model string `json:"model,omitempty"`
	// Status is the current operational status.
	Status string `json:"status"`
}

// MCPRegistry provides MCP tool availability.
type MCPRegistry interface {
	CompactSummary() string
	GetToolsByAgent(agentID string) []string
	GetAllTools() map[string][]string
}

// SkillsRegistry provides skill availability.
type SkillsRegistry interface {
	GetByAgent(agentPlugin string) []string
	Available() []string
}

// PreferenceStore provides engineering preference instructions.
type PreferenceStore interface {
	GetInstructions() PreferenceInstructions
}

// RoleProvider provides role definitions.
type RoleProvider interface {
	GetRole(name RoleName) RoleContext
}

// AggregatorDeps groups the dependencies for the Aggregator constructor.
type AggregatorDeps struct {
	// Config provides project-level configuration.
	Config AggregatorConfig
	// Bank reads memory bank files. May be nil.
	Bank MemoryBankReader
	// Phase provides current phase state. May be nil.
	Phase PhaseProvider
	// MCPReg provides MCP tool availability. May be nil.
	MCPReg MCPRegistry
	// SkillReg provides skill availability. May be nil.
	SkillReg SkillsRegistry
	// AgentReg provides agent information. May be nil.
	AgentReg AgentLister
	// Prefs provides engineering preferences. May be nil.
	Prefs PreferenceStore
	// Roles provides role definitions. May be nil.
	Roles RoleProvider
	// Logger is the structured logger. Falls back to slog.Default() if nil.
	Logger *slog.Logger
	// Version is the Crux binary version string.
	Version string
}

// AggregatorConfig is the minimal config interface needed by the aggregator.
type AggregatorConfig struct {
	// ProjectName is the project name.
	ProjectName string `json:"project_name"`
	// ProjectDescription is a short project description.
	ProjectDescription string `json:"project_description,omitempty"`
	// Language is the primary programming language.
	Language string `json:"language,omitempty"`
	// Frameworks lists frameworks in use.
	Frameworks []string `json:"frameworks,omitempty"`
	// RepoRoot is the project root path.
	RepoRoot string `json:"repo_root"`
	// KeyConcerns lists cross-cutting concerns.
	KeyConcerns []string `json:"key_concerns,omitempty"`
}

// Aggregator assembles InstructionData from all subsystems.
type Aggregator struct {
	config   AggregatorConfig
	bank     MemoryBankReader
	phase    PhaseProvider
	mcpReg   MCPRegistry
	skillReg SkillsRegistry
	agentReg AgentLister
	prefs    PreferenceStore
	roles    RoleProvider
	logger   *slog.Logger
	version  string
}

// NewAggregator creates an Aggregator from the provided dependencies.
// All subsystem dependencies are optional and nil-safe.
func NewAggregator(deps AggregatorDeps) *Aggregator {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Aggregator{
		config:   deps.Config,
		bank:     deps.Bank,
		phase:    deps.Phase,
		mcpReg:   deps.MCPReg,
		skillReg: deps.SkillReg,
		agentReg: deps.AgentReg,
		prefs:    deps.Prefs,
		roles:    deps.Roles,
		logger:   logger,
		version:  deps.Version,
	}
}

// Build assembles InstructionData for a specific agent and role.
// Every subsystem read is nil-safe: if a subsystem is not configured,
// the corresponding context field is left empty.
func (a *Aggregator) Build(_ context.Context, agentID string, role RoleName) (*InstructionData, error) {
	data := &InstructionData{
		GeneratedAt: time.Now().UTC(),
		CruxVersion: a.version,
	}

	a.populateProject(data)
	a.populatePhase(data)
	a.populateAgent(data, agentID)
	a.populateRole(data, role)
	a.populatePrefs(data)
	a.populateMemory(data)
	a.populateMCP(data, agentID)
	a.populateSkills(data, agentID)
	a.populateTeam(data, agentID)

	return data, nil
}

// BuildForOrchestrator assembles InstructionData for the orchestrator role,
// including information about ALL agents in the session.
func (a *Aggregator) BuildForOrchestrator(ctx context.Context) (*InstructionData, error) {
	data, err := a.Build(ctx, "orchestrator", RolePlanner)
	if err != nil {
		return nil, fmt.Errorf("building orchestrator data: %w", err)
	}

	// Include all agents in team context (don't exclude self).
	if a.agentReg != nil {
		agents := a.agentReg.ListAgents()
		data.Team.Agents = make([]TeamMember, 0, len(agents))
		for _, ag := range agents {
			data.Team.Agents = append(data.Team.Agents, TeamMember{
				ID:     ag.ID,
				Role:   RoleName(ag.Role),
				CLI:    AgentCLI(ag.Plugin),
				Status: ag.Status,
			})
		}
	}

	// Include all tools (not filtered by agent).
	if a.mcpReg != nil {
		data.MCP.AllTools = a.mcpReg.GetAllTools()
	}

	return data, nil
}

func (a *Aggregator) populateProject(data *InstructionData) {
	data.Project = ProjectContext{
		Name:        a.config.ProjectName,
		Description: a.config.ProjectDescription,
		Language:    a.config.Language,
		Frameworks:  a.config.Frameworks,
		RepoRoot:    a.config.RepoRoot,
		KeyConcerns: a.config.KeyConcerns,
	}
}

func (a *Aggregator) populatePhase(data *InstructionData) {
	if a.phase == nil {
		return
	}
	data.Phase = PhaseContext{
		CurrentID:     a.phase.CurrentPhaseID(),
		CurrentName:   a.phase.CurrentPhaseName(),
		Progress:      a.phase.CurrentProgress(),
		Dependencies:  a.phase.CurrentDependencies(),
		ExitCriteria:  a.phase.CurrentExitCriteria(),
		FilesInScope:  a.phase.CurrentFilesInScope(),
		FilesReadOnly: a.phase.CurrentFilesReadOnly(),
	}
}

func (a *Aggregator) populateAgent(data *InstructionData, agentID string) {
	data.Agent.ID = agentID

	if a.agentReg == nil {
		return
	}
	info, ok := a.agentReg.GetAgent(agentID)
	if !ok {
		a.logger.Warn("agent not found in registry", "agent_id", agentID)
		return
	}
	data.Agent.CLI = AgentCLI(info.Plugin)
	data.Agent.Model = info.Model
	data.Agent.Permissions = info.Permission
}

func (a *Aggregator) populateRole(data *InstructionData, role RoleName) {
	if a.roles != nil {
		data.Role = a.roles.GetRole(role)
		return
	}
	// Fallback: set just the name.
	data.Role.Name = role
}

func (a *Aggregator) populatePrefs(data *InstructionData) {
	if a.prefs == nil {
		return
	}
	data.Prefs = a.prefs.GetInstructions()
}

func (a *Aggregator) populateMemory(data *InstructionData) {
	if a.bank == nil {
		return
	}

	bankFiles := map[string]*string{
		"project-brief.md":  &data.Memory.ProjectBrief,
		"active-context.md": &data.Memory.ActiveContext,
		"tech-context.md":   &data.Memory.TechContext,
		"system-patterns.md": &data.Memory.SystemPatterns,
	}

	for filename, target := range bankFiles {
		content, err := a.bank.Read(filename)
		if err != nil {
			a.logger.Warn("failed to read memory bank file", "file", filename, "error", err)
			continue
		}
		*target = content
	}
}

func (a *Aggregator) populateMCP(data *InstructionData, agentID string) {
	if a.mcpReg == nil {
		return
	}
	data.MCP.Summary = a.mcpReg.CompactSummary()
	data.MCP.AgentTools = a.mcpReg.GetToolsByAgent(agentID)
}

func (a *Aggregator) populateSkills(data *InstructionData, agentID string) {
	if a.skillReg == nil {
		return
	}
	data.Skills.Available = a.skillReg.Available()

	// Look up agent plugin to filter skills.
	if a.agentReg != nil {
		if info, ok := a.agentReg.GetAgent(agentID); ok {
			data.Skills.AgentSkills = a.skillReg.GetByAgent(info.Plugin)
		}
	}
}

func (a *Aggregator) populateTeam(data *InstructionData, currentAgentID string) {
	if a.agentReg == nil {
		return
	}
	agents := a.agentReg.ListAgents()
	data.Team.Agents = make([]TeamMember, 0, len(agents))
	for _, ag := range agents {
		if ag.ID == currentAgentID {
			continue
		}
		data.Team.Agents = append(data.Team.Agents, TeamMember{
			ID:     ag.ID,
			Role:   RoleName(ag.Role),
			CLI:    AgentCLI(ag.Plugin),
			Status: ag.Status,
		})
	}
}
