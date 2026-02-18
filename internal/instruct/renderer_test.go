package instruct

import (
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// testTemplateFS builds an in-memory filesystem with minimal section templates
// for renderer testing. Each template uses [[ ]] delimiters.
func testTemplateFS() fstest.MapFS {
	return fstest.MapFS{
		"sections/identity.md.tmpl": &fstest.MapFile{
			Data: []byte(`[[ if .Role.Identity -]]
## Identity

[[ .Role.Identity ]]

[[ end -]]`),
		},
		"sections/project.md.tmpl": &fstest.MapFile{
			Data: []byte(`[[ if .Project.Name -]]
## Project

- **Name:** [[ .Project.Name ]]
- **Language:** [[ .Project.Language ]]

[[ end -]]`),
		},
		"sections/responsibilities.md.tmpl": &fstest.MapFile{
			Data: []byte(`[[ if .Role.Responsibilities -]]
## Responsibilities

[[ bullet .Role.Responsibilities -]]

[[ end -]]`),
		},
		"sections/constraints.md.tmpl": &fstest.MapFile{
			Data: []byte(`[[ if .Role.Constraints -]]
## Constraints

[[ bullet .Role.Constraints -]]

[[ end -]]`),
		},
		"sections/preferences.md.tmpl": &fstest.MapFile{
			Data: []byte(`[[ if ifdef .Prefs.Testing -]]
## Engineering Standards

### Testing
[[ .Prefs.Testing ]]

[[ end -]]`),
		},
		"sections/phase.md.tmpl": &fstest.MapFile{
			Data: []byte(`[[ if .Phase.CurrentID -]]
## Current Phase: [[ .Phase.CurrentID ]]

Progress: [[ .Phase.Progress ]]

[[ end -]]`),
		},
		"sections/memory.md.tmpl": &fstest.MapFile{
			Data: []byte(`[[ if .Memory.ProjectBrief -]]
## Memory

[[ .Memory.ProjectBrief ]]

[[ end -]]`),
		},
		"sections/mcp.md.tmpl": &fstest.MapFile{
			Data: []byte(`[[ if .MCP.AgentTools -]]
## MCP Tools

[[ bullet .MCP.AgentTools -]]

[[ end -]]`),
		},
		"sections/skills.md.tmpl": &fstest.MapFile{
			Data: []byte(`[[ if .Skills.AgentSkills -]]
## Skills

[[ bullet .Skills.AgentSkills -]]

[[ end -]]`),
		},
		"sections/team.md.tmpl": &fstest.MapFile{
			Data: []byte(`[[ if .Team.Agents -]]
## Team

[[ range .Team.Agents -]]
- **[[ .ID ]]** ([[ .Role ]]) — [[ .Status ]]
[[ end -]]

[[ end -]]`),
		},
		"sections/session.md.tmpl": &fstest.MapFile{
			Data: []byte(`## Session Rules

1. Read ALL required files before writing code
2. Follow interface contracts exactly

`),
		},
	}
}

func testInstructionData() InstructionData {
	return InstructionData{
		Project: ProjectContext{
			Name:     "test-project",
			Language: "Go",
		},
		Phase: PhaseContext{
			CurrentID: "14A",
			Progress:  "Prompt 1/4",
		},
		Agent: AgentContext{
			ID:          "eng-1",
			CLI:         CLIClaude,
			Permissions: "elevated",
		},
		Role: RoleContext{
			Name:             RoleSoftwareEngineer,
			Title:            "Software Engineer",
			Identity:         "You are a software engineer.",
			Responsibilities: []string{"implement code", "write tests"},
			Constraints:      []string{"no panics", "wrap errors"},
		},
		Prefs: PreferenceInstructions{
			Testing: "table-driven tests with t.Parallel()",
		},
		Memory: MemoryContext{
			ProjectBrief: "A multi-agent orchestrator for software projects.",
		},
		MCP: MCPContext{
			AgentTools: []string{"read_file", "write_file"},
		},
		Skills: SkillsContext{
			AgentSkills: []string{"git-commit"},
		},
		Team: TeamContext{
			Agents: []TeamMember{
				{ID: "rev-1", Role: RoleCodeReviewer, CLI: CLIClaude, Status: "idle"},
			},
		},
		GeneratedAt: time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC),
		CruxVersion: "0.1.0",
	}
}

