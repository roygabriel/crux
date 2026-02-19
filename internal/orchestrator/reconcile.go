package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/roygabriel/crux/internal/memory/session"
	"github.com/roygabriel/crux/internal/phase"
	"github.com/roygabriel/crux/pkg/types"
)

// SessionResumer provides access to the latest persisted session.
type SessionResumer interface {
	ResumeLatest() (*session.SessionContext, error)
}

// ReconcileResult describes claimed vs verified session cursor state.
type ReconcileResult struct {
	ClaimedPhase   types.PhaseID
	ClaimedPrompt  int
	VerifiedPhase  types.PhaseID
	VerifiedPrompt int
	RolledBack     bool
	FailedGates    []string
	MissingFiles   []string
	JournalEntry   string
}

// SessionReconciler verifies persisted session cursor state against disk/gates.
type SessionReconciler struct {
	engine     *phase.Engine
	gateRunner *phase.GateRunner
	sessions   SessionResumer
	journal    DecisionRecorder
	logger     *slog.Logger
}

// NewSessionReconciler builds a reconciler for startup resume checks.
func NewSessionReconciler(
	engine *phase.Engine,
	gateRunner *phase.GateRunner,
	sessions SessionResumer,
	journal DecisionRecorder,
	logger *slog.Logger,
) *SessionReconciler {
	if logger == nil {
		logger = slog.Default()
	}
	return &SessionReconciler{
		engine:     engine,
		gateRunner: gateRunner,
		sessions:   sessions,
		journal:    journal,
		logger:     logger,
	}
}

// Reconcile verifies all prompts that were previously marked complete.
func (r *SessionReconciler) Reconcile(ctx context.Context, root string) (*ReconcileResult, error) {
	result := &ReconcileResult{}
	if r.engine == nil || r.gateRunner == nil || r.sessions == nil {
		return result, nil
	}

	sc, err := r.sessions.ResumeLatest()
	if err != nil {
		if errors.Is(err, session.ErrSessionNotFound) {
			return result, nil
		}
		return nil, fmt.Errorf("session reconcile: resume latest: %w", err)
	}

	result.ClaimedPhase = types.PhaseID(sc.CurrentPhase)
	result.ClaimedPrompt = sc.PromptProgress

	progress := r.engine.Progress()
	order := r.engine.PhaseOrder()
	if len(order) == 0 {
		return result, nil
	}
	orderIndex := make(map[types.PhaseID]int, len(order))
	for i, id := range order {
		orderIndex[id] = i
	}
	claimedIdx, hasClaimed := orderIndex[result.ClaimedPhase]

	completed := make(map[types.PhaseID]int, len(order))
	for _, id := range order {
		completed[id] = 0
	}

	for _, id := range order {
		prog, ok := progress[id]
		if !ok {
			continue
		}
		maxCompleted := 0
		switch {
		case !hasClaimed:
			maxCompleted = 0
		case orderIndex[id] < claimedIdx:
			maxCompleted = len(prog.Prompts)
		case id == result.ClaimedPhase:
			if result.ClaimedPrompt > 1 {
				maxCompleted = result.ClaimedPrompt - 1
			}
		default:
			maxCompleted = 0
		}
		if maxCompleted <= 0 {
			continue
		}
		if maxCompleted > len(prog.Prompts) {
			maxCompleted = len(prog.Prompts)
		}

		for i := 0; i < maxCompleted; i++ {
			prompt := prog.Prompts[i]
			evidence, recErr := ReconcileFiles(ctx, root, *prog.Spec, "")
			if recErr != nil {
				return nil, fmt.Errorf("session reconcile: %s prompt %d reconcile: %w", id, prompt.PromptNumber, recErr)
			}
			if len(prog.Spec.FilesNew) > 0 && !evidence.IsComplete() {
				result.MissingFiles = append(result.MissingFiles, evidence.Missing...)
				result.JournalEntry = fmt.Sprintf(
					"Session reconciliation rolled back from %s:%d due to missing files: %s",
					result.ClaimedPhase, result.ClaimedPrompt, strings.Join(evidence.Missing, ", "),
				)
				r.finalizeResult(ctx, result, order, progress, completed)
				return result, nil
			}

			gateResults, gateErr := r.gateRunner.RunAll(ctx, prompt.Verification)
			if gateErr != nil {
				result.FailedGates = append(result.FailedGates, fmt.Sprintf("phase %s prompt %d: %v", id, prompt.PromptNumber, gateErr))
				result.JournalEntry = fmt.Sprintf(
					"Session reconciliation rolled back from %s:%d due to gate runner error on %s prompt %d",
					result.ClaimedPhase, result.ClaimedPrompt, id, prompt.PromptNumber,
				)
				r.finalizeResult(ctx, result, order, progress, completed)
				return result, nil
			}
			failed := failedGateSummaries(id, prompt.PromptNumber, gateResults)
			if len(failed) > 0 {
				result.FailedGates = append(result.FailedGates, failed...)
				result.JournalEntry = fmt.Sprintf(
					"Session reconciliation rolled back from %s:%d due to gate failures on %s prompt %d",
					result.ClaimedPhase, result.ClaimedPrompt, id, prompt.PromptNumber,
				)
				r.finalizeResult(ctx, result, order, progress, completed)
				return result, nil
			}

			completed[id]++
		}
	}

	r.finalizeResult(ctx, result, order, progress, completed)
	return result, nil
}

