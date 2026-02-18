package instruct

import (
	"context"
	"errors"
	"testing"
)

// Mock implementations for testing.

type mockBank struct {
	files map[string]string
}

func (m *mockBank) Read(filename string) (string, error) {
	content, ok := m.files[filename]
	if !ok {
		return "", errors.New("file not found")
	}
	return content, nil
}

func (m *mockBank) Summary() (string, error) {
	return "mock summary", nil
}

type mockPhase struct{}

func (m *mockPhase) CurrentPhaseID() string        { return "14A" }
func (m *mockPhase) CurrentPhaseName() string       { return "Instruction Engine" }
func (m *mockPhase) CurrentProgress() string        { return "Prompt 2/4" }
func (m *mockPhase) CurrentDependencies() []string  { return []string{"13A"} }
func (m *mockPhase) CurrentExitCriteria() []string  { return []string{"go build ./...", "go test ./..."} }
func (m *mockPhase) CurrentFilesInScope() []string  { return []string{"internal/instruct/"} }
func (m *mockPhase) CurrentFilesReadOnly() []string { return []string{"internal/phase/template.go"} }

type mockAgentLister struct {
	agents []AgentInfo
}

func (m *mockAgentLister) ListAgents() []AgentInfo { return m.agents }

func (m *mockAgentLister) GetAgent(id string) (AgentInfo, bool) {
	for _, a := range m.agents {
		if a.ID == id {
			return a, true
		}
	}
	return AgentInfo{}, false
}

type mockMCPRegistry struct{}

func (m *mockMCPRegistry) CompactSummary() string { return "2 servers, 5 tools" }

func (m *mockMCPRegistry) GetToolsByAgent(_ string) []string {
	return []string{"read_file", "write_file"}
}

func (m *mockMCPRegistry) GetAllTools() map[string][]string {
	return map[string][]string{
		"eng-1": {"read_file", "write_file"},
		"rev-1": {"read_file"},
	}
}

type mockSkillsRegistry struct{}

func (m *mockSkillsRegistry) Available() []string { return []string{"git", "test", "lint"} }

func (m *mockSkillsRegistry) GetByAgent(plugin string) []string {
	if plugin == "claude" {
		return []string{"git", "test"}
	}
	return nil
}

type mockPreferenceStore struct{}

func (m *mockPreferenceStore) GetInstructions() PreferenceInstructions {
	return PreferenceInstructions{
		Testing:       "Table-driven tests",
		ErrorHandling: "Wrap with context",
	}
}

type mockRoleProvider struct{}

func (m *mockRoleProvider) GetRole(name RoleName) RoleContext {
	return RoleContext{
		Name:             name,
		Title:            "Software Engineer",
		Identity:         "You are a software engineer.",
		Responsibilities: []string{"implement code", "write tests"},
		Constraints:      []string{"no panics"},
		Communication:    []string{"report via structured message"},
	}
}

func fullDeps() AggregatorDeps {
	return AggregatorDeps{
		Config: AggregatorConfig{
			ProjectName: "crux",
			Language:    "Go",
			Frameworks:  []string{"cobra"},
			RepoRoot:    "/home/user/crux",
			KeyConcerns: []string{"security"},
		},
		Bank: &mockBank{
			files: map[string]string{
				"project-brief.md":   "Crux is a multi-agent orchestrator.",
				"active-context.md":  "Working on phase 14A.",
				"tech-context.md":    "Go 1.25, SQLite WASM.",
				"system-patterns.md": "Dependency injection.",
			},
		},
		Phase: &mockPhase{},
		MCPReg: &mockMCPRegistry{},
		SkillReg: &mockSkillsRegistry{},
		AgentReg: &mockAgentLister{
			agents: []AgentInfo{
				{ID: "eng-1", Plugin: "claude", Role: "software-engineer", Permission: "elevated", Model: "opus", Status: "busy"},
				{ID: "rev-1", Plugin: "claude", Role: "code-reviewer", Permission: "readonly", Status: "idle"},
			},
		},
		Prefs:   &mockPreferenceStore{},
		Roles:   &mockRoleProvider{},
		Version: "0.1.0",
	}
}

