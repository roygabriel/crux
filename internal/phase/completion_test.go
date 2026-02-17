package phase_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/roygabriel/crux/internal/memory/worknotes"
	"github.com/roygabriel/crux/internal/phase"
	"github.com/roygabriel/crux/internal/plugin"
	"github.com/roygabriel/crux/pkg/types"
)

// mockPhaseEngine implements phase.PhaseEngine for testing.
type mockPhaseEngine struct {
	progress  map[types.PhaseID]phase.PhaseProgress
	order     []types.PhaseID
	advanceOK bool
	advanced  []types.PhaseID
}

func (m *mockPhaseEngine) Progress() map[types.PhaseID]phase.PhaseProgress {
	result := make(map[types.PhaseID]phase.PhaseProgress, len(m.progress))
	for k, v := range m.progress {
		result[k] = v
	}
	return result
}

func (m *mockPhaseEngine) ForceAdvance(_ context.Context, phaseID types.PhaseID) error {
	if !m.advanceOK {
		return errors.New("advance not allowed")
	}
	m.advanced = append(m.advanced, phaseID)
	// Simulate advancement by incrementing completed prompts.
	if prog, ok := m.progress[phaseID]; ok {
		prog.CompletedPrompts++
		m.progress[phaseID] = prog
	}
	return nil
}

func (m *mockPhaseEngine) PhaseOrder() []types.PhaseID { return m.order }

// mockDecisionRecorder implements phase.DecisionRecorder for testing.
type mockDecisionRecorder struct {
	recorded []types.Decision
}

func (m *mockDecisionRecorder) Record(_ context.Context, d types.Decision) error {
	m.recorded = append(m.recorded, d)
	return nil
}

// mockWorkNotes implements phase.WorkNotesManager for testing.
type mockWorkNotes struct {
	statuses        map[string]string
	decisions       []string
	sessions        []worknotes.SessionLogEntry
	promptProgress  map[string]map[int]bool
	initCalled      map[string]bool
	readNotes       map[string]*worknotes.WorkNotes
}

func newMockWorkNotes() *mockWorkNotes {
	return &mockWorkNotes{
		statuses:       make(map[string]string),
		promptProgress: make(map[string]map[int]bool),
		initCalled:     make(map[string]bool),
		readNotes:      make(map[string]*worknotes.WorkNotes),
	}
}

func (m *mockWorkNotes) Read(phaseID string) (*worknotes.WorkNotes, error) {
	if n, ok := m.readNotes[phaseID]; ok {
		return n, nil
	}
	return &worknotes.WorkNotes{PhaseID: phaseID}, nil
}

func (m *mockWorkNotes) Init(phaseID, _ string) error {
	m.initCalled[phaseID] = true
	return nil
}

func (m *mockWorkNotes) AppendDecision(phaseID, decision, rationale string) error {
	m.decisions = append(m.decisions, decision+": "+rationale)
	return nil
}

func (m *mockWorkNotes) AppendSession(phaseID string, entry worknotes.SessionLogEntry) error {
	m.sessions = append(m.sessions, entry)
	return nil
}

func (m *mockWorkNotes) UpdatePromptProgress(phaseID string, promptNum int, complete bool) error {
	if m.promptProgress[phaseID] == nil {
		m.promptProgress[phaseID] = make(map[int]bool)
	}
	m.promptProgress[phaseID][promptNum] = complete
	return nil
}

func (m *mockWorkNotes) UpdateStatus(phaseID string, status string) error {
	m.statuses[phaseID] = status
	return nil
}

func (m *mockWorkNotes) Render(notes *worknotes.WorkNotes) string {
	return "rendered"
}

func newTestCompletionDeps(passingGates bool) (*mockPhaseEngine, *mockDecisionRecorder, *mockWorkNotes, *phase.GateRunner) {
	gateCmd := "true"
	if !passingGates {
		gateCmd = "false"
	}

	engine := &mockPhaseEngine{
		progress: map[types.PhaseID]phase.PhaseProgress{
			"1A": {
				Spec: &phase.PhaseSpec{ID: "1A", Name: "First Phase"},
				Prompts: []phase.PromptContract{
					{
						PhaseID:      "1A",
						PromptNumber: 1,
						TotalPrompts: 2,
						Title:        "Step One",
						Task:         "Do step one.",
						Verification: []phase.Gate{
							{Command: gateCmd, Expected: "exit 0", Type: phase.GateAutomated},
						},
					},
					{
						PhaseID:      "1A",
						PromptNumber: 2,
						TotalPrompts: 2,
						Title:        "Step Two",
						Task:         "Do step two.",
						Verification: []phase.Gate{
							{Command: gateCmd, Expected: "exit 0", Type: phase.GateAutomated},
						},
					},
				},
				CompletedPrompts: 0,
			},
		},
		order:     []types.PhaseID{"1A"},
		advanceOK: true,
	}

	journal := &mockDecisionRecorder{}
	wn := newMockWorkNotes()
	runner := phase.NewGateRunner(".", 5*time.Second, slog.Default())

	return engine, journal, wn, runner
}

