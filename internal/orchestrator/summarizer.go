package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/roygabriel/crux/internal/memory/worknotes"
)

// WorkNotesReader reads work notes for a phase.
type WorkNotesReader interface {
	Read(phaseID string) (*worknotes.WorkNotes, error)
}

// ContextLayers holds progressively summarized context at different detail levels.
type ContextLayers struct {
	// Recent contains the last few entries at full detail.
	Recent string `json:"recent"`
	// Summarized contains older entries at reduced detail.
	Summarized string `json:"summarized"`
	// RecentCount is the number of entries in the Recent layer.
	RecentCount int `json:"recent_count"`
	// SummarizedCount is the number of entries in the Summarized layer.
	SummarizedCount int `json:"summarized_count"`
	// ArchivedCount is the number of entries too old to include.
	ArchivedCount int `json:"archived_count"`
}

// Summarizer implements progressive context summarization for prompts.
type Summarizer struct {
	workNotes WorkNotesReader
	journal   JournalSearcher
	maxBudget int
	logger    *slog.Logger
}

// NewSummarizer creates a Summarizer with the given dependencies.
func NewSummarizer(workNotes WorkNotesReader, journal JournalSearcher, maxBudget int, logger *slog.Logger) *Summarizer {
	if logger == nil {
		logger = slog.Default()
	}
	if maxBudget <= 0 {
		maxBudget = 3000
	}
	return &Summarizer{
		workNotes: workNotes,
		journal:   journal,
		maxBudget: maxBudget,
		logger:    logger,
	}
}

// SummarizeForPrompt implements phase.ContextSummarizer. It reads work notes
// for the phase, builds progressive layers, and trims to fit the token budget.
func (s *Summarizer) SummarizeForPrompt(_ context.Context, phaseID string, _ int) (string, error) {
	notes, err := s.workNotes.Read(phaseID)
	if err != nil {
		return "", fmt.Errorf("reading work notes for phase %s: %w", phaseID, err)
	}

	entries := notes.SessionLog
	if len(entries) == 0 {
		return "", nil
	}

	layers := s.buildLayers(entries)
	text := renderLayers(layers)
	s.trimToFit(layers, s.maxBudget)

	if EstimateTokens(text) > s.maxBudget {
		text = renderLayers(layers)
	}

	return text, nil
}

// buildLayers partitions session log entries into progressive detail layers.
// - Layer 1 (Recent): last 3 entries — full detail
// - Layer 2 (Summarized): entries 4-10 — decisions+outcomes only
// - Layer 3 (Archived): entries >10 — count only
func (s *Summarizer) buildLayers(entries []worknotes.SessionLogEntry) *ContextLayers {
	layers := &ContextLayers{}

	n := len(entries)
	recentStart := n - 3
	if recentStart < 0 {
		recentStart = 0
	}

	// Recent: last 3 entries, full detail.
	var recentLines []string
	for _, e := range entries[recentStart:] {
		recentLines = append(recentLines, renderFullEntry(e))
	}
	layers.Recent = strings.Join(recentLines, "\n")
	layers.RecentCount = n - recentStart

	// Summarized: entries before recent, up to 7 (indices recentStart-7 .. recentStart-1).
	sumStart := recentStart - 7
	if sumStart < 0 {
		sumStart = 0
	}
	if recentStart > 0 {
		var sumLines []string
		for _, e := range entries[sumStart:recentStart] {
			sumLines = append(sumLines, renderSummarizedEntry(e))
		}
		layers.Summarized = strings.Join(sumLines, "\n")
		layers.SummarizedCount = recentStart - sumStart
	}

	// Archived: everything older than summarized.
	if sumStart > 0 {
		layers.ArchivedCount = sumStart
	}

	return layers
}

// trimToFit trims layers to fit within the token budget.
// It trims Summarized first, then reduces Recent to last entry only.
func (s *Summarizer) trimToFit(layers *ContextLayers, budget int) {
	total := EstimateTokens(renderLayers(layers))
	if total <= budget {
		return
	}

	// First: drop summarized.
	if layers.Summarized != "" {
		layers.ArchivedCount += layers.SummarizedCount
		layers.Summarized = ""
		layers.SummarizedCount = 0
		total = EstimateTokens(renderLayers(layers))
		if total <= budget {
			return
		}
	}

	// Second: keep only the last entry in Recent.
	lines := strings.Split(layers.Recent, "\n### ")
	if len(lines) > 1 {
		lastEntry := "### " + lines[len(lines)-1]
		layers.ArchivedCount += layers.RecentCount - 1
		layers.RecentCount = 1
		layers.Recent = lastEntry
	}
}

// renderLayers combines all layers into a single output string.
func renderLayers(layers *ContextLayers) string {
	var b strings.Builder

	if layers.Recent != "" {
		b.WriteString("Recent activity:\n")
		b.WriteString(layers.Recent)
		b.WriteByte('\n')
	}

	if layers.Summarized != "" {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("Earlier activity (summarized):\n")
		b.WriteString(layers.Summarized)
		b.WriteByte('\n')
	}

	if layers.ArchivedCount > 0 {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(fmt.Sprintf("[%d older entries archived]", layers.ArchivedCount))
		b.WriteByte('\n')
	}

	return b.String()
}

// renderFullEntry renders a session log entry with all fields.
func renderFullEntry(e worknotes.SessionLogEntry) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("### %s\n", e.Timestamp))
	b.WriteString(fmt.Sprintf("- What changed: %s\n", e.Changed))
	b.WriteString(fmt.Sprintf("- Why: %s\n", e.Why))
	if e.Blockers != "" {
		b.WriteString(fmt.Sprintf("- Blockers: %s\n", e.Blockers))
	} else {
		b.WriteString("- Blockers: none\n")
	}
	b.WriteString(fmt.Sprintf("- Next: %s", e.Next))
	return b.String()
}

// renderSummarizedEntry renders a session log entry with decisions+outcomes only.
func renderSummarizedEntry(e worknotes.SessionLogEntry) string {
	return fmt.Sprintf("- %s: %s (%s)", e.Timestamp, e.Changed, e.Why)
}

// EstimateTokens provides a rough token estimate using len/4.
func EstimateTokens(text string) int {
	return len(text) / 4
}
