package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/roygabriel/crux/pkg/types"
)

// JournalSearcher provides decision search capabilities for RAG context.
type JournalSearcher interface {
	SemanticSearch(ctx context.Context, query string, n int) ([]types.Decision, error)
	Search(ctx context.Context, query string, n int) ([]types.Decision, error)
}

// DecisionRAG retrieves relevant past decisions to inform new ones.
type DecisionRAG struct {
	journal JournalSearcher
	logger  *slog.Logger
}

// NewDecisionRAG creates a DecisionRAG backed by the given journal.
func NewDecisionRAG(journal JournalSearcher, logger *slog.Logger) *DecisionRAG {
	if logger == nil {
		logger = slog.Default()
	}
	return &DecisionRAG{
		journal: journal,
		logger:  logger,
	}
}

// BeforeDecision retrieves relevant past decisions and blockers for the given
// situation, formatted as an XML-tagged context block. Errors are logged and
// result in partial or empty context rather than failure.
func (r *DecisionRAG) BeforeDecision(ctx context.Context, situation string) (string, error) {
	var decisionLines []string
	var blockerLines []string

	// Retrieve semantically similar decisions.
	decisions, err := r.journal.SemanticSearch(ctx, situation, 5)
	if err != nil {
		r.logger.Warn("semantic search failed for decision RAG", "error", err)
	} else {
		for _, d := range decisions {
			decisionLines = append(decisionLines, formatDecisionLine(d))
		}
	}

	// Retrieve related blockers.
	blockers, err := r.journal.Search(ctx, "blocker "+situation, 3)
	if err != nil {
		r.logger.Warn("blocker search failed for decision RAG", "error", err)
	} else {
		for _, d := range blockers {
			blockerLines = append(blockerLines, formatBlockerLine(d))
		}
	}

	return buildDecisionContext(decisionLines, blockerLines), nil
}

// formatDecisionLine renders a single decision as a bullet line.
func formatDecisionLine(d types.Decision) string {
	agent := ""
	if d.AgentID != "" {
		agent = fmt.Sprintf(" (agent: %s)", d.AgentID)
	}
	return fmt.Sprintf("- Phase %s Prompt %d: %s because %s%s",
		d.PhaseID, d.PromptNum, d.Action, d.Rationale, agent)
}

// formatBlockerLine renders a blocker decision as a bullet line.
func formatBlockerLine(d types.Decision) string {
	return fmt.Sprintf("- Phase %s: %s", d.PhaseID, d.Context)
}

// buildDecisionContext wraps decision and blocker lines in XML tags.
func buildDecisionContext(decisions, blockers []string) string {
	var b strings.Builder
	b.WriteString("<decision_context>\n")

	if len(decisions) > 0 {
		b.WriteString("Relevant past decisions:\n")
		for _, line := range decisions {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}

	if len(blockers) > 0 {
		if len(decisions) > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("Related blockers:\n")
		for _, line := range blockers {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}

	b.WriteString("</decision_context>")
	return b.String()
}