func (r *SessionReconciler) finalizeResult(
	ctx context.Context,
	result *ReconcileResult,
	order []types.PhaseID,
	progress map[types.PhaseID]phase.PhaseProgress,
	completed map[types.PhaseID]int,
) {
	phaseID, promptNum := nextCursor(order, progress, completed)
	result.VerifiedPhase = phaseID
	result.VerifiedPrompt = promptNum
	if result.VerifiedPhase != result.ClaimedPhase || result.VerifiedPrompt != result.ClaimedPrompt {
		result.RolledBack = true
		if result.JournalEntry == "" {
			result.JournalEntry = fmt.Sprintf(
				"Session reconciliation rolled back from %s:%d to %s:%d.",
				result.ClaimedPhase, result.ClaimedPrompt, result.VerifiedPhase, result.VerifiedPrompt,
			)
		}
		r.logger.Warn("session reconciliation rollback",
			"claimed_phase", result.ClaimedPhase,
			"claimed_prompt", result.ClaimedPrompt,
			"verified_phase", result.VerifiedPhase,
			"verified_prompt", result.VerifiedPrompt,
			"missing_files", result.MissingFiles,
			"failed_gates", result.FailedGates,
		)
		r.recordRollbackDecision(ctx, result)
	}
}

func (r *SessionReconciler) recordRollbackDecision(ctx context.Context, result *ReconcileResult) {
	if r.journal == nil || result == nil || !result.RolledBack {
		return
	}
	_ = r.journal.Record(ctx, types.Decision{
		Timestamp: time.Now().UTC(),
		AgentID:   "orchestrator",
		PhaseID:   result.VerifiedPhase,
		PromptNum: result.VerifiedPrompt,
		Context:   "session reconciliation",
		Action:    "rollback",
		Rationale: result.JournalEntry,
	})
}

func nextCursor(
	order []types.PhaseID,
	progress map[types.PhaseID]phase.PhaseProgress,
	completed map[types.PhaseID]int,
) (types.PhaseID, int) {
	for _, id := range order {
		prog := progress[id]
		total := len(prog.Prompts)
		done := completed[id]
		if done < total {
			return id, done + 1
		}
	}
	// All prompts verified complete.
	if len(order) == 0 {
		return "", 0
	}
	last := order[len(order)-1]
	return last, 0
}

func failedGateSummaries(phaseID types.PhaseID, promptNum int, results []phase.GateResult) []string {
	var failures []string
	for _, g := range results {
		if g.Passed {
			continue
		}
		cmd := strings.TrimSpace(g.Gate.Command)
		if cmd == "" {
			cmd = g.Gate.Expected
		}
		failures = append(failures, fmt.Sprintf("phase %s prompt %d gate failed: %s", phaseID, promptNum, cmd))
	}
	return failures
}