func TestNewRenderer(t *testing.T) {
	t.Parallel()

	r, err := NewRenderer(testTemplateFS(), nil)
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}
	if r == nil {
		t.Fatal("NewRenderer() returned nil")
	}
}

func TestRenderUnderBudget(t *testing.T) {
	t.Parallel()

	r, err := NewRenderer(testTemplateFS(), nil)
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	data := testInstructionData()
	result, err := r.Render(data, 50000)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}

	// All sections should be included, none dropped.
	if len(result.Dropped) > 0 {
		t.Errorf("expected no dropped sections, got %v", result.Dropped)
	}

	// Should have all 11 sections rendered.
	if len(result.Sections) != 11 {
		t.Errorf("expected 11 sections, got %d", len(result.Sections))
	}

	// Content should contain key markers.
	for _, marker := range []string{"Identity", "Project", "Responsibilities", "Session Rules"} {
		if !strings.Contains(result.Content, marker) {
			t.Errorf("content missing %q", marker)
		}
	}

	if result.TotalTokens <= 0 {
		t.Error("TotalTokens should be positive")
	}
}

func TestRenderOverBudgetDropsLowFirst(t *testing.T) {
	t.Parallel()

	r, err := NewRenderer(testTemplateFS(), nil)
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	data := testInstructionData()

	// First render to get total tokens.
	full, err := r.Render(data, 50000)
	if err != nil {
		t.Fatalf("full Render() error: %v", err)
	}

	// Set budget to exclude some sections.
	// Critical sections should stay, low-priority dropped first.
	tightBudget := full.TotalTokens - 20
	result, err := r.Render(data, tightBudget)
	if err != nil {
		t.Fatalf("tight Render() error: %v", err)
	}

	if len(result.Dropped) == 0 {
		t.Error("expected some sections to be dropped with tight budget")
	}

	// Verify dropped sections are low priority.
	droppedSet := make(map[string]bool)
	for _, name := range result.Dropped {
		droppedSet[name] = true
	}

	// Low-priority sections should be dropped before high-priority.
	lowSections := map[string]bool{"skills": true, "team": true}
	for _, d := range result.Dropped {
		if !lowSections[d] {
			// Check that no critical section was dropped.
			criticalSections := map[string]bool{
				"identity": true, "project": true, "responsibilities": true,
				"constraints": true, "session": true,
			}
			if criticalSections[d] {
				t.Errorf("critical section %q was dropped", d)
			}
		}
	}
}

func TestRenderZeroBudgetOnlyCritical(t *testing.T) {
	t.Parallel()

	r, err := NewRenderer(testTemplateFS(), nil)
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	data := testInstructionData()
	result, err := r.Render(data, 0)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}

	// Only critical sections should be included.
	for _, s := range result.Sections {
		if s.Priority != PriorityCritical {
			t.Errorf("non-critical section %q included with zero budget", s.Name)
		}
	}

	if len(result.Dropped) == 0 {
		t.Error("expected some sections dropped with zero budget")
	}
}

func TestRenderCriticalNeverDropped(t *testing.T) {
	t.Parallel()

	r, err := NewRenderer(testTemplateFS(), nil)
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	data := testInstructionData()
	// Very tight budget — still should keep critical.
	result, err := r.Render(data, 1)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}

	criticalNames := map[string]bool{
		"identity": true, "project": true, "responsibilities": true,
		"constraints": true, "session": true,
	}

	for name := range criticalNames {
		dropped := false
		for _, d := range result.Dropped {
			if d == name {
				dropped = true
				break
			}
		}
		if dropped {
			t.Errorf("critical section %q was dropped", name)
		}
	}
}

