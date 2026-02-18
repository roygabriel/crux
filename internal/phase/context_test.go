package phase_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/roygabriel/crux/internal/memory/worknotes"
	"github.com/roygabriel/crux/internal/phase"
	"github.com/roygabriel/crux/pkg/types"
)

// mockDecisionSearcher implements phase.DecisionSearcher for testing.
type mockDecisionSearcher struct {
	decisions []types.Decision
	err       error
}

func (m *mockDecisionSearcher) SemanticSearch(_ context.Context, _ string, _ int) ([]types.Decision, error) {
	return m.decisions, m.err
}

// mockBankSummarizer implements phase.BankSummarizer for testing.
type mockBankSummarizer struct {
	summary string
	err     error
}

func (m *mockBankSummarizer) Summary() (string, error) {
	return m.summary, m.err
}

// mockContextWorkNotes wraps mockWorkNotes with pre-loaded notes for context tests.
type mockContextWorkNotes struct {
	notes  map[string]*worknotes.WorkNotes
	errMap map[string]error
}

func (m *mockContextWorkNotes) Read(phaseID string) (*worknotes.WorkNotes, error) {
	if e, ok := m.errMap[phaseID]; ok {
		return nil, e
	}
	if n, ok := m.notes[phaseID]; ok {
		return n, nil
	}
	return nil, errors.New("not found")
}

func (m *mockContextWorkNotes) Init(_, _ string) error { return nil }
func (m *mockContextWorkNotes) AppendDecision(_, _, _ string) error { return nil }
func (m *mockContextWorkNotes) AppendSession(_ string, _ worknotes.SessionLogEntry) error {
	return nil
}
func (m *mockContextWorkNotes) UpdatePromptProgress(_ string, _ int, _ bool) error { return nil }
func (m *mockContextWorkNotes) UpdateStatus(_, _ string) error                     { return nil }
func (m *mockContextWorkNotes) Render(notes *worknotes.WorkNotes) string {
	return "## Work Notes for " + notes.PhaseID
}

func TestBuildForPrompt_FullContext(t *testing.T) {
	wn := &mockContextWorkNotes{
		notes: map[string]*worknotes.WorkNotes{
			"2A": {PhaseID: "2A", PhaseName: "Database", Status: "In progress"},
		},
	}
	js := &mockDecisionSearcher{
		decisions: []types.Decision{
			{
				PhaseID:   "1A",
				PromptNum: 1,
				Context:   "Schema design",
				Action:    "Used SQLite",
				Rationale: "CGO-free WASM driver",
			},
		},
	}
	bank := &mockBankSummarizer{summary: "Project overview content"}

	cb := phase.NewContextBuilder(js, wn, bank, slog.Default())

	contract := phase.PromptContract{
		PhaseID:      "2A",
		PromptNumber: 1,
		TotalPrompts: 2,
		Title:        "Create Schema",
		Task:         "Set up the database schema.",
		Verification: []phase.Gate{
			{Command: "go build ./...", Expected: "exit 0", Type: phase.GateAutomated},
		},
	}
	spec := phase.PhaseSpec{ID: "2A", Name: "Database Layer"}

	data, err := cb.BuildForPrompt(context.Background(), contract, spec, "engineer", "standard")
	if err != nil {
		t.Fatalf("BuildForPrompt: %v", err)
	}

	if data.WorkNotes == "" {
		t.Error("expected non-empty WorkNotes")
	}
	if data.Decisions == "" {
		t.Error("expected non-empty Decisions")
	}
	if data.BankSummary == "" {
		t.Error("expected non-empty BankSummary")
	}
	if data.Role != "engineer" {
		t.Errorf("Role = %q, want %q", data.Role, "engineer")
	}
	if data.PhaseID != "2A" {
		t.Errorf("PhaseID = %q, want %q", data.PhaseID, "2A")
	}
	if data.Task != "Set up the database schema." {
		t.Errorf("Task = %q, want matching contract task", data.Task)
	}
}

