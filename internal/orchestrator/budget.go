package orchestrator

import (
	"log/slog"
	"strings"

	"github.com/roygabriel/crux/internal/config"
)

// ContextBudget enforces token budgets for context sections injected into prompts.
type ContextBudget struct {
	// TotalBudget is the total token budget across all sections.
	TotalBudget int `json:"total_budget"`
	// WorldStateBudget is the token budget for world state.
	WorldStateBudget int `json:"world_state_budget"`
	// DecisionRAGBudget is the token budget for decision RAG context.
	DecisionRAGBudget int `json:"decision_rag_budget"`
	// SummaryBudget is the token budget for work notes summary.
	SummaryBudget int `json:"summary_budget"`
	// ReserveBudget is the token budget reserved for prompt structure.
	ReserveBudget int `json:"reserve_budget"`

	logger *slog.Logger
}

// DefaultBudget creates a ContextBudget with default values.
func DefaultBudget(logger *slog.Logger) *ContextBudget {
	if logger == nil {
		logger = slog.Default()
	}
	return &ContextBudget{
		TotalBudget:       8000,
		WorldStateBudget:  300,
		DecisionRAGBudget: 1500,
		SummaryBudget:     3000,
		ReserveBudget:     3200,
		logger:            logger,
	}
}

// BudgetFromConfig creates a ContextBudget from config, falling back to
// defaults for zero values.
func BudgetFromConfig(cfg config.ContextConfig, logger *slog.Logger) *ContextBudget {
	if logger == nil {
		logger = slog.Default()
	}
	b := DefaultBudget(logger)
	if cfg.TotalBudget > 0 {
		b.TotalBudget = cfg.TotalBudget
	}
	if cfg.WorldState > 0 {
		b.WorldStateBudget = cfg.WorldState
	}
	if cfg.DecisionRAG > 0 {
		b.DecisionRAGBudget = cfg.DecisionRAG
	}
	if cfg.Summary > 0 {
		b.SummaryBudget = cfg.Summary
	}
	if cfg.Reserve > 0 {
		b.ReserveBudget = cfg.Reserve
	}
	return b
}

// Enforce trims context sections to fit within the configured token budget.
// It implements phase.ContextEnforcer.
func (b *ContextBudget) Enforce(workNotes, decisions, bankSummary string) (string, string, string) {
	// Trim workNotes to SummaryBudget.
	if EstimateTokens(workNotes) > b.SummaryBudget {
		b.logger.Warn("trimming work notes to fit budget",
			"tokens", EstimateTokens(workNotes),
			"budget", b.SummaryBudget,
		)
		workNotes = TrimToTokens(workNotes, b.SummaryBudget)
	}

	// Trim decisions to DecisionRAGBudget.
	if EstimateTokens(decisions) > b.DecisionRAGBudget {
		b.logger.Warn("trimming decisions to fit budget",
			"tokens", EstimateTokens(decisions),
			"budget", b.DecisionRAGBudget,
		)
		decisions = TrimToTokens(decisions, b.DecisionRAGBudget)
	}

	// Calculate remaining budget for bank summary.
	usedTokens := b.WorldStateBudget + b.ReserveBudget + EstimateTokens(workNotes) + EstimateTokens(decisions)
	remaining := b.TotalBudget - usedTokens
	if remaining < 0 {
		remaining = 0
	}

	if EstimateTokens(bankSummary) > remaining {
		b.logger.Warn("trimming bank summary to fit remaining budget",
			"tokens", EstimateTokens(bankSummary),
			"budget", remaining,
		)
		bankSummary = TrimToTokens(bankSummary, remaining)
	}

	return workNotes, decisions, bankSummary
}

// TrimToTokens truncates text to fit within maxTokens at a line boundary,
// appending a truncation marker.
func TrimToTokens(text string, maxTokens int) string {
	if EstimateTokens(text) <= maxTokens {
		return text
	}

	maxBytes := maxTokens * 4
	if maxBytes >= len(text) {
		return text
	}

	// Find the last newline before the byte limit.
	truncated := text[:maxBytes]
	lastNewline := strings.LastIndex(truncated, "\n")
	if lastNewline > 0 {
		truncated = truncated[:lastNewline]
	}

	return truncated + "\n[...truncated]"
}
