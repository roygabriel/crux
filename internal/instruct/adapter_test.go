package instruct

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// testRenderResult builds a RenderResult using the test template FS and
// test instruction data, rendering at a generous budget so all sections
// are included.
func testRenderResult(t *testing.T) *RenderResult {
	t.Helper()
	r, err := NewRenderer(testTemplateFS(), nil)
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}
	result, err := r.Render(testInstructionData(), 50000)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	return result
}

func TestAdapterForCLI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cli     AgentCLI
		wantCLI AgentCLI
		wantErr bool
	}{
		{"claude", CLIClaude, CLIClaude, false},
		{"codex", CLICodex, CLICodex, false},
		{"gemini", CLIGemini, CLIGemini, false},
		{"copilot", CLICopilot, CLICopilot, false},
		{"unknown", AgentCLI("vim"), "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			adapter, err := AdapterForCLI(tt.cli)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error for unknown CLI")
				}
				if !errors.Is(err, ErrUnknownCLI) {
					t.Errorf("error should wrap ErrUnknownCLI, got: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("AdapterForCLI(%s) error: %v", tt.cli, err)
			}
			if adapter.CLI() != tt.wantCLI {
				t.Errorf("CLI() = %s, want %s", adapter.CLI(), tt.wantCLI)
			}
		})
	}
}

func TestAdapterProperties(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cli          AgentCLI
		budget       int
		reload       ReloadMethod
		reloadCmd    string
	}{
		{CLIClaude, BudgetClaude, ReloadRestart, "exit\n"},
		{CLICodex, BudgetCodex, ReloadNewSession, "/new\n"},
		{CLIGemini, BudgetGemini, ReloadMemoryRefresh, "/memory refresh\n"},
		{CLICopilot, BudgetCopilot, ReloadRestart, "exit\n"},
	}
	for _, tt := range tests {
		t.Run(string(tt.cli), func(t *testing.T) {
			t.Parallel()
			a, err := AdapterForCLI(tt.cli)
			if err != nil {
				t.Fatalf("AdapterForCLI(%s) error: %v", tt.cli, err)
			}
			if a.TokenBudget() != tt.budget {
				t.Errorf("TokenBudget() = %d, want %d", a.TokenBudget(), tt.budget)
			}
			if a.ReloadMethod() != tt.reload {
				t.Errorf("ReloadMethod() = %s, want %s", a.ReloadMethod(), tt.reload)
			}
			if a.ReloadCommand() != tt.reloadCmd {
				t.Errorf("ReloadCommand() = %q, want %q", a.ReloadCommand(), tt.reloadCmd)
			}
		})
	}
}