func TestBuildFullyPopulated(t *testing.T) {
	t.Parallel()

	agg := NewAggregator(fullDeps())
	data, err := agg.Build(context.Background(), "eng-1", RoleSoftwareEngineer)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Project.
	if data.Project.Name != "crux" {
		t.Errorf("Project.Name = %q, want %q", data.Project.Name, "crux")
	}
	if data.Project.Language != "Go" {
		t.Errorf("Project.Language = %q, want %q", data.Project.Language, "Go")
	}

	// Phase.
	if data.Phase.CurrentID != "14A" {
		t.Errorf("Phase.CurrentID = %q, want %q", data.Phase.CurrentID, "14A")
	}
	if data.Phase.Progress != "Prompt 2/4" {
		t.Errorf("Phase.Progress = %q, want %q", data.Phase.Progress, "Prompt 2/4")
	}

	// Agent.
	if data.Agent.ID != "eng-1" {
		t.Errorf("Agent.ID = %q, want %q", data.Agent.ID, "eng-1")
	}
	if data.Agent.CLI != CLIClaude {
		t.Errorf("Agent.CLI = %q, want %q", data.Agent.CLI, CLIClaude)
	}
	if data.Agent.Permissions != "elevated" {
		t.Errorf("Agent.Permissions = %q, want %q", data.Agent.Permissions, "elevated")
	}

	// Role.
	if data.Role.Name != RoleSoftwareEngineer {
		t.Errorf("Role.Name = %q, want %q", data.Role.Name, RoleSoftwareEngineer)
	}
	if len(data.Role.Responsibilities) == 0 {
		t.Error("Role.Responsibilities should be populated")
	}

	// Prefs.
	if data.Prefs.Testing != "Table-driven tests" {
		t.Errorf("Prefs.Testing = %q, want %q", data.Prefs.Testing, "Table-driven tests")
	}

	// Memory.
	if data.Memory.ProjectBrief == "" {
		t.Error("Memory.ProjectBrief should be populated")
	}
	if data.Memory.ActiveContext == "" {
		t.Error("Memory.ActiveContext should be populated")
	}

	// MCP.
	if data.MCP.Summary != "2 servers, 5 tools" {
		t.Errorf("MCP.Summary = %q", data.MCP.Summary)
	}
	if len(data.MCP.AgentTools) != 2 {
		t.Errorf("MCP.AgentTools length = %d, want 2", len(data.MCP.AgentTools))
	}

	// Skills.
	if len(data.Skills.Available) != 3 {
		t.Errorf("Skills.Available length = %d, want 3", len(data.Skills.Available))
	}
	if len(data.Skills.AgentSkills) != 2 {
		t.Errorf("Skills.AgentSkills length = %d, want 2", len(data.Skills.AgentSkills))
	}

	// Team (should exclude current agent).
	if len(data.Team.Agents) != 1 {
		t.Errorf("Team.Agents length = %d, want 1", len(data.Team.Agents))
	}
	if len(data.Team.Agents) > 0 && data.Team.Agents[0].ID != "rev-1" {
		t.Errorf("Team.Agents[0].ID = %q, want %q", data.Team.Agents[0].ID, "rev-1")
	}

	// Metadata.
	if data.CruxVersion != "0.1.0" {
		t.Errorf("CruxVersion = %q, want %q", data.CruxVersion, "0.1.0")
	}
	if data.GeneratedAt.IsZero() {
		t.Error("GeneratedAt should be set")
	}
}

func TestBuildNilBank(t *testing.T) {
	t.Parallel()

	deps := fullDeps()
	deps.Bank = nil
	agg := NewAggregator(deps)

	data, err := agg.Build(context.Background(), "eng-1", RoleSoftwareEngineer)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if data.Memory.ProjectBrief != "" {
		t.Error("Memory should be empty with nil bank")
	}
}

func TestBuildNilPhase(t *testing.T) {
	t.Parallel()

	deps := fullDeps()
	deps.Phase = nil
	agg := NewAggregator(deps)

	data, err := agg.Build(context.Background(), "eng-1", RoleSoftwareEngineer)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if data.Phase.CurrentID != "" {
		t.Error("Phase should be empty with nil phase provider")
	}
}

func TestBuildNilMCPReg(t *testing.T) {
	t.Parallel()

	deps := fullDeps()
	deps.MCPReg = nil
	agg := NewAggregator(deps)

	data, err := agg.Build(context.Background(), "eng-1", RoleSoftwareEngineer)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if data.MCP.Summary != "" {
		t.Error("MCP should be empty with nil registry")
	}
}

func TestBuildNilSkillReg(t *testing.T) {
	t.Parallel()

	deps := fullDeps()
	deps.SkillReg = nil
	agg := NewAggregator(deps)

	data, err := agg.Build(context.Background(), "eng-1", RoleSoftwareEngineer)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if len(data.Skills.Available) != 0 {
		t.Error("Skills should be empty with nil registry")
	}
}

