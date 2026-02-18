package phase_test

import (
	"strings"
	"testing"

	"github.com/roygabriel/crux/internal/phase"
	"github.com/roygabriel/crux/pkg/types"
)

func TestBuildPromptData_AllFields(t *testing.T) {
	contract := phase.PromptContract{
		PhaseID:      "2A",
		PromptNumber: 1,
		TotalPrompts: 3,
		Title:        "Setup DB Schema",
		RequiredReading: []string{
			"docs/OVERVIEW.md",
			"internal/store/store.go",
		},
		InterfaceContract: "type Store interface {\n\tGet(ctx context.Context, id string) error\n}",
		Task:              "Create the database schema.",
		Items:             []string{"Create tables", "Add indices"},
		Constraints:       []string{"No CGO"},
		Verification: []phase.Gate{
			{Command: "go build ./...", Expected: "exit 0", Type: phase.GateAutomated},
			{Command: "go test ./...", Expected: "exit 0", Type: phase.GateAutomated},
		},
		Acceptance: []string{"Schema matches spec", "Migrations run clean"},
	}

	spec := phase.PhaseSpec{
		ID:   "2A",
		Name: "Database Layer",
	}

	data := phase.BuildPromptData(contract, spec, "some work notes", "some decisions", "bank summary", "engineer", "standard", "")

	if data.Role != "engineer" {
		t.Errorf("Role = %q, want %q", data.Role, "engineer")
	}
	if data.Permission != "standard" {
		t.Errorf("Permission = %q, want %q", data.Permission, "standard")
	}
	if data.PhaseID != "2A" {
		t.Errorf("PhaseID = %q, want %q", data.PhaseID, "2A")
	}
	if data.PhaseName != "Database Layer" {
		t.Errorf("PhaseName = %q, want %q", data.PhaseName, "Database Layer")
	}
	if data.Title != "Setup DB Schema" {
		t.Errorf("Title = %q, want %q", data.Title, "Setup DB Schema")
	}
	if data.PromptNumber != 1 {
		t.Errorf("PromptNumber = %d, want 1", data.PromptNumber)
	}
	if data.TotalPrompts != 3 {
		t.Errorf("TotalPrompts = %d, want 3", data.TotalPrompts)
	}
	if len(data.RequiredReading) != 2 {
		t.Errorf("len(RequiredReading) = %d, want 2", len(data.RequiredReading))
	}
	if data.InterfaceContract == "" {
		t.Error("InterfaceContract should not be empty")
	}
	if data.Task != "Create the database schema." {
		t.Errorf("Task = %q, want %q", data.Task, "Create the database schema.")
	}
	if len(data.Items) != 2 {
		t.Errorf("len(Items) = %d, want 2", len(data.Items))
	}
	if len(data.Acceptance) != 2 {
		t.Errorf("len(Acceptance) = %d, want 2", len(data.Acceptance))
	}
	if len(data.Verification) != 2 {
		t.Errorf("len(Verification) = %d, want 2", len(data.Verification))
	}
	if data.WorkNotes != "some work notes" {
		t.Errorf("WorkNotes = %q, want %q", data.WorkNotes, "some work notes")
	}
	if data.Decisions != "some decisions" {
		t.Errorf("Decisions = %q, want %q", data.Decisions, "some decisions")
	}
	if data.BankSummary != "bank summary" {
		t.Errorf("BankSummary = %q, want %q", data.BankSummary, "bank summary")
	}

	// Constraints should include the contract constraint plus defaults.
	if len(data.Constraints) != 4 {
		t.Fatalf("len(Constraints) = %d, want 4 (1 contract + 3 defaults)", len(data.Constraints))
	}
	if data.Constraints[0] != "No CGO" {
		t.Errorf("Constraints[0] = %q, want %q", data.Constraints[0], "No CGO")
	}
	// Check defaults are present.
	constraintSet := make(map[string]bool)
	for _, c := range data.Constraints {
		constraintSet[c] = true
	}
	for _, dc := range []string{
		"Do not modify files outside the scope of this prompt.",
		"Update work notes after completing the task.",
		"Run all verification commands before considering the task complete.",
	} {
		if !constraintSet[dc] {
			t.Errorf("missing default constraint: %q", dc)
		}
	}
}

func TestBuildPromptData_EmptyOptionals(t *testing.T) {
	contract := phase.PromptContract{
		PhaseID:      "1A",
		PromptNumber: 1,
		TotalPrompts: 1,
		Title:        "Minimal Prompt",
		Task:         "Do something.",
	}

	spec := phase.PhaseSpec{
		ID:   "1A",
		Name: "First Phase",
	}

	data := phase.BuildPromptData(contract, spec, "", "", "", "engineer", "readonly", "")

	if data.InterfaceContract != "" {
		t.Errorf("InterfaceContract should be empty, got %q", data.InterfaceContract)
	}
	if len(data.Acceptance) != 0 {
		t.Errorf("Acceptance should be empty, got %v", data.Acceptance)
	}
	if len(data.RequiredReading) != 0 {
		t.Errorf("RequiredReading should be empty, got %v", data.RequiredReading)
	}
	if data.WorkNotes != "" {
		t.Errorf("WorkNotes should be empty, got %q", data.WorkNotes)
	}
	if data.Decisions != "" {
		t.Errorf("Decisions should be empty, got %q", data.Decisions)
	}

	// Defaults should still be in constraints.
	if len(data.Constraints) != 3 {
		t.Fatalf("len(Constraints) = %d, want 3 (defaults only)", len(data.Constraints))
	}
}