func TestClaudePrepareFiles(t *testing.T) {
	t.Parallel()

	result := testRenderResult(t)
	a, _ := AdapterForCLI(CLIClaude)

	files, err := a.PrepareFiles(result, "/project", nil)
	if err != nil {
		t.Fatalf("PrepareFiles() error: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	// Primary file.
	primary := files[0]
	if primary.Path != filepath.Join("/project", "CLAUDE.md") {
		t.Errorf("primary path = %s, want CLAUDE.md", primary.Path)
	}
	if primary.Purpose != "primary" {
		t.Errorf("primary purpose = %s, want primary", primary.Purpose)
	}
	if !strings.Contains(primary.Content, markerBeginPrefix) {
		t.Error("primary content should contain BEGIN marker")
	}
	if !strings.Contains(primary.Content, MarkerEnd) {
		t.Error("primary content should contain END marker")
	}

	// Rules file.
	rules := files[1]
	if rules.Path != filepath.Join("/project", ".claude", "rules", "crux-session.md") {
		t.Errorf("rules path = %s", rules.Path)
	}
	if rules.Purpose != "rules" {
		t.Errorf("rules purpose = %s, want rules", rules.Purpose)
	}
	if !strings.HasPrefix(rules.Content, "---\n") {
		t.Error("rules content should start with YAML frontmatter")
	}
	if !strings.Contains(rules.Content, "description:") {
		t.Error("rules content should contain frontmatter description")
	}
}

func TestClaudePreservesUserContent(t *testing.T) {
	t.Parallel()

	result := testRenderResult(t)
	a, _ := AdapterForCLI(CLIClaude)

	primaryPath := filepath.Join("/project", "CLAUDE.md")
	existingContent := "# My Custom Header\n\nSome user notes.\n\n" +
		markerBeginDefault + "\nold generated content\n" + MarkerEnd +
		"\n\n## My Footer\n\nMore user notes.\n"

	files, err := a.PrepareFiles(result, "/project", map[string]string{
		primaryPath: existingContent,
	})
	if err != nil {
		t.Fatalf("PrepareFiles() error: %v", err)
	}

	primary := files[0]
	if !strings.Contains(primary.Content, "My Custom Header") {
		t.Error("user content before markers should be preserved")
	}
	if !strings.Contains(primary.Content, "My Footer") {
		t.Error("user content after markers should be preserved")
	}
	if strings.Contains(primary.Content, "old generated content") {
		t.Error("old generated content should be replaced")
	}
}

func TestCodexPrepareFiles(t *testing.T) {
	t.Parallel()

	result := testRenderResult(t)
	a, _ := AdapterForCLI(CLICodex)

	files, err := a.PrepareFiles(result, "/project", nil)
	if err != nil {
		t.Fatalf("PrepareFiles() error: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	agents := files[0]
	override := files[1]

	if agents.Path != filepath.Join("/project", "AGENTS.md") {
		t.Errorf("agents path = %s", agents.Path)
	}
	if override.Path != filepath.Join("/project", "AGENTS.override.md") {
		t.Errorf("override path = %s", override.Path)
	}

	// Stable sections should be in AGENTS.md.
	for _, name := range []string{"Identity", "Project", "Responsibilities", "Constraints"} {
		if !strings.Contains(agents.Content, name) {
			t.Errorf("AGENTS.md should contain %s section", name)
		}
	}

	// Volatile sections should be in AGENTS.override.md.
	for _, name := range []string{"Phase", "Memory", "Session"} {
		if !strings.Contains(override.Content, name) {
			t.Errorf("AGENTS.override.md should contain %s section", name)
		}
	}

	// No markers in codex files.
	if strings.Contains(agents.Content, markerBeginPrefix) {
		t.Error("AGENTS.md should not contain markers")
	}
}

func TestCodexValidateRejectsOversize(t *testing.T) {
	t.Parallel()

	a, _ := AdapterForCLI(CLICodex)

	oversize := strings.Repeat("x", codexMaxBytes+1)
	if err := a.ValidateOutput(oversize); err == nil {
		t.Error("expected error for oversized content")
	}

	undersize := strings.Repeat("x", codexMaxBytes-1)
	if err := a.ValidateOutput(undersize); err != nil {
		t.Errorf("unexpected error for undersized content: %v", err)
	}
}

func TestGeminiPrepareFiles(t *testing.T) {
	t.Parallel()

	result := testRenderResult(t)
	a, _ := AdapterForCLI(CLIGemini)

	primaryPath := filepath.Join("/project", "GEMINI.md")
	existingContent := "# User Notes\n\n" +
		markerBeginDefault + "\nold content\n" + MarkerEnd +
		"\n\n## User Footer\n"

	files, err := a.PrepareFiles(result, "/project", map[string]string{
		primaryPath: existingContent,
	})
	if err != nil {
		t.Fatalf("PrepareFiles() error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	primary := files[0]
	if primary.Path != primaryPath {
		t.Errorf("path = %s, want %s", primary.Path, primaryPath)
	}
	if !strings.Contains(primary.Content, "User Notes") {
		t.Error("user content before markers should be preserved")
	}
	if !strings.Contains(primary.Content, "User Footer") {
		t.Error("user content after markers should be preserved")
	}
	if strings.Contains(primary.Content, "old content") {
		t.Error("old generated content should be replaced")
	}
}

func TestGeminiValidateWarnsAtRef(t *testing.T) {
	t.Parallel()

	a, _ := AdapterForCLI(CLIGemini)

	err := a.ValidateOutput("Read @./src/main.go for context")
	if err == nil {
		t.Fatal("expected warning for @./ reference")
	}
	var warning *ValidationWarning
	if !errors.As(err, &warning) {
		t.Errorf("expected *ValidationWarning, got %T", err)
	}

	if err := a.ValidateOutput("clean content without references"); err != nil {
		t.Errorf("unexpected error for clean content: %v", err)
	}
}

func TestCopilotPrepareFiles(t *testing.T) {
	t.Parallel()

	result := testRenderResult(t)
	a, _ := AdapterForCLI(CLICopilot)

	files, err := a.PrepareFiles(result, "/project", nil)
	if err != nil {
		t.Fatalf("PrepareFiles() error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	primary := files[0]
	if primary.Path != filepath.Join("/project", "COPILOT.md") {
		t.Errorf("path = %s", primary.Path)
	}
	if !strings.Contains(primary.Content, markerBeginPrefix) {
		t.Error("content should contain BEGIN marker")
	}
	if !strings.Contains(primary.Content, MarkerEnd) {
		t.Error("content should contain END marker")
	}
}

func TestClaudeValidateOutput(t *testing.T) {
	t.Parallel()

	a, _ := AdapterForCLI(CLIClaude)

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{"clean", "normal markdown content", false},
		{"leaked_go_template", "Hello {{ .Name }} world", true},
		{"leaked_crux_template", "Hello [[ .Name ]] world", true},
		{"oversize", strings.Repeat("x", claudeMaxBytes+1), true},
		{"at_limit", strings.Repeat("x", claudeMaxBytes), false},
		{"go_code_blocks_ok", "```go\nmap[string]string{}\n```", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := a.ValidateOutput(tt.content)
			if tt.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestInsertGenerated(t *testing.T) {
	t.Parallel()

	t.Run("first_time_wraps", func(t *testing.T) {
		t.Parallel()
		result := InsertGenerated("", "generated content")
		if !strings.Contains(result, markerBeginDefault) {
			t.Error("should contain BEGIN marker")
		}
		if !strings.Contains(result, MarkerEnd) {
			t.Error("should contain END marker")
		}
		if !strings.Contains(result, "generated content") {
			t.Error("should contain generated content")
		}
	})

	t.Run("re_generation_preserves_user", func(t *testing.T) {
		t.Parallel()
		existing := "# User Header\n\n" +
			markerBeginDefault + "\nold stuff\n" + MarkerEnd +
			"\n\n## User Footer\n"

		result := InsertGenerated(existing, "new stuff")
		if !strings.Contains(result, "User Header") {
			t.Error("should preserve user header")
		}
		if !strings.Contains(result, "User Footer") {
			t.Error("should preserve user footer")
		}
		if strings.Contains(result, "old stuff") {
			t.Error("should replace old generated content")
		}
		if !strings.Contains(result, "new stuff") {
			t.Error("should contain new generated content")
		}
	})
}

func TestInsertGeneratedIdempotent(t *testing.T) {
	t.Parallel()

	existing := "# User Header\n\n" +
		markerBeginDefault + "\noriginal\n" + MarkerEnd +
		"\n\n## User Footer\n"

	// First round.
	round1 := InsertGenerated(existing, "round 1 content")
	if !strings.Contains(round1, "User Header") {
		t.Error("round 1 should preserve user header")
	}

	// Second round on the output of first.
	round2 := InsertGenerated(round1, "round 2 content")
	if !strings.Contains(round2, "User Header") {
		t.Error("round 2 should preserve user header")
	}
	if !strings.Contains(round2, "User Footer") {
		t.Error("round 2 should preserve user footer")
	}
	if !strings.Contains(round2, "round 2 content") {
		t.Error("round 2 should contain latest content")
	}
	if strings.Contains(round2, "round 1 content") {
		t.Error("round 2 should not contain round 1 content")
	}
}

func TestExtractUserContent(t *testing.T) {
	t.Parallel()

	t.Run("valid_pair", func(t *testing.T) {
		t.Parallel()
		content := "User before\n\n" +
			markerBeginDefault + "\ngenerated\n" + MarkerEnd +
			"\n\nUser after"

		before, after := ExtractUserContent(content)
		if before != "User before" {
			t.Errorf("before = %q, want %q", before, "User before")
		}
		if after != "User after" {
			t.Errorf("after = %q, want %q", after, "User after")
		}
	})

	t.Run("no_markers", func(t *testing.T) {
		t.Parallel()
		before, after := ExtractUserContent("just plain content")
		if before != "" || after != "" {
			t.Errorf("expected empty strings, got %q, %q", before, after)
		}
	})
}

func TestInsertGeneratedMalformedMarkers(t *testing.T) {
	t.Parallel()

	t.Run("only_begin_no_end", func(t *testing.T) {
		t.Parallel()
		content := "User text\n" + markerBeginDefault + "\nsome content"
		result := InsertGenerated(content, "new content")
		// Should treat as first-time since no valid pair.
		if !strings.HasPrefix(result, markerBeginDefault) {
			t.Error("malformed markers should result in fresh wrap")
		}
		if !strings.Contains(result, "new content") {
			t.Error("should contain new content")
		}
	})

	t.Run("only_end_no_begin", func(t *testing.T) {
		t.Parallel()
		content := "User text\n" + MarkerEnd
		result := InsertGenerated(content, "new content")
		if !strings.HasPrefix(result, markerBeginDefault) {
			t.Error("malformed markers should result in fresh wrap")
		}
	})
}
