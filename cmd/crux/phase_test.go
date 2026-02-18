package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/roygabriel/crux/internal/phase"
)

func TestRenderSpecTemplate(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		phaseName  string
		dependsOn  []string
		numPrompts int
		wantID     string
		wantName   string
		wantDeps   string
		wantPrompt string
	}{
		{
			name:       "basic no deps",
			id:         "9A",
			phaseName:  "Testing Phase",
			dependsOn:  nil,
			numPrompts: 2,
			wantID:     "# Phase 9A: Testing Phase",
			wantName:   "Testing Phase",
			wantDeps:   "None",
			wantPrompt: "### Prompt 2",
		},
		{
			name:       "with dependencies",
			id:         "3B",
			phaseName:  "Integration",
			dependsOn:  []string{"1A", "2A"},
			numPrompts: 3,
			wantID:     "# Phase 3B: Integration",
			wantDeps:   "- Phase 1A",
			wantPrompt: "### Prompt 3",
		},
		{
			name:       "single prompt",
			id:         "1A",
			phaseName:  "Bootstrap",
			dependsOn:  nil,
			numPrompts: 1,
			wantID:     "# Phase 1A: Bootstrap",
			wantDeps:   "None",
			wantPrompt: "### Prompt 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderSpecTemplate(tt.id, tt.phaseName, tt.dependsOn, tt.numPrompts)

			if !strings.Contains(result, tt.wantID) {
				t.Errorf("missing header %q in output", tt.wantID)
			}
			if !strings.Contains(result, tt.wantDeps) {
				t.Errorf("missing deps %q in output", tt.wantDeps)
			}
			if !strings.Contains(result, tt.wantPrompt) {
				t.Errorf("missing prompt %q in output", tt.wantPrompt)
			}
			if !strings.Contains(result, "## Exit Criteria") {
				t.Error("missing Exit Criteria section")
			}

			// Round-trip: the rendered spec should parse without error.
			spec, err := phase.ParseSpec(writeTemp(t, result, "PHASE"+tt.id+".md"))
			if err != nil {
				t.Fatalf("ParseSpec round-trip failed: %v", err)
			}
			if string(spec.ID) != tt.id {
				t.Errorf("parsed ID = %q, want %q", spec.ID, tt.id)
			}
			if spec.Name != tt.phaseName {
				t.Errorf("parsed Name = %q, want %q", spec.Name, tt.phaseName)
			}
		})
	}
}

func TestRenderPromptTemplate(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		numPrompts int
		wantCount  int
	}{
		{"two prompts", "4A", 2, 2},
		{"single prompt", "1A", 1, 1},
		{"four prompts", "5B", 4, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderPromptTemplate(tt.id, tt.numPrompts)

			if !strings.Contains(result, "# Phase "+tt.id+" Implementation Prompts") {
				t.Error("missing phase header")
			}

			count := strings.Count(result, "## Prompt ")
			if count != tt.wantCount {
				t.Errorf("prompt count = %d, want %d", count, tt.wantCount)
			}

			for i := 1; i <= tt.numPrompts; i++ {
				expected := "## Prompt " + strings.Replace("N of M", "N", itoa(i), 1)
				expected = strings.Replace(expected, "M", itoa(tt.numPrompts), 1)
				if !strings.Contains(result, expected) {
					t.Errorf("missing %q in output", expected)
				}
			}
		})
	}
}

func TestPhaseValidate_MissingExitCriteria(t *testing.T) {
	dir := t.TempDir()

	// Write a spec with no exit criteria.
	specContent := `# Phase 99A: Test Phase

## Status
Planned

## Depends On
None

## Tasks

### Prompt 1
- Do something

## Files

### New
- test.go

## Exit Criteria

## Progress Notes
`
	specPath := filepath.Join(dir, "PHASE99A.md")
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatal(err)
	}

	spec, err := phase.ParseSpec(specPath)
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	if len(spec.ExitCriteria) != 0 {
		t.Errorf("expected 0 exit criteria, got %d", len(spec.ExitCriteria))
	}
}

func TestPhaseCreate_GeneratesFiles(t *testing.T) {
	dir := t.TempDir()

	specPath := filepath.Join(dir, "PHASE10A.md")
	promptPath := filepath.Join(dir, "PHASE10A-PROMPT.md")

	specContent := renderSpecTemplate("10A", "Generated Phase", []string{"1A"}, 3)
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatal(err)
	}

	promptContent := renderPromptTemplate("10A", 3)
	if err := os.WriteFile(promptPath, []byte(promptContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Verify spec file exists and parses.
	if _, err := os.Stat(specPath); err != nil {
		t.Fatalf("spec file not created: %v", err)
	}
	spec, err := phase.ParseSpec(specPath)
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	if string(spec.ID) != "10A" {
		t.Errorf("spec ID = %q, want %q", spec.ID, "10A")
	}
	if spec.Name != "Generated Phase" {
		t.Errorf("spec Name = %q, want %q", spec.Name, "Generated Phase")
	}
	if len(spec.DependsOn) != 1 || string(spec.DependsOn[0]) != "1A" {
		t.Errorf("spec DependsOn = %v, want [1A]", spec.DependsOn)
	}

	// Verify prompt file exists.
	if _, err := os.Stat(promptPath); err != nil {
		t.Fatalf("prompt file not created: %v", err)
	}
	data, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(data), "## Prompt "); count != 3 {
		t.Errorf("prompt count = %d, want 3", count)
	}
}

func TestPhaseListOutput_JSON(t *testing.T) {
	output := PhaseListOutput{
		ID:        "1A",
		Name:      "Test Phase",
		Status:    "planned",
		Completed: 0,
		Total:     3,
		DependsOn: []string{"0A"},
	}

	if output.ID != "1A" {
		t.Errorf("ID = %q, want %q", output.ID, "1A")
	}
	if output.Total != 3 {
		t.Errorf("Total = %d, want 3", output.Total)
	}
}

// writeTemp writes content to a temp file and returns the path.
func writeTemp(t *testing.T, content, name string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
