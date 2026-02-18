package instruct

import (
	"strings"
	"testing"
	"time"
)

func embeddedRenderer(t *testing.T) *Renderer {
	t.Helper()
	fsys, err := TemplatesFS()
	if err != nil {
		t.Fatalf("TemplatesFS() error: %v", err)
	}
	r, err := NewRenderer(fsys, nil)
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}
	return r
}

func fullInstructionData(role RoleName, title, identity string) InstructionData {
	return InstructionData{
		Project: ProjectContext{
			Name:        "crux",
			Description: "Multi-agent orchestrator",
			Language:    "Go",
			Frameworks:  []string{"cobra", "bubbletea"},
			RepoRoot:    "/home/user/crux",
			KeyConcerns: []string{"security", "performance", "testability"},
		},
		Phase: PhaseContext{
			CurrentID:    "14A",
			CurrentName:  "Instruction Engine",
			Progress:     "Prompt 3/4",
			Dependencies: []string{"13A"},
			ExitCriteria: []string{"go build ./...", "go test -race ./..."},
			FilesInScope: []string{"internal/instruct/"},
			FilesReadOnly: []string{
				"internal/phase/template.go",
				"internal/config/config.go",
			},
		},
		Agent: AgentContext{
			ID:          "engineer-1",
			CLI:         CLIClaude,
			Model:       "claude-opus-4-20250514",
			Permissions: "elevated",
			Branch:      "feat/phase-14a",
		},
		Role: RoleContext{
			Name:     role,
			Title:    title,
			Identity: identity,
			Responsibilities: []string{
				"Implement code per prompt specifications",
				"Write comprehensive tests",
				"Follow interface contracts exactly",
			},
			Constraints: []string{
				"Do not panic in production code paths",
				"Wrap errors with context",
				"context.Context as first parameter for I/O",
			},
			Communication: []string{
				"Report completion via structured message",
				"Log blockers with file and line number",
			},
		},
		Prefs: PreferenceInstructions{
			Testing:       "Table-driven tests with t.Parallel()",
			ErrorHandling: "Wrap all errors with fmt.Errorf context",
			Organization:  "One type per file, tests adjacent",
		},
		Memory: MemoryContext{
			ProjectBrief:    "Crux is a multi-agent orchestrator for AI-assisted software development.",
			ActiveContext:    "Working on Phase 14A: Instruction Engine Core.",
			TechContext:      "Go 1.25, SQLite via WASM, no CGO.",
			SystemPatterns:   "Dependency injection, interface-based testing.",
			RecentDecisions: []string{"Chose [[ ]] delimiters for templates", "tiktoken for precise counting"},
		},
		MCP: MCPContext{
			Summary:    "2 MCP servers, 5 tools available",
			AgentTools: []string{"read_file", "write_file", "run_command"},
		},
		Skills: SkillsContext{
			Available:   []string{"git-commit", "test-runner", "lint"},
			AgentSkills: []string{"git-commit", "test-runner"},
		},
		Team: TeamContext{
			Agents: []TeamMember{
				{ID: "reviewer-1", Role: RoleCodeReviewer, CLI: CLIClaude, Status: "idle"},
				{ID: "pm-1", Role: RoleProjectManager, CLI: CLICodex, Status: "busy"},
			},
		},
		Custom:      map[string]string{"Sprint Notes": "Focus on test coverage this sprint."},
		GeneratedAt: time.Date(2026, 2, 18, 14, 30, 0, 0, time.UTC),
		CruxVersion: "0.1.0",
	}
}

func TestEmbeddedTemplatesLoad(t *testing.T) {
	t.Parallel()
	embeddedRenderer(t)
}

func TestUniversalTemplateRendersAllRoles(t *testing.T) {
	t.Parallel()

	roles := []struct {
		role     RoleName
		title    string
		identity string
	}{
		{RolePlanner, "Planner", "You are a senior technical planner."},
		{RoleProjectManager, "Project Manager", "You are a project manager coordinating agents."},
		{RoleSoftwareEngineer, "Software Engineer", "You are a software engineer implementing features."},
		{RoleSystemsEngineer, "Systems Engineer", "You are a systems engineer managing infrastructure."},
		{RoleCodeReviewer, "Code Reviewer", "You are a code reviewer ensuring quality."},
	}

	r := embeddedRenderer(t)

	for _, tc := range roles {
		t.Run(string(tc.role), func(t *testing.T) {
			t.Parallel()
			data := fullInstructionData(tc.role, tc.title, tc.identity)
			result, err := r.Render(data, BudgetGemini)
			if err != nil {
				t.Fatalf("Render() error: %v", err)
			}

			if !strings.Contains(result.Content, tc.identity) {
				t.Error("content should contain role identity")
			}
			if !strings.Contains(result.Content, "crux") {
				t.Error("content should contain project name")
			}
			if result.TotalTokens <= 0 {
				t.Error("TotalTokens should be positive")
			}
		})
	}
}

