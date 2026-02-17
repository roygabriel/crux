package phase_test

import (
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/roygabriel/crux/internal/phase"
	"github.com/roygabriel/crux/pkg/types"
)

func TestProgressSummary_Basic(t *testing.T) {
	engine := &mockPhaseEngine{
		progress: map[types.PhaseID]phase.PhaseProgress{
			"1A": {
				Spec: &phase.PhaseSpec{ID: "1A", Name: "Project Skeleton"},
				Prompts: []phase.PromptContract{
					{PromptNumber: 1, TotalPrompts: 4},
					{PromptNumber: 2, TotalPrompts: 4},
					{PromptNumber: 3, TotalPrompts: 4},
					{PromptNumber: 4, TotalPrompts: 4},
				},
				CompletedPrompts: 2,
			},
		},
		order: []types.PhaseID{"1A"},
	}

	tracker := phase.NewTracker(engine, slog.Default())
	summary, err := tracker.ProgressSummary("1A")
	if err != nil {
		t.Fatalf("ProgressSummary: %v", err)
	}

	if !strings.Contains(summary, "2/4") {
		t.Errorf("summary missing prompt count, got: %s", summary)
	}
	if !strings.Contains(summary, "Project Skeleton") {
		t.Errorf("summary missing phase name, got: %s", summary)
	}
	if !strings.Contains(summary, "1A") {
		t.Errorf("summary missing phase ID, got: %s", summary)
	}
}

func TestProgressSummary_UnknownPhase(t *testing.T) {
	engine := &mockPhaseEngine{
		progress: map[types.PhaseID]phase.PhaseProgress{},
		order:    []types.PhaseID{},
	}

	tracker := phase.NewTracker(engine, slog.Default())
	_, err := tracker.ProgressSummary("NOPE")
	if err == nil {
		t.Fatal("expected error for unknown phase")
	}
	if !errors.Is(err, types.ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestOverallProgress_MultiplePhases(t *testing.T) {
	engine := &mockPhaseEngine{
		progress: map[types.PhaseID]phase.PhaseProgress{
			"1A": {
				Spec:             &phase.PhaseSpec{ID: "1A", Name: "Skeleton"},
				Prompts:          make([]phase.PromptContract, 3),
				CompletedPrompts: 3,
			},
			"2A": {
				Spec:             &phase.PhaseSpec{ID: "2A", Name: "Database"},
				Prompts:          make([]phase.PromptContract, 4),
				CompletedPrompts: 2,
			},
			"3A": {
				Spec:             &phase.PhaseSpec{ID: "3A", Name: "API"},
				Prompts:          make([]phase.PromptContract, 2),
				CompletedPrompts: 0,
			},
		},
		order: []types.PhaseID{"1A", "2A", "3A"},
	}

	tracker := phase.NewTracker(engine, slog.Default())
	output := tracker.OverallProgress()

	lines := strings.Split(output, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), output)
	}

	// 1A is complete (3/3).
	if !strings.HasPrefix(lines[0], "[x]") {
		t.Errorf("line 0 should start with [x], got: %s", lines[0])
	}
	// 2A is in progress (2/4).
	if !strings.HasPrefix(lines[1], "[>]") {
		t.Errorf("line 1 should start with [>], got: %s", lines[1])
	}
	// 3A is not started (0/2).
	if !strings.HasPrefix(lines[2], "[ ]") {
		t.Errorf("line 2 should start with [ ], got: %s", lines[2])
	}
}