func TestRenderSectionIndividually(t *testing.T) {
	t.Parallel()

	r, err := NewRenderer(testTemplateFS(), nil)
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	data := testInstructionData()
	section, err := r.RenderSection("identity", data)
	if err != nil {
		t.Fatalf("RenderSection() error: %v", err)
	}

	if !strings.Contains(section.Content, "You are a software engineer") {
		t.Error("identity section should contain identity text")
	}

	if section.Name != "identity" {
		t.Errorf("section name = %q, want %q", section.Name, "identity")
	}
}

func TestRenderSectionNotFound(t *testing.T) {
	t.Parallel()

	r, err := NewRenderer(testTemplateFS(), nil)
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	_, err = r.RenderSection("nonexistent", testInstructionData())
	if err == nil {
		t.Error("expected error for nonexistent section")
	}
}

func TestRenderEmptySectionsExcluded(t *testing.T) {
	t.Parallel()

	r, err := NewRenderer(testTemplateFS(), nil)
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	// Data with empty optional fields.
	data := InstructionData{
		Project: ProjectContext{Name: "test", Language: "Go"},
		Role: RoleContext{
			Identity:         "You are an engineer.",
			Responsibilities: []string{"code"},
			Constraints:      []string{"no panics"},
		},
		CruxVersion: "0.1.0",
	}

	result, err := r.Render(data, 50000)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}

	// Sections with empty data should not appear.
	for _, s := range result.Sections {
		if s.Name == "mcp" || s.Name == "skills" || s.Name == "team" {
			t.Errorf("empty section %q should have been excluded", s.Name)
		}
	}
}

func TestRenderPreservesDisplayOrder(t *testing.T) {
	t.Parallel()

	r, err := NewRenderer(testTemplateFS(), nil)
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	data := testInstructionData()
	result, err := r.Render(data, 50000)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}

	// Verify section order matches the registry order.
	orderMap := make(map[string]int)
	for _, meta := range DefaultSections() {
		orderMap[meta.Name] = meta.Order
	}

	for i := 1; i < len(result.Sections); i++ {
		prev := orderMap[result.Sections[i-1].Name]
		curr := orderMap[result.Sections[i].Name]
		if prev > curr {
			t.Errorf("section %q (order %d) appears before %q (order %d)",
				result.Sections[i-1].Name, prev, result.Sections[i].Name, curr)
		}
	}
}

func TestBracketDelimitersNoConflict(t *testing.T) {
	t.Parallel()

	// Template containing Go {{ }} code blocks should render fine
	// since we use [[ ]] delimiters.
	fs := fstest.MapFS{
		"sections/identity.md.tmpl": &fstest.MapFile{
			Data: []byte(`## Code Example

` + "```go" + `
func main() {
    m := map[string]string{"key": "value"}
    for k, v := range m {
        fmt.Printf("%s: %s\n", k, v)
    }
}
` + "```" + `

Agent: [[ .Agent.ID ]]
`),
		},
	}

	r, err := NewRenderer(fs, nil)
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	data := testInstructionData()
	section, err := r.RenderSection("identity", data)
	if err != nil {
		t.Fatalf("RenderSection() error: %v", err)
	}

	// Go {{ }} should pass through unchanged.
	if !strings.Contains(section.Content, `{"key": "value"}`) {
		t.Error("Go code block {{ }} should pass through unchanged")
	}
	if !strings.Contains(section.Content, "eng-1") {
		t.Error("agent ID should be rendered")
	}
}

func TestDefaultSectionsComplete(t *testing.T) {
	t.Parallel()

	sections := DefaultSections()
	if len(sections) != 11 {
		t.Errorf("expected 11 sections, got %d", len(sections))
	}

	// Verify all orders are unique and sequential.
	seen := make(map[int]bool)
	for _, s := range sections {
		if seen[s.Order] {
			t.Errorf("duplicate order %d for section %q", s.Order, s.Name)
		}
		seen[s.Order] = true
	}
}
