package phase

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/roygabriel/crux/pkg/types"
)

// DecisionSearcher searches the decision journal for relevant context.
type DecisionSearcher interface {
	SemanticSearch(ctx context.Context, query string, n int) ([]types.Decision, error)
}

// BankSummarizer produces a combined memory bank summary.
type BankSummarizer interface {
	Summary() (string, error)
}

// ContextBuilder assembles prompt context from work notes, journal, and memory bank.
type ContextBuilder struct {
	journal   DecisionSearcher
	workNotes WorkNotesManager
	bank      BankSummarizer
	logger    *slog.Logger
}

// NewContextBuilder creates a ContextBuilder with the given dependencies.
func NewContextBuilder(journal DecisionSearcher, workNotes WorkNotesManager, bank BankSummarizer, logger *slog.Logger) *ContextBuilder {
	if logger == nil {
		logger = slog.Default()
	}
	return &ContextBuilder{
		journal:   journal,
		workNotes: workNotes,
		bank:      bank,
		logger:    logger,
	}
}

// BuildForPrompt assembles a PromptData by gathering context from work notes,
// journal, and memory bank. Missing or errored sources degrade gracefully —
// those sections are left empty rather than returning an error.
func (cb *ContextBuilder) BuildForPrompt(
	ctx context.Context, contract PromptContract, spec PhaseSpec, agentRole, agentPerm string,
) (PromptData, error) {
	// Read work notes — first prompt may not have notes yet.
	var workNotesText string
	notes, err := cb.workNotes.Read(string(spec.ID))
	if err != nil {
		cb.logger.Debug("no work notes found", "phase", spec.ID, "error", err)
	} else {
		workNotesText = cb.workNotes.Render(notes)
	}

	// Search journal for relevant decisions.
	var decisionsText string
	if contract.Task != "" {
		decisions, err := cb.journal.SemanticSearch(ctx, contract.Task, 5)
		if err != nil {
			cb.logger.Warn("journal search failed", "phase", spec.ID, "error", err)
		} else {
			decisionsText = formatDecisions(decisions)
		}
	}

	// Get memory bank summary.
	var bankSummary string
	summary, err := cb.bank.Summary()
	if err != nil {
		cb.logger.Warn("bank summary failed", "phase", spec.ID, "error", err)
	} else {
		bankSummary = summary
	}

	return BuildPromptData(contract, spec, workNotesText, decisionsText, bankSummary, agentRole, agentPerm), nil
}

// formatDecisions renders a slice of decisions into a readable text block.
func formatDecisions(decisions []types.Decision) string {
	if len(decisions) == 0 {
		return ""
	}

	var lines []string
	for _, d := range decisions {
		lines = append(lines, fmt.Sprintf("- [Phase %s, Prompt %d] %s -> %s (because: %s)",
			d.PhaseID, d.PromptNum, d.Context, d.Action, d.Rationale))
	}
	return strings.Join(lines, "\n")
}