func TestBuildNilAgentReg(t *testing.T) {
	t.Parallel()

	deps := fullDeps()
	deps.AgentReg = nil
	agg := NewAggregator(deps)

	data, err := agg.Build(context.Background(), "eng-1", RoleSoftwareEngineer)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if data.Agent.CLI != "" {
		t.Error("Agent.CLI should be empty with nil agent registry")
	}
	if len(data.Team.Agents) != 0 {
		t.Error("Team should be empty with nil agent registry")
	}
}

func TestBuildNilPrefs(t *testing.T) {
	t.Parallel()

	deps := fullDeps()
	deps.Prefs = nil
	agg := NewAggregator(deps)

	data, err := agg.Build(context.Background(), "eng-1", RoleSoftwareEngineer)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if data.Prefs.Testing != "" {
		t.Error("Prefs should be empty with nil preference store")
	}
}

func TestBuildNilRoles(t *testing.T) {
	t.Parallel()

	deps := fullDeps()
	deps.Roles = nil
	agg := NewAggregator(deps)

	data, err := agg.Build(context.Background(), "eng-1", RoleSoftwareEngineer)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if data.Role.Name != RoleSoftwareEngineer {
		t.Errorf("Role.Name = %q, want %q", data.Role.Name, RoleSoftwareEngineer)
	}
	// Title should be empty since no role provider.
	if data.Role.Title != "" {
		t.Error("Role.Title should be empty with nil role provider")
	}
}

func TestBuildNilLogger(t *testing.T) {
	t.Parallel()

	deps := fullDeps()
	deps.Logger = nil
	agg := NewAggregator(deps)

	data, err := agg.Build(context.Background(), "eng-1", RoleSoftwareEngineer)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if data.Project.Name != "crux" {
		t.Error("Build should succeed with nil logger")
	}
}

func TestBuildAllNil(t *testing.T) {
	t.Parallel()

	agg := NewAggregator(AggregatorDeps{
		Config: AggregatorConfig{
			ProjectName: "minimal",
		},
		Version: "0.1.0",
	})

	data, err := agg.Build(context.Background(), "eng-1", RoleSoftwareEngineer)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	if data.Project.Name != "minimal" {
		t.Errorf("Project.Name = %q, want %q", data.Project.Name, "minimal")
	}
	if data.CruxVersion != "0.1.0" {
		t.Errorf("CruxVersion = %q, want %q", data.CruxVersion, "0.1.0")
	}
}

func TestBuildForOrchestrator(t *testing.T) {
	t.Parallel()

	agg := NewAggregator(fullDeps())
	data, err := agg.BuildForOrchestrator(context.Background())
	if err != nil {
		t.Fatalf("BuildForOrchestrator() error: %v", err)
	}

	// Should include ALL agents (not exclude self like Build does).
	if len(data.Team.Agents) != 2 {
		t.Errorf("Team.Agents length = %d, want 2 (all agents)", len(data.Team.Agents))
	}

	// Should include all tools map.
	if len(data.MCP.AllTools) == 0 {
		t.Error("MCP.AllTools should be populated for orchestrator")
	}
}

func TestBuildUnknownAgent(t *testing.T) {
	t.Parallel()

	agg := NewAggregator(fullDeps())
	data, err := agg.Build(context.Background(), "nonexistent", RoleSoftwareEngineer)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Should still work, just with minimal agent info.
	if data.Agent.ID != "nonexistent" {
		t.Errorf("Agent.ID = %q, want %q", data.Agent.ID, "nonexistent")
	}
	if data.Agent.CLI != "" {
		t.Error("Agent.CLI should be empty for unknown agent")
	}
}

func TestBuildBankReadError(t *testing.T) {
	t.Parallel()

	// Bank that only has some files.
	deps := fullDeps()
	deps.Bank = &mockBank{
		files: map[string]string{
			"project-brief.md": "Brief content.",
			// Other files missing.
		},
	}

	agg := NewAggregator(deps)
	data, err := agg.Build(context.Background(), "eng-1", RoleSoftwareEngineer)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Should have the file that exists.
	if data.Memory.ProjectBrief != "Brief content." {
		t.Errorf("Memory.ProjectBrief = %q, want %q", data.Memory.ProjectBrief, "Brief content.")
	}

	// Missing files should be empty, not cause error.
	if data.Memory.ActiveContext != "" {
		t.Error("Missing bank file should result in empty field")
	}
}
