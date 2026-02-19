package phase_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/roygabriel/crux/internal/phase"
	"github.com/roygabriel/crux/pkg/types"
)

func testdataPath(name string) string {
	return filepath.Join("testdata", name)
}

func TestParseSpec_PHASE1A(t *testing.T) {
	spec, err := phase.ParseSpec(testdataPath("PHASE1A.md"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	if spec.ID != "1A" {
		t.Errorf("ID = %q, want %q", spec.ID, "1A")
	}
	if spec.Name != "Project Skeleton, Types, Config, CLI Framework" {
		t.Errorf("Name = %q, want %q", spec.Name, "Project Skeleton, Types, Config, CLI Framework")
	}
	if spec.Status != types.PhasePlanned {
		t.Errorf("Status = %q, want %q", spec.Status, types.PhasePlanned)
	}
	if len(spec.DependsOn) != 0 {
		t.Errorf("DependsOn = %v, want empty", spec.DependsOn)
	}
	if len(spec.FilesNew) != 19 {
		t.Errorf("len(FilesNew) = %d, want 19", len(spec.FilesNew))
	}
	if len(spec.FilesModified) != 0 {
		t.Errorf("len(FilesModified) = %d, want 0", len(spec.FilesModified))
	}
	if len(spec.FilesRef) != 3 {
		t.Errorf("len(FilesRef) = %d, want 3", len(spec.FilesRef))
	}
	if len(spec.ExitCriteria) != 6 {
		t.Errorf("len(ExitCriteria) = %d, want 6", len(spec.ExitCriteria))
	}
	if len(spec.Tasks) != 4 {
		t.Errorf("len(Tasks) = %d, want 4", len(spec.Tasks))
	}
}

func TestParseSpec_PHASE2A(t *testing.T) {
	spec, err := phase.ParseSpec(testdataPath("PHASE2A.md"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	if spec.ID != "2A" {
		t.Errorf("ID = %q, want %q", spec.ID, "2A")
	}
	if spec.DependsOn == nil || len(spec.DependsOn) != 1 || spec.DependsOn[0] != "1B" {
		t.Errorf("DependsOn = %v, want [1B]", spec.DependsOn)
	}
	// PHASE2A has no ### Modified subsection — jumps from New to Referenced.
	if spec.FilesModified != nil {
		t.Errorf("FilesModified = %v, want nil (missing subsection)", spec.FilesModified)
	}
}

func TestParseSpec_PHASE3B(t *testing.T) {
	spec, err := phase.ParseSpec(testdataPath("PHASE3B.md"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	if spec.ID != "3B" {
		t.Errorf("ID = %q, want %q", spec.ID, "3B")
	}
	// No Tasks or Files sections.
	if len(spec.Tasks) != 0 {
		t.Errorf("len(Tasks) = %d, want 0", len(spec.Tasks))
	}
	if len(spec.FilesNew) != 0 {
		t.Errorf("len(FilesNew) = %d, want 0", len(spec.FilesNew))
	}
	// Exit criteria still parsed.
	if len(spec.ExitCriteria) != 6 {
		t.Errorf("len(ExitCriteria) = %d, want 6", len(spec.ExitCriteria))
	}
}

func TestParseSpec_GateTypes(t *testing.T) {
	spec, err := phase.ParseSpec(testdataPath("PHASE1A.md"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	if len(spec.ExitCriteria) < 6 {
		t.Fatalf("need at least 6 exit criteria, got %d", len(spec.ExitCriteria))
	}

	// First 5 gates should be automated (have backtick commands).
	for i := 0; i < 5; i++ {
		if spec.ExitCriteria[i].Type != phase.GateAutomated {
			t.Errorf("ExitCriteria[%d].Type = %q, want %q", i, spec.ExitCriteria[i].Type, phase.GateAutomated)
		}
		if spec.ExitCriteria[i].Command == "" {
			t.Errorf("ExitCriteria[%d].Command is empty, want non-empty", i)
		}
	}

	// Last gate should be human-approval (no backtick command).
	if spec.ExitCriteria[5].Type != phase.GateHumanApproval {
		t.Errorf("ExitCriteria[5].Type = %q, want %q", spec.ExitCriteria[5].Type, phase.GateHumanApproval)
	}
	if spec.ExitCriteria[5].Command != "" {
		t.Errorf("ExitCriteria[5].Command = %q, want empty", spec.ExitCriteria[5].Command)
	}
}

func TestParseSpec_NotFound(t *testing.T) {
	_, err := phase.ParseSpec("testdata/DOES_NOT_EXIST.md")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("error = %v, want wrapped os.ErrNotExist", err)
	}
}

func TestParseSpec_Rationale(t *testing.T) {
	spec, err := phase.ParseSpec(testdataPath("PHASE1A.md"))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	if spec.Rationale == "" {
		t.Fatal("Rationale is empty, want multi-line prose")
	}
	if !containsSubstring(spec.Rationale, "Bootstrap everything") {
		t.Errorf("Rationale does not contain expected text, got: %q", spec.Rationale)
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && contains(s, sub))
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// writeDepSpec writes a minimal phase spec with the given depends-on lines
// and returns the temp file path.
func writeDepSpec(t *testing.T, h1 string, dependsOnLines string) string {
	t.Helper()
	content := fmt.Sprintf(`%s

## Status

planned

## Depends On

%s

## Exit Criteria

- [ ] manual check
`, h1, dependsOnLines)
	path := filepath.Join(t.TempDir(), "PHASE.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseDependsOn_Formats(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []types.PhaseID
	}{
		{"none", "None", nil},
		{"single with prefix", "Phase 1B", []types.PhaseID{"1B"}},
		{"single bare ID", "1B", []types.PhaseID{"1B"}},
		{"bullet with prefix", "- Phase 1B", []types.PhaseID{"1B"}},
		{"ID with title suffix", "Phase 1A: Terminal Infrastructure", []types.PhaseID{"1A"}},
		{"comma-sep with Phase each", "Phase 1A, Phase 2B, Phase 3A", []types.PhaseID{"1A", "2B", "3A"}},
		{"comma-sep mixed prefix", "Phase 1A, 2B, 3A", []types.PhaseID{"1A", "2B", "3A"}},
		{"bullet + comma-sep", "- Phase 1A, Phase 2B", []types.PhaseID{"1A", "2B"}},
		{"bare comma-sep", "1A, 2B", []types.PhaseID{"1A", "2B"}},
		{"comma-sep with title suffixes", "Phase 1A: Foo, Phase 2B: Bar", []types.PhaseID{"1A", "2B"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeDepSpec(t,"# Phase T1: Test", tt.input)
			spec, err := phase.ParseSpec(path)
			if err != nil {
				t.Fatalf("ParseSpec: %v", err)
			}
			if len(spec.DependsOn) != len(tt.expected) {
				t.Fatalf("DependsOn = %v (len %d), want %v (len %d)",
					spec.DependsOn, len(spec.DependsOn), tt.expected, len(tt.expected))
			}
			for i, id := range spec.DependsOn {
				if id != tt.expected[i] {
					t.Errorf("DependsOn[%d] = %q, want %q", i, id, tt.expected[i])
				}
			}
		})
	}
}

func TestParseDependsOn_MultiLine(t *testing.T) {
	depLines := "- Phase 1A\n- Phase 2B\n- Phase 3A"
	path := writeDepSpec(t,"# Phase T2: Test", depLines)
	spec, err := phase.ParseSpec(path)
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	want := []types.PhaseID{"1A", "2B", "3A"}
	if len(spec.DependsOn) != len(want) {
		t.Fatalf("DependsOn = %v, want %v", spec.DependsOn, want)
	}
	for i, id := range spec.DependsOn {
		if id != want[i] {
			t.Errorf("DependsOn[%d] = %q, want %q", i, id, want[i])
		}
	}
}

func TestParseSpec_MalformedH1(t *testing.T) {
	path := writeDepSpec(t,"# Manual verification", "None")
	spec, err := phase.ParseSpec(path)
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	if spec.ID != "" {
		t.Errorf("ID = %q, want empty for malformed H1", spec.ID)
	}
	if spec.Name != "Manual verification" {
		t.Errorf("Name = %q, want %q", spec.Name, "Manual verification")
	}
}

func TestParseSpec_H1WithNonStandardID(t *testing.T) {
	path := writeDepSpec(t, "# Manual verification: some title", "None")
	spec, err := phase.ParseSpec(path)
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	// The ID is extracted but non-standard — a warning is logged.
	if spec.ID != "Manual verification" {
		t.Errorf("ID = %q, want %q", spec.ID, "Manual verification")
	}
	if spec.Name != "some title" {
		t.Errorf("Name = %q, want %q", spec.Name, "some title")
	}
}

func TestParseSpec_ExitCriteriaAsH1(t *testing.T) {
	// LLM sometimes generates exit criteria as H1 lines instead of checkboxes.
	content := `# Phase 1A: Terminal Infrastructure

## Status

planned

## Depends On

None

## Exit Criteria

# Module initialization and dependencies
# Build succeeds
# Code quality
# Manual verification: run the program, should show blank screen
# Press Ctrl+C to exit cleanly
`
	path := filepath.Join(t.TempDir(), "PHASE.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	spec, err := phase.ParseSpec(path)
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	// Phase header should NOT be overwritten by exit criteria H1 lines.
	if spec.ID != "1A" {
		t.Errorf("ID = %q, want %q", spec.ID, "1A")
	}
	if spec.Name != "Terminal Infrastructure" {
		t.Errorf("Name = %q, want %q", spec.Name, "Terminal Infrastructure")
	}

	// All 5 H1 lines should be parsed as exit criteria gates.
	if len(spec.ExitCriteria) != 5 {
		t.Fatalf("len(ExitCriteria) = %d, want 5", len(spec.ExitCriteria))
	}
	for i, gate := range spec.ExitCriteria {
		if gate.Type != phase.GateHumanApproval {
			t.Errorf("ExitCriteria[%d].Type = %q, want %q", i, gate.Type, phase.GateHumanApproval)
		}
		if gate.Expected == "" {
			t.Errorf("ExitCriteria[%d].Expected is empty", i)
		}
	}
	if spec.ExitCriteria[0].Expected != "Module initialization and dependencies" {
		t.Errorf("ExitCriteria[0].Expected = %q, want %q",
			spec.ExitCriteria[0].Expected, "Module initialization and dependencies")
	}
}

func TestParseSpec_ExitCriteriaH1BulletItems(t *testing.T) {
	// LLM formats sub-items as "# - description".
	content := `# Phase 2A: Gameplay

## Status

planned

## Depends On

Phase 1A

## Exit Criteria

# Manual verification:
# - Player ship at bottom, moves left/right
# - Press Space to shoot bullets
`
	path := filepath.Join(t.TempDir(), "PHASE.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	spec, err := phase.ParseSpec(path)
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	if spec.ID != "2A" {
		t.Errorf("ID = %q, want %q", spec.ID, "2A")
	}
	// "# Manual verification:" has content "Manual verification:" after stripping "# ".
	// "# - Player ship..." becomes "Player ship..." after stripping "# " and "- ".
	if len(spec.ExitCriteria) != 3 {
		t.Fatalf("len(ExitCriteria) = %d, want 3", len(spec.ExitCriteria))
	}
	if spec.ExitCriteria[1].Expected != "Player ship at bottom, moves left/right" {
		t.Errorf("ExitCriteria[1].Expected = %q, want %q",
			spec.ExitCriteria[1].Expected, "Player ship at bottom, moves left/right")
	}
}
