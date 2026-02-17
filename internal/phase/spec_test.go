package phase_test

import (
	"errors"
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
