package instruct

import (
	"encoding/json"
	"testing"
	"time"
)

func TestFnJoin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		items []string
		sep   string
		want  string
	}{
		{"basic", []string{"a", "b", "c"}, ", ", "a, b, c"},
		{"single", []string{"only"}, ", ", "only"},
		{"empty_slice", []string{}, ", ", ""},
		{"nil_slice", nil, ", ", ""},
		{"empty_sep", []string{"a", "b"}, "", "ab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := fnJoin(tt.items, tt.sep)
			if got != tt.want {
				t.Errorf("fnJoin(%v, %q) = %q, want %q", tt.items, tt.sep, got, tt.want)
			}
		})
	}
}

func TestFnIndent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		n    int
		text string
		want string
	}{
		{"basic", 2, "line1\nline2", "  line1\n  line2"},
		{"zero", 0, "text", "text"},
		{"negative", -1, "text", "text"},
		{"empty_text", 4, "", ""},
		{"empty_lines", 2, "a\n\nb", "  a\n\n  b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := fnIndent(tt.n, tt.text)
			if got != tt.want {
				t.Errorf("fnIndent(%d, %q) = %q, want %q", tt.n, tt.text, got, tt.want)
			}
		})
	}
}

func TestFnBullet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		items []string
		want  string
	}{
		{"basic", []string{"one", "two"}, "- one\n- two\n"},
		{"single", []string{"only"}, "- only\n"},
		{"empty", []string{}, ""},
		{"nil", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := fnBullet(tt.items)
			if got != tt.want {
				t.Errorf("fnBullet(%v) = %q, want %q", tt.items, got, tt.want)
			}
		})
	}
}

func TestFnNumbered(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		items []string
		want  string
	}{
		{"basic", []string{"first", "second"}, "1. first\n2. second\n"},
		{"single", []string{"only"}, "1. only\n"},
		{"empty", []string{}, ""},
		{"nil", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := fnNumbered(tt.items)
			if got != tt.want {
				t.Errorf("fnNumbered(%v) = %q, want %q", tt.items, got, tt.want)
			}
		})
	}
}

func TestFnIfdef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    string
		want bool
	}{
		{"non_empty", "hello", true},
		{"empty", "", false},
		{"whitespace", " ", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := fnIfdef(tt.s)
			if got != tt.want {
				t.Errorf("fnIfdef(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestFnTruncate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		n    int
		text string
		want string
	}{
		{"no_truncate", 10, "short", "short"},
		{"exact_length", 5, "short", "short"},
		{"truncate", 8, "hello world", "hello..."},
		{"very_short", 3, "hello", "hel"},
		{"zero", 0, "hello", ""},
		{"empty", 5, "", ""},
		{"negative", -1, "hello", ""},
		{"unicode", 6, "helloworld", "hel..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := fnTruncate(tt.n, tt.text)
			if got != tt.want {
				t.Errorf("fnTruncate(%d, %q) = %q, want %q", tt.n, tt.text, got, tt.want)
			}
		})
	}
}

func TestFnUpper(t *testing.T) {
	t.Parallel()

	if got := fnUpper("hello"); got != "HELLO" {
		t.Errorf("fnUpper(hello) = %q, want HELLO", got)
	}
	if got := fnUpper(""); got != "" {
		t.Errorf("fnUpper(\"\") = %q, want \"\"", got)
	}
}

func TestFnLower(t *testing.T) {
	t.Parallel()

	if got := fnLower("HELLO"); got != "hello" {
		t.Errorf("fnLower(HELLO) = %q, want hello", got)
	}
	if got := fnLower(""); got != "" {
		t.Errorf("fnLower(\"\") = %q, want \"\"", got)
	}
}

func TestFnTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    string
		want string
	}{
		{"basic", "hello world", "Hello World"},
		{"already_title", "Hello", "Hello"},
		{"empty", "", ""},
		{"single_word", "test", "Test"},
		{"mixed_case", "hELLO wORLD", "HELLO WORLD"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := fnTitle(tt.s)
			if got != tt.want {
				t.Errorf("fnTitle(%q) = %q, want %q", tt.s, got, tt.want)
			}
		})
	}
}