func TestUniversalTemplateGeneratedMarkers(t *testing.T) {
	t.Parallel()

	r := embeddedRenderer(t)
	data := fullInstructionData(RoleSoftwareEngineer, "Software Engineer", "You are an engineer.")

	// Use universal template directly.
	tmpl := r.templates.Lookup("universal")
	if tmpl == nil {
		t.Fatal("universal template not found")
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	content := buf.String()
	if !strings.Contains(content, "<!-- BEGIN GENERATED") {
		t.Error("missing BEGIN GENERATED marker")
	}
	if !strings.Contains(content, "<!-- END GENERATED -->") {
		t.Error("missing END GENERATED marker")
	}
	if !strings.Contains(content, "0.1.0") {
		t.Error("missing version in generated marker")
	}
}

func TestEmptySectionsProduceNoHeaders(t *testing.T) {
	t.Parallel()

	r := embeddedRenderer(t)

	// Minimal data — most sections empty.
	data := InstructionData{
		Project: ProjectContext{Name: "minimal", Language: "Go"},
		Role: RoleContext{
			Name:             RoleSoftwareEngineer,
			Title:            "Engineer",
			Identity:         "You are an engineer.",
			Responsibilities: []string{"code"},
			Constraints:      []string{"no panics"},
		},
		GeneratedAt: time.Now(),
		CruxVersion: "0.1.0",
	}

	result, err := r.Render(data, BudgetGemini)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}

	// These sections should not have headers since data is empty.
	emptyHeaders := []string{
		"## Available MCP Tools",
		"## Available Skills",
		"## Team",
		"## Active Context",
		"## Recent Decisions",
	}
	for _, h := range emptyHeaders {
		if strings.Contains(result.Content, h) {
			t.Errorf("empty section header %q should not appear", h)
		}
	}
}

func TestSectionPartialsRenderIndependently(t *testing.T) {
	t.Parallel()

	r := embeddedRenderer(t)
	data := fullInstructionData(RoleSoftwareEngineer, "Software Engineer", "You are an engineer.")

	sections := DefaultSections()
	for _, meta := range sections {
		t.Run(meta.Name, func(t *testing.T) {
			t.Parallel()
			section, err := r.RenderSection(meta.Name, data)
			if err != nil {
				t.Fatalf("RenderSection(%q) error: %v", meta.Name, err)
			}
			if section.Content == "" {
				t.Errorf("section %q rendered empty with full data", meta.Name)
			}
		})
	}
}

func TestGoCodeBlocksPassThrough(t *testing.T) {
	t.Parallel()

	r := embeddedRenderer(t)
	data := fullInstructionData(RoleSoftwareEngineer, "Engineer", "You are an engineer.")
	// Put Go code in a custom section.
	data.Custom = map[string]string{
		"Code Example": "```go\nfunc main() {\n\tm := map[string]string{\"a\": \"b\"}\n\tfmt.Println(m)\n}\n```",
	}

	tmpl := r.templates.Lookup("universal")
	if tmpl == nil {
		t.Fatal("universal template not found")
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	if !strings.Contains(buf.String(), `map[string]string{"a": "b"}`) {
		t.Error("Go {{ }} in custom section should pass through unchanged")
	}
}

func TestSectionPrioritiesMatchRegistry(t *testing.T) {
	t.Parallel()

	r := embeddedRenderer(t)
	data := fullInstructionData(RoleSoftwareEngineer, "Engineer", "You are an engineer.")

	result, err := r.Render(data, BudgetGemini)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}

	// Build expected priority map from registry.
	expected := make(map[string]SectionPriority)
	for _, meta := range DefaultSections() {
		expected[meta.Name] = meta.Priority
	}

	for _, s := range result.Sections {
		if exp, ok := expected[s.Name]; ok {
			if s.Priority != exp {
				t.Errorf("section %q priority = %d, want %d", s.Name, s.Priority, exp)
			}
		}
	}
}