func TestBuildForPrompt_EmptyJournal(t *testing.T) {
	wn := &mockContextWorkNotes{
		notes: map[string]*worknotes.WorkNotes{
			"1A": {PhaseID: "1A"},
		},
	}
	js := &mockDecisionSearcher{decisions: nil}
	bank := &mockBankSummarizer{summary: "bank stuff"}

	cb := phase.NewContextBuilder(js, wn, bank, slog.Default())

	contract := phase.PromptContract{
		PhaseID: "1A", PromptNumber: 1, TotalPrompts: 1,
		Title: "First", Task: "Do first.",
	}
	spec := phase.PhaseSpec{ID: "1A", Name: "First Phase"}

	data, err := cb.BuildForPrompt(context.Background(), contract, spec, "engineer", "standard")
	if err != nil {
		t.Fatalf("BuildForPrompt: %v", err)
	}

	if data.Decisions != "" {
		t.Errorf("Decisions should be empty, got %q", data.Decisions)
	}
}

func TestBuildForPrompt_MissingWorkNotes(t *testing.T) {
	wn := &mockContextWorkNotes{
		notes:  map[string]*worknotes.WorkNotes{},
		errMap: map[string]error{"1A": errors.New("file not found")},
	}
	js := &mockDecisionSearcher{decisions: nil}
	bank := &mockBankSummarizer{summary: "bank"}

	cb := phase.NewContextBuilder(js, wn, bank, slog.Default())

	contract := phase.PromptContract{
		PhaseID: "1A", PromptNumber: 1, TotalPrompts: 1,
		Title: "First", Task: "Do.",
	}
	spec := phase.PhaseSpec{ID: "1A", Name: "First"}

	data, err := cb.BuildForPrompt(context.Background(), contract, spec, "engineer", "standard")
	if err != nil {
		t.Fatalf("BuildForPrompt should not error on missing work notes: %v", err)
	}
	if data.WorkNotes != "" {
		t.Errorf("WorkNotes should be empty when read fails, got %q", data.WorkNotes)
	}
}

func TestBuildForPrompt_BankError(t *testing.T) {
	wn := &mockContextWorkNotes{
		notes: map[string]*worknotes.WorkNotes{"1A": {PhaseID: "1A"}},
	}
	js := &mockDecisionSearcher{decisions: nil}
	bank := &mockBankSummarizer{err: errors.New("disk error")}

	cb := phase.NewContextBuilder(js, wn, bank, slog.Default())

	contract := phase.PromptContract{
		PhaseID: "1A", PromptNumber: 1, TotalPrompts: 1,
		Title: "First", Task: "Do.",
	}
	spec := phase.PhaseSpec{ID: "1A", Name: "First"}

	data, err := cb.BuildForPrompt(context.Background(), contract, spec, "engineer", "standard")
	if err != nil {
		t.Fatalf("BuildForPrompt should not error on bank failure: %v", err)
	}
	if data.BankSummary != "" {
		t.Errorf("BankSummary should be empty on error, got %q", data.BankSummary)
	}
}

// --- Mock ContextSummarizer ---

type mockContextSummarizer struct {
	result string
	err    error
}

func (m *mockContextSummarizer) SummarizeForPrompt(_ context.Context, _ string, _ int) (string, error) {
	return m.result, m.err
}

func TestBuildForPrompt_WithSummarizer(t *testing.T) {
	wn := &mockContextWorkNotes{
		notes: map[string]*worknotes.WorkNotes{
			"1A": {PhaseID: "1A"},
		},
	}
	js := &mockDecisionSearcher{decisions: nil}
	bank := &mockBankSummarizer{summary: ""}
	summarizer := &mockContextSummarizer{result: "Summarized context for prompt"}

	cb := phase.NewContextBuilder(js, wn, bank, slog.Default())
	cb.SetSummarizer(summarizer)

	contract := phase.PromptContract{
		PhaseID: "1A", PromptNumber: 1, TotalPrompts: 1,
		Title: "Test", Task: "Do.",
	}
	spec := phase.PhaseSpec{ID: "1A", Name: "Test"}

	data, err := cb.BuildForPrompt(context.Background(), contract, spec, "engineer", "standard")
	if err != nil {
		t.Fatalf("BuildForPrompt: %v", err)
	}
	if data.WorkNotes != "Summarized context for prompt" {
		t.Errorf("WorkNotes = %q, want summarizer output", data.WorkNotes)
	}
}

