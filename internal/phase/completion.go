package phase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/roygabriel/crux/internal/memory/worknotes"
	"github.com/roygabriel/crux/internal/plugin"
	"github.com/roygabriel/crux/pkg/types"
)

// PhaseEngine is the subset of Engine behavior needed for prompt completion and tracking.
type PhaseEngine interface {
	Progress() map[types.PhaseID]PhaseProgress
	ForceAdvance(ctx context.Context, phaseID types.PhaseID) error
	PhaseOrder() []types.PhaseID
}

// DecisionRecorder records decisions to the journal.
type DecisionRecorder interface {
	Record(ctx context.Context, d types.Decision) error
}

// WorkNotesManager manages per-phase work notes.
type WorkNotesManager interface {
	Read(phaseID string) (*worknotes.WorkNotes, error)
	Init(phaseID, phaseName string) error
	AppendDecision(phaseID, decision, rationale string) error
	AppendSession(phaseID string, entry worknotes.SessionLogEntry) error
	UpdatePromptProgress(phaseID string, promptNum int, complete bool) error
	UpdateStatus(phaseID string, status string) error
	Render(notes *worknotes.WorkNotes) string
}

// CompletionResult captures the outcome of handling a prompt completion.
type CompletionResult struct {
	// Passed indicates whether all verification gates succeeded.
	Passed bool `json:"passed"`
	// GateResults lists the outcome of each verification gate.
	GateResults []GateResult `json:"gate_results,omitempty"`
	// NextPrompt is the next prompt contract, or nil if the phase is complete.
	NextPrompt *PromptContract `json:"next_prompt,omitempty"`
	// Decisions lists the decisions recorded during this completion.
	Decisions []types.Decision `json:"decisions,omitempty"`
}

// CompletionHandler processes prompt completions by running verification gates,
// recording decisions, updating work notes, and advancing the engine.
type CompletionHandler struct {
	engine    PhaseEngine
	gates     *GateRunner
	journal   DecisionRecorder
	workNotes WorkNotesManager
	logger    *slog.Logger
}

// NewCompletionHandler creates a CompletionHandler with the given dependencies.
func NewCompletionHandler(engine PhaseEngine, gates *GateRunner, journal DecisionRecorder, workNotes WorkNotesManager, logger *slog.Logger) *CompletionHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &CompletionHandler{
		engine:    engine,
		gates:     gates,
		journal:   journal,
		workNotes: workNotes,
		logger:    logger,
	}
}