func TestFnContains(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		s      string
		substr string
		want   bool
	}{
		{"found", "hello world", "world", true},
		{"not_found", "hello world", "xyz", false},
		{"empty_substr", "hello", "", true},
		{"empty_both", "", "", true},
		{"empty_s", "", "a", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := fnContains(tt.s, tt.substr)
			if got != tt.want {
				t.Errorf("fnContains(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}

func TestFnHasRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		role   RoleName
		target string
		want   bool
	}{
		{"match", RoleSoftwareEngineer, "software-engineer", true},
		{"no_match", RolePlanner, "software-engineer", false},
		{"empty_target", RolePlanner, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := fnHasRole(tt.role, tt.target)
			if got != tt.want {
				t.Errorf("fnHasRole(%q, %q) = %v, want %v", tt.role, tt.target, got, tt.want)
			}
		})
	}
}

func TestFnHasPermission(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		perm   string
		target string
		want   bool
	}{
		{"match", "elevated", "elevated", true},
		{"no_match", "readonly", "elevated", false},
		{"empty", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := fnHasPermission(tt.perm, tt.target)
			if got != tt.want {
				t.Errorf("fnHasPermission(%q, %q) = %v, want %v", tt.perm, tt.target, got, tt.want)
			}
		})
	}
}

func TestFnTokenCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want int
	}{
		{"basic", "hello world test", 4},
		{"empty", "", 0},
		{"short", "hi", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := fnTokenCount(tt.text)
			if got != tt.want {
				t.Errorf("fnTokenCount(%q) = %d, want %d", tt.text, got, tt.want)
			}
		})
	}
}

func TestDefaultFuncMapRegistered(t *testing.T) {
	t.Parallel()

	fm := DefaultFuncMap()
	expected := []string{
		"join", "indent", "bullet", "numbered", "ifdef",
		"truncate", "upper", "lower", "title", "contains",
		"hasRole", "hasPermission", "tokenCount",
	}
	for _, name := range expected {
		if fm[name] == nil {
			t.Errorf("DefaultFuncMap missing function %q", name)
		}
	}
}

func TestInstructionDataJSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := InstructionData{
		Project: ProjectContext{
			Name:        "test-project",
			Language:    "Go",
			Frameworks:  []string{"cobra", "bubbletea"},
			RepoRoot:    "/home/user/project",
			KeyConcerns: []string{"security", "performance"},
		},
		Phase: PhaseContext{
			CurrentID:   "14A",
			CurrentName: "Instruction Engine",
			Progress:    "Prompt 1/4",
			ExitCriteria: []string{
				"go build ./...",
				"go test -race ./...",
			},
		},
		Agent: AgentContext{
			ID:          "engineer-1",
			CLI:         CLIClaude,
			Model:       "claude-opus-4-20250514",
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
			Testing:       "table-driven tests",
			ErrorHandling: "wrap with context",
		},
		Memory: MemoryContext{
			ProjectBrief:    "A multi-agent orchestrator.",
			RecentDecisions: []string{"chose SQLite over Postgres"},
		},
		MCP: MCPContext{
			Summary:    "2 servers, 5 tools",
			AgentTools: []string{"read_file", "write_file"},
		},
		Skills: SkillsContext{
			Available:   []string{"git", "test"},
			AgentSkills: []string{"git"},
		},
		Team: TeamContext{
			Agents: []TeamMember{
				{ID: "reviewer-1", Role: RoleCodeReviewer, CLI: CLIClaude, Status: "idle"},
			},
		},
		Custom:      map[string]string{"Notes": "extra context"},
		GeneratedAt: time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC),
		CruxVersion: "0.1.0",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded InstructionData
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Re-marshal and compare bytes for equality.
	data2, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}

	if string(data) != string(data2) {
		t.Error("JSON round-trip produced different output")
	}
}

func TestSectionPriorityString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		priority SectionPriority
		want     string
	}{
		{PriorityCritical, "critical"},
		{PriorityHigh, "high"},
		{PriorityMedium, "medium"},
		{PriorityLow, "low"},
		{SectionPriority(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			if got := tt.priority.String(); got != tt.want {
				t.Errorf("SectionPriority(%d).String() = %q, want %q", tt.priority, got, tt.want)
			}
		})
	}
}

func TestAgentCLIString(t *testing.T) {
	t.Parallel()

	if got := CLIClaude.String(); got != "claude" {
		t.Errorf("CLIClaude.String() = %q, want %q", got, "claude")
	}
}

func TestRoleNameString(t *testing.T) {
	t.Parallel()

	if got := RoleSoftwareEngineer.String(); got != "software-engineer" {
		t.Errorf("RoleSoftwareEngineer.String() = %q, want %q", got, "software-engineer")
	}
}