func TestBuildForPrompt_SummarizerFallback(t *testing.T) {
	wn := &mockContextWorkNotes{
		notes: map[string]*worknotes.WorkNotes{
			"1A": {PhaseID: "1A"},
		},
	}
	js := &mockDecisionSearcher{decisions: nil}
	bank := &mockBankSummarizer{summary: ""}
	summarizer := &mockContextSummarizer{err: errors.New("summarizer failed")}

	cb := phase.NewContextBuilder(js, wn, bank, slog.Default())
	cb.SetSummarizer(summarizer)

	contract := phase.PromptContract{
		PhaseID: "1A", PromptNumber: 1, TotalPrompts: 1,
		Title: "Test", Task: "Do.",
	}
	spec := phase.PhaseSpec{ID: "1A", Name: "Test"}

	data, err := cb.BuildForPrompt(context.Background(), contract, spec, "engineer", "standard")
	if err != nil {
		t.Fatalf("BuildForPrompt should not error on summarizer failure: %v", err)
	}
	// Should fall back to raw notes.
	if data.WorkNotes != "## Work Notes for 1A" {
		t.Errorf("WorkNotes = %q, want raw notes fallback", data.WorkNotes)
	}
}

func TestFormatDecisions(t *testing.T) {
	tests := []struct {
		name      string
		decisions []types.Decision
		wantEmpty bool
		wantCount int
	}{
		{
			name:      "zero decisions",
			decisions: nil,
			wantEmpty: true,
		},
		{
			name: "one decision",
			decisions: []types.Decision{
				{PhaseID: "1A", PromptNum: 1, Context: "ctx", Action: "act", Rationale: "why"},
			},
			wantCount: 1,
		},
		{
			name: "multiple decisions",
			decisions: []types.Decision{
				{PhaseID: "1A", PromptNum: 1, Context: "ctx1", Action: "act1", Rationale: "why1"},
				{PhaseID: "2A", PromptNum: 2, Context: "ctx2", Action: "act2", Rationale: "why2"},
				{PhaseID: "3A", PromptNum: 1, Context: "ctx3", Action: "act3", Rationale: "why3"},
			},
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use the ContextBuilder indirectly — create full context to exercise formatDecisions.
			wn := &mockContextWorkNotes{
				notes: map[string]*worknotes.WorkNotes{"1A": {PhaseID: "1A"}},
			}
			js := &mockDecisionSearcher{decisions: tt.decisions}
			bank := &mockBankSummarizer{summary: ""}

			cb := phase.NewContextBuilder(js, wn, bank, slog.Default())
			contract := phase.PromptContract{
				PhaseID: "1A", PromptNumber: 1, TotalPrompts: 1,
				Title: "Test", Task: "query",
			}
			spec := phase.PhaseSpec{ID: "1A", Name: "Test"}

			data, err := cb.BuildForPrompt(context.Background(), contract, spec, "engineer", "standard")
			if err != nil {
				t.Fatalf("BuildForPrompt: %v", err)
			}

			if tt.wantEmpty {
				if data.Decisions != "" {
					t.Errorf("expected empty decisions, got %q", data.Decisions)
				}
				return
			}

			lines := strings.Split(data.Decisions, "\n")
			if len(lines) != tt.wantCount {
				t.Errorf("decision lines = %d, want %d", len(lines), tt.wantCount)
			}

			// Each line should have the expected format.
			for _, line := range lines {
				if !strings.HasPrefix(line, "- [Phase ") {
					t.Errorf("line format wrong: %s", line)
				}
				if !strings.Contains(line, "(because:") {
					t.Errorf("line missing rationale: %s", line)
				}
			}
		})
	}
}

func TestBuildForPrompt_IncludesRoleDefinition(t *testing.T) {
	wn := &mockContextWorkNotes{
		notes: map[string]*worknotes.WorkNotes{
			"1A": {PhaseID: "1A"},
		},
	}
	js := &mockDecisionSearcher{decisions: nil}
	bank := &mockBankSummarizer{summary: ""}

	cb := phase.NewContextBuilder(js, wn, bank, slog.Default())

	contract := phase.PromptContract{
		PhaseID: "1A", PromptNumber: 1, TotalPrompts: 1,
		Title: "Test", Task: "Do.",
	}
	spec := phase.PhaseSpec{ID: "1A", Name: "Test"}

	data, err := cb.BuildForPrompt(context.Background(), contract, spec, "engineer", "standard")
	if err != nil {
		t.Fatalf("BuildForPrompt: %v", err)
	}
	if data.RoleDefinition == "" {
		t.Error("expected non-empty RoleDefinition for engineer role")
	}
}
