package phase_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/roygabriel/crux/internal/phase"
)

func TestParsePromptDoc_PHASE1A(t *testing.T) {
	prompts, err := phase.ParsePromptDoc(testdataPath("PHASE1A-PROMPT.md"))
	if err != nil {
		t.Fatalf("ParsePromptDoc: %v", err)
	}

	if len(prompts) != 4 {
		t.Fatalf("len(prompts) = %d, want 4", len(prompts))
	}

	tests := []struct {
		idx   int
		num   int
		total int
		title string
	}{
		{0, 1, 4, "Go Module, Directory Layout, Makefile"},
		{1, 2, 4, "Core Shared Types"},
		{2, 3, 4, "Configuration"},
		{3, 4, 4, "CLI Framework"},
	}
	for _, tt := range tests {
		p := prompts[tt.idx]
		if p.PromptNumber != tt.num {
			t.Errorf("prompts[%d].PromptNumber = %d, want %d", tt.idx, p.PromptNumber, tt.num)
		}
		if p.TotalPrompts != tt.total {
			t.Errorf("prompts[%d].TotalPrompts = %d, want %d", tt.idx, p.TotalPrompts, tt.total)
		}
		if p.Title != tt.title {
			t.Errorf("prompts[%d].Title = %q, want %q", tt.idx, p.Title, tt.title)
		}
		if p.PhaseID != "1A" {
			t.Errorf("prompts[%d].PhaseID = %q, want %q", tt.idx, p.PhaseID, "1A")
		}
	}
}

func TestParsePromptDoc_InterfaceContract(t *testing.T) {
	prompts, err := phase.ParsePromptDoc(testdataPath("PHASE1A-PROMPT.md"))
	if err != nil {
		t.Fatalf("ParsePromptDoc: %v", err)
	}

	// Prompt 2 (index 1) has a Go code block interface contract.
	if prompts[1].InterfaceContract == "" {
		t.Error("prompts[1].InterfaceContract is empty, want Go code block")
	}
	if !contains(prompts[1].InterfaceContract, "type AgentID string") {
		t.Errorf("InterfaceContract does not contain expected type definition")
	}
}

func TestParsePromptDoc_NoInterfaceContract(t *testing.T) {
	prompts, err := phase.ParsePromptDoc(testdataPath("PHASE1A-PROMPT.md"))
	if err != nil {
		t.Fatalf("ParsePromptDoc: %v", err)
	}

	// Prompt 1 (index 0) has no Interface Contract section.
	if prompts[0].InterfaceContract != "" {
		t.Errorf("prompts[0].InterfaceContract = %q, want empty", prompts[0].InterfaceContract)
	}
}

func TestParsePromptDoc_Verification(t *testing.T) {
	prompts, err := phase.ParsePromptDoc(testdataPath("PHASE1A-PROMPT.md"))
	if err != nil {
		t.Fatalf("ParsePromptDoc: %v", err)
	}

	// Prompt 1 has 2 verification commands: go mod tidy, go vet ./...
	if len(prompts[0].Verification) != 2 {
		t.Fatalf("len(prompts[0].Verification) = %d, want 2", len(prompts[0].Verification))
	}
	if prompts[0].Verification[0].Command != "go mod tidy" {
		t.Errorf("Verification[0].Command = %q, want %q", prompts[0].Verification[0].Command, "go mod tidy")
	}
	if prompts[0].Verification[0].Type != phase.GateAutomated {
		t.Errorf("Verification[0].Type = %q, want %q", prompts[0].Verification[0].Type, phase.GateAutomated)
	}

	// Prompt 2 has 3 verification commands.
	if len(prompts[1].Verification) != 3 {
		t.Fatalf("len(prompts[1].Verification) = %d, want 3", len(prompts[1].Verification))
	}
}

func TestParsePromptDoc_RequiredReading(t *testing.T) {
	prompts, err := phase.ParsePromptDoc(testdataPath("PHASE1A-PROMPT.md"))
	if err != nil {
		t.Fatalf("ParsePromptDoc: %v", err)
	}

	// Prompt 1: "- README.md (repo layout)" → "README.md"
	if len(prompts[0].RequiredReading) != 2 {
		t.Fatalf("len(prompts[0].RequiredReading) = %d, want 2", len(prompts[0].RequiredReading))
	}
	if prompts[0].RequiredReading[0] != "README.md" {
		t.Errorf("RequiredReading[0] = %q, want %q", prompts[0].RequiredReading[0], "README.md")
	}
	if prompts[0].RequiredReading[1] != "LLM.md" {
		t.Errorf("RequiredReading[1] = %q, want %q", prompts[0].RequiredReading[1], "LLM.md")
	}

	// Prompt 2 has annotations stripped.
	if len(prompts[1].RequiredReading) != 3 {
		t.Fatalf("len(prompts[1].RequiredReading) = %d, want 3", len(prompts[1].RequiredReading))
	}
	if prompts[1].RequiredReading[0] != "docs/phases/PHASE1A.md" {
		t.Errorf("RequiredReading[0] = %q, want %q", prompts[1].RequiredReading[0], "docs/phases/PHASE1A.md")
	}
}