func TestBuildPromptData_DuplicateConstraints(t *testing.T) {
	contract := phase.PromptContract{
		PhaseID:      "1A",
		PromptNumber: 1,
		TotalPrompts: 1,
		Title:        "Dup",
		Task:         "Test dedup.",
		Constraints: []string{
			"Do not modify files outside the scope of this prompt.",
			"Custom rule",
		},
	}

	spec := phase.PhaseSpec{ID: types.PhaseID("1A"), Name: "Test"}

	data := phase.BuildPromptData(contract, spec, "", "", "", "engineer", "standard", "")

	// Should have: "Do not modify..." (deduped), "Custom rule", + 2 remaining defaults = 4
	if len(data.Constraints) != 4 {
		t.Errorf("len(Constraints) = %d, want 4 (deduped)", len(data.Constraints))
	}
}

func TestRenderPrompt_NonEmpty(t *testing.T) {
	data := phase.PromptData{
		Role:         "engineer",
		Permission:   "standard",
		PhaseID:      "2A",
		PhaseName:    "Database Layer",
		Title:        "Setup Schema",
		PromptNumber: 1,
		TotalPrompts: 2,
		Task:         "Create the schema.",
		Verification: []string{"go build ./...", "go test ./..."},
		Constraints:  []string{"No CGO"},
	}

	output, err := phase.RenderPrompt(data)
	if err != nil {
		t.Fatalf("RenderPrompt: %v", err)
	}

	for _, want := range []string{
		"## Role",
		"engineer",
		"### Task",
		"### Verification",
		"### Stop Rule",
		"### Session Management",
		"Phase 2A: Database Layer",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestRenderPrompt_EmptyOptionals(t *testing.T) {
	data := phase.PromptData{
		Role:         "engineer",
		Permission:   "standard",
		PhaseID:      "1A",
		PhaseName:    "First",
		Title:        "Minimal",
		PromptNumber: 1,
		TotalPrompts: 1,
		Task:         "Do it.",
		Constraints:  []string{"Be careful"},
	}

	output, err := phase.RenderPrompt(data)
	if err != nil {
		t.Fatalf("RenderPrompt: %v", err)
	}

	// Empty optionals should not produce section headers.
	if strings.Contains(output, "### Prior Decisions") {
		t.Error("output should not contain '### Prior Decisions' when Decisions is empty")
	}
	if strings.Contains(output, "### Work Notes") {
		t.Error("output should not contain '### Work Notes' when WorkNotes is empty")
	}
	if strings.Contains(output, "### Memory Bank Summary") {
		t.Error("output should not contain '### Memory Bank Summary' when BankSummary is empty")
	}
	if strings.Contains(output, "### Interface Contract") {
		t.Error("output should not contain '### Interface Contract' when InterfaceContract is empty")
	}
	if strings.Contains(output, "### Acceptance Criteria") {
		t.Error("output should not contain '### Acceptance Criteria' when Acceptance is empty")
	}
	if strings.Contains(output, "### Required Reading") {
		t.Error("output should not contain '### Required Reading' when RequiredReading is empty")
	}
	if strings.Contains(output, "### Role Definition") {
		t.Error("output should not contain '### Role Definition' when RoleDefinition is empty")
	}
}

func TestRenderPrompt_DefaultSections(t *testing.T) {
	data := phase.PromptData{
		Role:         "engineer",
		Permission:   "readonly",
		PhaseID:      "1A",
		PhaseName:    "Test",
		Title:        "Test Prompt",
		PromptNumber: 1,
		TotalPrompts: 1,
	}

	output, err := phase.RenderPrompt(data)
	if err != nil {
		t.Fatalf("RenderPrompt: %v", err)
	}

	// Stop Rule and Session Management should always be present.
	if !strings.Contains(output, "### Stop Rule") {
		t.Error("output missing '### Stop Rule' section")
	}
	if !strings.Contains(output, "### Session Management") {
		t.Error("output missing '### Session Management' section")
	}
}

func TestBuildPromptData_WithRoleDefinition(t *testing.T) {
	contract := phase.PromptContract{
		PhaseID:      "1A",
		PromptNumber: 1,
		TotalPrompts: 1,
		Title:        "Test",
		Task:         "Do something.",
	}
	spec := phase.PhaseSpec{ID: "1A", Name: "Test"}

	roleDef := "You are an implementation-focused engineer."
	data := phase.BuildPromptData(contract, spec, "", "", "", "engineer", "standard", roleDef)

	if data.RoleDefinition != roleDef {
		t.Errorf("RoleDefinition = %q, want %q", data.RoleDefinition, roleDef)
	}
}

func TestRenderPrompt_WithRoleDefinition(t *testing.T) {
	data := phase.PromptData{
		Role:           "engineer",
		Permission:     "standard",
		PhaseID:        "1A",
		PhaseName:      "Test",
		Title:          "Test Prompt",
		PromptNumber:   1,
		TotalPrompts:   1,
		RoleDefinition: "You are an implementation-focused engineer.",
	}

	output, err := phase.RenderPrompt(data)
	if err != nil {
		t.Fatalf("RenderPrompt: %v", err)
	}

	if !strings.Contains(output, "### Role Definition") {
		t.Error("output missing '### Role Definition' header")
	}
	if !strings.Contains(output, "implementation-focused engineer") {
		t.Error("output missing role definition content")
	}
}

func TestRenderPrompt_EmptyRoleDefinition(t *testing.T) {
	data := phase.PromptData{
		Role:         "engineer",
		Permission:   "standard",
		PhaseID:      "1A",
		PhaseName:    "Test",
		Title:        "Test Prompt",
		PromptNumber: 1,
		TotalPrompts: 1,
	}

	output, err := phase.RenderPrompt(data)
	if err != nil {
		t.Fatalf("RenderPrompt: %v", err)
	}

	if strings.Contains(output, "### Role Definition") {
		t.Error("output should not contain '### Role Definition' when RoleDefinition is empty")
	}
}