func TestHandleCompletion_AllGatesPass(t *testing.T) {
	engine, journal, wn, runner := newTestCompletionDeps(true)

	handler := phase.NewCompletionHandler(engine, runner, journal, wn, slog.Default())

	output := plugin.AgentOutput{
		IsComplete: true,
		Decisions: []plugin.OutputDecision{
			{Decision: "Used SQLite", Rationale: "No CGO requirement met with WASM driver"},
		},
	}

	result, err := handler.HandleCompletion(context.Background(), "1A", 1, output)
	if err != nil {
		t.Fatalf("HandleCompletion: %v", err)
	}

	if !result.Passed {
		t.Error("expected Passed=true")
	}
	if len(result.GateResults) != 1 {
		t.Errorf("len(GateResults) = %d, want 1", len(result.GateResults))
	}
	if len(result.Decisions) != 1 {
		t.Errorf("len(Decisions) = %d, want 1", len(result.Decisions))
	}

	// Check decisions were recorded.
	if len(journal.recorded) != 1 {
		t.Errorf("journal.recorded = %d, want 1", len(journal.recorded))
	}

	// Check engine was advanced.
	if len(engine.advanced) != 1 || engine.advanced[0] != "1A" {
		t.Errorf("engine.advanced = %v, want [1A]", engine.advanced)
	}

	// Check work notes were updated.
	if !wn.promptProgress["1A"][1] {
		t.Error("expected prompt 1 marked complete in work notes")
	}
}

func TestHandleCompletion_GateFails(t *testing.T) {
	engine, journal, wn, runner := newTestCompletionDeps(false)

	handler := phase.NewCompletionHandler(engine, runner, journal, wn, slog.Default())

	result, err := handler.HandleCompletion(context.Background(), "1A", 1, plugin.AgentOutput{})
	if err != nil {
		t.Fatalf("HandleCompletion: %v", err)
	}

	if result.Passed {
		t.Error("expected Passed=false")
	}

	// Engine should NOT have been advanced.
	if len(engine.advanced) != 0 {
		t.Errorf("engine should not advance on failure, advanced = %v", engine.advanced)
	}

	// Work notes should be set to Blocked.
	if wn.statuses["1A"] != "Blocked" {
		t.Errorf("status = %q, want %q", wn.statuses["1A"], "Blocked")
	}

	// No decisions should be recorded.
	if len(journal.recorded) != 0 {
		t.Errorf("journal.recorded = %d, want 0", len(journal.recorded))
	}
}

func TestHandleCompletion_PhaseComplete(t *testing.T) {
	engine, journal, wn, runner := newTestCompletionDeps(true)

	// Set up engine with only 1 prompt (make it the last).
	engine.progress["1A"] = phase.PhaseProgress{
		Spec: &phase.PhaseSpec{ID: "1A", Name: "First Phase"},
		Prompts: []phase.PromptContract{
			{
				PhaseID:      "1A",
				PromptNumber: 1,
				TotalPrompts: 1,
				Title:        "Only Step",
				Task:         "Do it.",
				Verification: []phase.Gate{
					{Command: "true", Expected: "exit 0", Type: phase.GateAutomated},
				},
			},
		},
		CompletedPrompts: 0,
	}

	handler := phase.NewCompletionHandler(engine, runner, journal, wn, slog.Default())

	result, err := handler.HandleCompletion(context.Background(), "1A", 1, plugin.AgentOutput{})
	if err != nil {
		t.Fatalf("HandleCompletion: %v", err)
	}

	if !result.Passed {
		t.Error("expected Passed=true")
	}
	if result.NextPrompt != nil {
		t.Error("expected NextPrompt=nil for last prompt")
	}

	// Work notes should be set to Complete.
	if wn.statuses["1A"] != "Complete" {
		t.Errorf("status = %q, want %q", wn.statuses["1A"], "Complete")
	}
}

func TestHandleCompletion_NextPrompt(t *testing.T) {
	engine, _, wn, runner := newTestCompletionDeps(true)

	handler := phase.NewCompletionHandler(engine, runner, &mockDecisionRecorder{}, wn, slog.Default())

	result, err := handler.HandleCompletion(context.Background(), "1A", 1, plugin.AgentOutput{})
	if err != nil {
		t.Fatalf("HandleCompletion: %v", err)
	}

	if !result.Passed {
		t.Error("expected Passed=true")
	}
	if result.NextPrompt == nil {
		t.Fatal("expected NextPrompt to be non-nil")
	}
	if result.NextPrompt.PromptNumber != 2 {
		t.Errorf("NextPrompt.PromptNumber = %d, want 2", result.NextPrompt.PromptNumber)
	}
}

func TestHandleCompletion_NoDecisions(t *testing.T) {
	engine, journal, wn, runner := newTestCompletionDeps(true)

	handler := phase.NewCompletionHandler(engine, runner, journal, wn, slog.Default())

	// Empty output — no decisions.
	result, err := handler.HandleCompletion(context.Background(), "1A", 1, plugin.AgentOutput{})
	if err != nil {
		t.Fatalf("HandleCompletion: %v", err)
	}

	if !result.Passed {
		t.Error("expected Passed=true")
	}
	if len(result.Decisions) != 0 {
		t.Errorf("len(Decisions) = %d, want 0", len(result.Decisions))
	}
	if len(journal.recorded) != 0 {
		t.Errorf("journal.recorded = %d, want 0", len(journal.recorded))
	}

	// Engine should still advance.
	if len(engine.advanced) != 1 {
		t.Errorf("engine should advance even without decisions, advanced = %v", engine.advanced)
	}
}

func TestHandleCompletion_UnknownPhase(t *testing.T) {
	engine, _, wn, runner := newTestCompletionDeps(true)

	handler := phase.NewCompletionHandler(engine, runner, &mockDecisionRecorder{}, wn, slog.Default())

	_, err := handler.HandleCompletion(context.Background(), "NOPE", 1, plugin.AgentOutput{})
	if err == nil {
		t.Fatal("expected error for unknown phase")
	}
	if !errors.Is(err, types.ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}