func TestParsePromptDoc_NotFound(t *testing.T) {
	_, err := phase.ParsePromptDoc("testdata/DOES_NOT_EXIST.md")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("error = %v, want wrapped os.ErrNotExist", err)
	}
}

func TestParsePromptDoc_PHASE2A(t *testing.T) {
	prompts, err := phase.ParsePromptDoc(testdataPath("PHASE2A-PROMPT.md"))
	if err != nil {
		t.Fatalf("ParsePromptDoc: %v", err)
	}

	if len(prompts) != 4 {
		t.Fatalf("len(prompts) = %d, want 4", len(prompts))
	}

	// Prompt 1 of PHASE2A has an interface contract.
	if prompts[0].InterfaceContract == "" {
		t.Error("prompts[0].InterfaceContract is empty, want Go code block")
	}

	// All prompts should have PhaseID "2A".
	for i, p := range prompts {
		if p.PhaseID != "2A" {
			t.Errorf("prompts[%d].PhaseID = %q, want %q", i, p.PhaseID, "2A")
		}
	}
}

func TestParsePromptDoc_TaskItems(t *testing.T) {
	prompts, err := phase.ParsePromptDoc(testdataPath("PHASE1A-PROMPT.md"))
	if err != nil {
		t.Fatalf("ParsePromptDoc: %v", err)
	}

	// Prompt 1 has 6 numbered items.
	if len(prompts[0].Items) != 6 {
		t.Errorf("len(prompts[0].Items) = %d, want 6", len(prompts[0].Items))
	}

	// Task prose should not be empty.
	if prompts[0].Task == "" {
		t.Error("prompts[0].Task is empty, want task prose")
	}
}

func TestParsePromptDoc_Acceptance(t *testing.T) {
	prompts, err := phase.ParsePromptDoc(testdataPath("PHASE1A-PROMPT.md"))
	if err != nil {
		t.Fatalf("ParsePromptDoc: %v", err)
	}

	// Prompt 1 has 3 acceptance criteria.
	if len(prompts[0].Acceptance) != 3 {
		t.Errorf("len(prompts[0].Acceptance) = %d, want 3", len(prompts[0].Acceptance))
	}
}

func TestParsePromptDoc_NoTotalPromptHeadings(t *testing.T) {
	t.Parallel()

	content := `# PHASE 9B PROMPTS: Example

## Prompt 1: First task
### Task
Do first.
### Verification
` + "```bash\ntrue\n```" + `

---

## Prompt 2: Second task
### Task
Do second.
### Verification
` + "```bash\ntrue\n```" + `
`

	path := filepath.Join(t.TempDir(), "PHASE9B-PROMPT.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write prompt doc: %v", err)
	}

	prompts, err := phase.ParsePromptDoc(path)
	if err != nil {
		t.Fatalf("ParsePromptDoc: %v", err)
	}
	if len(prompts) != 2 {
		t.Fatalf("len(prompts) = %d, want 2", len(prompts))
	}
	if prompts[0].PromptNumber != 1 || prompts[1].PromptNumber != 2 {
		t.Fatalf("prompt numbers = (%d,%d), want (1,2)", prompts[0].PromptNumber, prompts[1].PromptNumber)
	}
	if prompts[0].TotalPrompts != 2 || prompts[1].TotalPrompts != 2 {
		t.Fatalf("total prompts = (%d,%d), want (2,2)", prompts[0].TotalPrompts, prompts[1].TotalPrompts)
	}
	if prompts[0].PhaseID != "9B" || prompts[1].PhaseID != "9B" {
		t.Fatalf("phase IDs = (%q,%q), want (\"9B\",\"9B\")", prompts[0].PhaseID, prompts[1].PhaseID)
	}
}

func TestParsePromptDoc_VerificationSkipsCommentsAndMarksManual(t *testing.T) {
	t.Parallel()

	content := `# PHASE 1A PROMPTS: Example

## Prompt 1: Verify
### Task
Do it.
### Verification
` + "```bash\n" + `
# Build check
go build ./...
# Manual verification: press Ctrl+C after launch
./space-invaders
` + "```" + `
`

	path := filepath.Join(t.TempDir(), "PHASE1A-PROMPT.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write prompt doc: %v", err)
	}

	prompts, err := phase.ParsePromptDoc(path)
	if err != nil {
		t.Fatalf("ParsePromptDoc: %v", err)
	}
	if len(prompts) != 1 {
		t.Fatalf("len(prompts) = %d, want 1", len(prompts))
	}
	if len(prompts[0].Verification) != 2 {
		t.Fatalf("len(verification) = %d, want 2", len(prompts[0].Verification))
	}
	if prompts[0].Verification[0].Type != phase.GateAutomated {
		t.Fatalf("verification[0].Type = %q, want automated", prompts[0].Verification[0].Type)
	}
	if prompts[0].Verification[0].Command != "go build ./..." {
		t.Fatalf("verification[0].Command = %q, want %q", prompts[0].Verification[0].Command, "go build ./...")
	}
	if prompts[0].Verification[1].Type != phase.GateHumanApproval {
		t.Fatalf("verification[1].Type = %q, want human-approval", prompts[0].Verification[1].Type)
	}
	if prompts[0].Verification[1].Command != "" {
		t.Fatalf("verification[1].Command = %q, want empty", prompts[0].Verification[1].Command)
	}
}