// HandleCompletion processes a completed prompt by running verification gates,
// recording decisions, updating work notes, and advancing the engine.
func (h *CompletionHandler) HandleCompletion(
	ctx context.Context, phaseID types.PhaseID, promptNum int, output plugin.AgentOutput,
) (*CompletionResult, error) {
	progress := h.engine.Progress()
	phaseProg, ok := progress[phaseID]
	if !ok {
		return nil, fmt.Errorf("unknown phase %s: %w", phaseID, types.ErrNotFound)
	}

	// Find the prompt contract by number.
	var prompt *PromptContract
	for i := range phaseProg.Prompts {
		if phaseProg.Prompts[i].PromptNumber == promptNum {
			prompt = &phaseProg.Prompts[i]
			break
		}
	}
	if prompt == nil {
		return nil, fmt.Errorf("prompt %d not found in phase %s: %w", promptNum, phaseID, types.ErrNotFound)
	}

	// Run verification gates.
	gateResults, err := h.gates.RunAll(ctx, prompt.Verification)
	if err != nil {
		return nil, fmt.Errorf("running gates for phase %s prompt %d: %w", phaseID, promptNum, err)
	}

	// Check if any gate failed.
	allPassed := true
	for _, r := range gateResults {
		if !r.Passed {
			allPassed = false
			break
		}
	}

	if !allPassed {
		// Gates failed — update work notes to blocked status.
		if err := h.workNotes.UpdateStatus(string(phaseID), "Blocked"); err != nil {
			h.logger.Warn("failed to update work notes status", "phase", phaseID, "error", err)
		}
		if err := h.workNotes.AppendSession(string(phaseID), worknotes.SessionLogEntry{
			Timestamp: time.Now().UTC().Format("2006-01-02 15:04"),
			Changed:   fmt.Sprintf("Prompt %d verification failed", promptNum),
			Why:       "Gate verification did not pass",
			Blockers:  formatBlockers(gateResults),
			Next:      "Fix failing gates and retry",
		}); err != nil {
			h.logger.Warn("failed to append session", "phase", phaseID, "error", err)
		}

		return &CompletionResult{
			Passed:      false,
			GateResults: gateResults,
		}, nil
	}

	// Gates passed — record decisions.
	decisions := convertDecisions(phaseID, promptNum, output.Decisions)
	for _, d := range decisions {
		if err := h.journal.Record(ctx, d); err != nil {
			h.logger.Warn("failed to record decision", "phase", phaseID, "error", err)
		}
		if err := h.workNotes.AppendDecision(string(phaseID), d.Action, d.Rationale); err != nil {
			h.logger.Warn("failed to append decision to work notes", "phase", phaseID, "error", err)
		}
	}

	// Update work notes with session summary and prompt progress.
	if err := h.workNotes.AppendSession(string(phaseID), worknotes.SessionLogEntry{
		Timestamp: time.Now().UTC().Format("2006-01-02 15:04"),
		Changed:   fmt.Sprintf("Prompt %d completed", promptNum),
		Why:       "All verification gates passed",
		Next:      "Proceed to next prompt",
	}); err != nil {
		h.logger.Warn("failed to append session", "phase", phaseID, "error", err)
	}
	if err := h.workNotes.UpdatePromptProgress(string(phaseID), promptNum, true); err != nil {
		h.logger.Warn("failed to update prompt progress", "phase", phaseID, "error", err)
	}

	// Advance the engine (gates already validated).
	if err := h.engine.ForceAdvance(ctx, phaseID); err != nil {
		return nil, fmt.Errorf("advancing phase %s: %w", phaseID, err)
	}

	// Determine next prompt.
	var nextPrompt *PromptContract
	updatedProgress := h.engine.Progress()
	updatedPhase := updatedProgress[phaseID]
	if updatedPhase.CompletedPrompts < len(updatedPhase.Prompts) {
		next := updatedPhase.Prompts[updatedPhase.CompletedPrompts]
		nextPrompt = &next
	}

	// If no next prompt, phase is complete.
	if nextPrompt == nil {
		if err := h.workNotes.UpdateStatus(string(phaseID), "Complete"); err != nil {
			h.logger.Warn("failed to update work notes status", "phase", phaseID, "error", err)
		}
	}

	return &CompletionResult{
		Passed:      true,
		GateResults: gateResults,
		NextPrompt:  nextPrompt,
		Decisions:   decisions,
	}, nil
}

// convertDecisions transforms agent output decisions into typed decisions.
func convertDecisions(phaseID types.PhaseID, promptNum int, outputDecisions []plugin.OutputDecision) []types.Decision {
	decisions := make([]types.Decision, 0, len(outputDecisions))
	now := time.Now().UTC()
	for _, od := range outputDecisions {
		decisions = append(decisions, types.Decision{
			Timestamp: now,
			PhaseID:   phaseID,
			PromptNum: promptNum,
			Context:   fmt.Sprintf("Phase %s, Prompt %d completion", phaseID, promptNum),
			Action:    od.Decision,
			Rationale: od.Rationale,
		})
	}
	return decisions
}

// formatBlockers builds a summary of failed gates for the session log.
func formatBlockers(results []GateResult) string {
	var blockers []string
	for _, r := range results {
		if !r.Passed {
			blockers = append(blockers, fmt.Sprintf("Gate %q failed", r.Gate.Command))
		}
	}
	if len(blockers) == 0 {
		return ""
	}
	return fmt.Sprintf("%d gate(s) failed: %s", len(blockers), joinStrings(blockers, "; "))
}

func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for _, s := range ss[1:] {
		result += sep + s
	}
	return result
}
