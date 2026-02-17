package orchestrator_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/roygabriel/crux/internal/memory/worknotes"
	"github.com/roygabriel/crux/internal/orchestrator"
)

// --- Mock WorkNotesReader ---

type mockWorkNotesReader struct {
	notes map[string]*worknotes.WorkNotes
	err   error
}

func (m *mockWorkNotesReader) Read(phaseID string) (*worknotes.WorkNotes, error) {
	if m.err != nil {
		return nil, m.err
	}
	notes, ok := m.notes[phaseID]
	if !ok {
		return nil, fmt.Errorf("no notes for phase %s", phaseID)
	}
	return notes, nil
}

// --- Helpers ---

func makeEntries(n int) []worknotes.SessionLogEntry {
	entries := make([]worknotes.SessionLogEntry, n)
	for i := 0; i < n; i++ {
		entries[i] = worknotes.SessionLogEntry{
			Timestamp: fmt.Sprintf("2026-02-17 %02d:00", 10+i),
			Changed:   fmt.Sprintf("Change %d", i+1),
			Why:       fmt.Sprintf("Reason %d", i+1),
			Blockers:  "",
			Next:      fmt.Sprintf("Next %d", i+1),
		}
	}
	return entries
}

// --- Tests ---

func TestSummarize_LessThan3(t *testing.T) {
	notes := &worknotes.WorkNotes{
		PhaseID:    "1A",
		SessionLog: makeEntries(2),
	}
	reader := &mockWorkNotesReader{notes: map[string]*worknotes.WorkNotes{"1A": notes}}
	s := orchestrator.NewSummarizer(reader, nil, 3000, nil)

	result, err := s.SummarizeForPrompt(context.Background(), "1A", 1)
	if err != nil {
		t.Fatalf("SummarizeForPrompt() error = %v", err)
	}

	// All entries should be in Recent, nothing in Summarized/Archived.
	if !containsStr(result, "Recent activity") {
		t.Error("expected Recent activity section")
	}
	if containsStr(result, "Earlier activity") {
		t.Error("did not expect Earlier activity section for < 3 entries")
	}
	if containsStr(result, "archived") {
		t.Error("did not expect archived section for < 3 entries")
	}
	// Both entries should be present.
	if !containsStr(result, "Change 1") || !containsStr(result, "Change 2") {
		t.Error("expected both entries in output")
	}
}

func TestSummarize_8Entries(t *testing.T) {
	notes := &worknotes.WorkNotes{
		PhaseID:    "1A",
		SessionLog: makeEntries(8),
	}
	reader := &mockWorkNotesReader{notes: map[string]*worknotes.WorkNotes{"1A": notes}}
	s := orchestrator.NewSummarizer(reader, nil, 10000, nil)

	result, err := s.SummarizeForPrompt(context.Background(), "1A", 1)
	if err != nil {
		t.Fatalf("SummarizeForPrompt() error = %v", err)
	}

	// 3 recent (entries 6,7,8), 5 summarized (entries 1-5), 0 archived.
	if !containsStr(result, "Recent activity") {
		t.Error("expected Recent activity section")
	}
	if !containsStr(result, "Earlier activity") {
		t.Error("expected Earlier activity section for 8 entries")
	}
	if containsStr(result, "archived") {
		t.Error("did not expect archived section for 8 entries")
	}
	// Recent should have full detail (What changed, Why, etc.).
	if !containsStr(result, "What changed: Change 8") {
		t.Error("expected full detail for recent entry 8")
	}
	// Summarized should have condensed format.
	if !containsStr(result, "Change 1") {
		t.Error("expected summarized entry 1")
	}
}

func TestSummarize_15Entries(t *testing.T) {
	notes := &worknotes.WorkNotes{
		PhaseID:    "1A",
		SessionLog: makeEntries(15),
	}
	reader := &mockWorkNotesReader{notes: map[string]*worknotes.WorkNotes{"1A": notes}}
	s := orchestrator.NewSummarizer(reader, nil, 10000, nil)

	result, err := s.SummarizeForPrompt(context.Background(), "1A", 1)
	if err != nil {
		t.Fatalf("SummarizeForPrompt() error = %v", err)
	}

	// 3 recent (13,14,15), 7 summarized (6-12), 5 archived (1-5).
	if !containsStr(result, "Recent activity") {
		t.Error("expected Recent activity section")
	}
	if !containsStr(result, "Earlier activity") {
		t.Error("expected Earlier activity section")
	}
	if !containsStr(result, "5 older entries archived") {
		t.Error("expected 5 archived entries")
	}
}

func TestTrimToFit_RemovesSummarizedFirst(t *testing.T) {
	notes := &worknotes.WorkNotes{
		PhaseID:    "1A",
		SessionLog: makeEntries(10),
	}
	reader := &mockWorkNotesReader{notes: map[string]*worknotes.WorkNotes{"1A": notes}}
	// Very small budget to force trimming.
	s := orchestrator.NewSummarizer(reader, nil, 50, nil)

	result, err := s.SummarizeForPrompt(context.Background(), "1A", 1)
	if err != nil {
		t.Fatalf("SummarizeForPrompt() error = %v", err)
	}

	// Should still have recent entries but summarized should be trimmed.
	if !containsStr(result, "Recent activity") {
		t.Error("expected Recent activity to survive trim")
	}
	// The summarized section should be removed first.
	if containsStr(result, "Earlier activity") {
		t.Error("expected Earlier activity to be trimmed first")
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{"empty", "", 0},
		{"short", "hello world", 2},
		{"medium", "The quick brown fox jumps over the lazy dog.", 11},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := orchestrator.EstimateTokens(tt.text)
			if got != tt.want {
				t.Errorf("EstimateTokens(%q) = %d, want %d", tt.text, got, tt.want)
			}
			// Verify within 2x of actual (len/4 approximation).
			actual := len(tt.text) / 4
			if got != actual {
				t.Errorf("EstimateTokens should equal len/4: got %d, actual %d", got, actual)
			}
		})
	}
}
