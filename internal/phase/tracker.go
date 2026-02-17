package phase

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/roygabriel/crux/pkg/types"
)

// Tracker provides progress summaries for phase execution.
type Tracker struct {
	engine PhaseEngine
	logger *slog.Logger
}

// NewTracker creates a Tracker backed by the given engine.
func NewTracker(engine PhaseEngine, logger *slog.Logger) *Tracker {
	if logger == nil {
		logger = slog.Default()
	}
	return &Tracker{engine: engine, logger: logger}
}

// ProgressSummary returns a single-line progress summary for the given phase.
func (t *Tracker) ProgressSummary(phaseID types.PhaseID) (string, error) {
	progress := t.engine.Progress()
	prog, ok := progress[phaseID]
	if !ok {
		return "", fmt.Errorf("unknown phase %s: %w", phaseID, types.ErrNotFound)
	}

	name := ""
	if prog.Spec != nil {
		name = prog.Spec.Name
	}
	total := len(prog.Prompts)

	return fmt.Sprintf("Phase %s: %s — %d/%d prompts complete", phaseID, name, prog.CompletedPrompts, total), nil
}

// OverallProgress renders a multi-line overview of all phases with status indicators.
// [x] = complete, [>] = in progress, [ ] = not started.
func (t *Tracker) OverallProgress() string {
	order := t.engine.PhaseOrder()
	progress := t.engine.Progress()

	var lines []string
	for _, id := range order {
		prog, ok := progress[id]
		if !ok {
			continue
		}
		name := ""
		if prog.Spec != nil {
			name = prog.Spec.Name
		}
		total := len(prog.Prompts)

		var indicator string
		switch {
		case total > 0 && prog.CompletedPrompts >= total:
			indicator = "[x]"
		case prog.CompletedPrompts > 0:
			indicator = "[>]"
		default:
			indicator = "[ ]"
		}

		lines = append(lines, fmt.Sprintf("%s Phase %s: %s (%d/%d)", indicator, id, name, prog.CompletedPrompts, total))
	}

	return strings.Join(lines, "\n")
}
