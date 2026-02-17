package orchestrator_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/roygabriel/crux/internal/config"
	"github.com/roygabriel/crux/internal/orchestrator"
)

func TestDefaultBudget_SumsToTotal(t *testing.T) {
	b := orchestrator.DefaultBudget(nil)

	sum := b.WorldStateBudget + b.DecisionRAGBudget + b.SummaryBudget + b.ReserveBudget
	if sum != b.TotalBudget {
		t.Errorf("budget components sum to %d, want %d (TotalBudget)", sum, b.TotalBudget)
	}
}

func TestEnforce_UnderBudget(t *testing.T) {
	b := orchestrator.DefaultBudget(nil)

	workNotes := "Short notes"
	decisions := "One decision"
	bank := "Brief summary"

	wn, dec, bs := b.Enforce(workNotes, decisions, bank)

	if wn != workNotes {
		t.Errorf("workNotes changed: got %q, want %q", wn, workNotes)
	}
	if dec != decisions {
		t.Errorf("decisions changed: got %q, want %q", dec, decisions)
	}
	if bs != bank {
		t.Errorf("bankSummary changed: got %q, want %q", bs, bank)
	}
}

func TestEnforce_OverBudget(t *testing.T) {
	b := orchestrator.DefaultBudget(slog.Default())

	// Create strings that exceed budgets.
	workNotes := strings.Repeat("Work notes line.\n", 1000)  // ~16000 chars = ~4000 tokens
	decisions := strings.Repeat("Decision line.\n", 1000)    // ~15000 chars = ~3750 tokens
	bank := strings.Repeat("Bank summary line.\n", 500)      // ~9500 chars = ~2375 tokens

	wn, dec, bs := b.Enforce(workNotes, decisions, bank)

	// workNotes should be trimmed to approximately SummaryBudget.
	// Allow small overhead for truncation marker.
	markerOverhead := orchestrator.EstimateTokens("\n[...truncated]")
	if orchestrator.EstimateTokens(wn) > b.SummaryBudget+markerOverhead {
		t.Errorf("workNotes tokens = %d, want <= %d", orchestrator.EstimateTokens(wn), b.SummaryBudget+markerOverhead)
	}

	// decisions should be trimmed to approximately DecisionRAGBudget.
	if orchestrator.EstimateTokens(dec) > b.DecisionRAGBudget+markerOverhead {
		t.Errorf("decisions tokens = %d, want <= %d", orchestrator.EstimateTokens(dec), b.DecisionRAGBudget+markerOverhead)
	}

	// All trimmed content should end with truncation marker.
	if !strings.HasSuffix(wn, "[...truncated]") {
		t.Error("trimmed workNotes should end with truncation marker")
	}
	if !strings.HasSuffix(dec, "[...truncated]") {
		t.Error("trimmed decisions should end with truncation marker")
	}

	// Bank should also be trimmed since remaining budget is small.
	if orchestrator.EstimateTokens(bs) > b.TotalBudget {
		t.Errorf("bankSummary tokens = %d, exceeds total budget", orchestrator.EstimateTokens(bs))
	}
}

func TestEnforce_LogsWarnings(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	logger := slog.New(handler)
	b := orchestrator.BudgetFromConfig(config.ContextConfig{
		TotalBudget: 100,
		WorldState:  10,
		DecisionRAG: 20,
		Summary:     30,
		Reserve:     40,
	}, logger)

	workNotes := strings.Repeat("x", 200)  // 50 tokens, over summary budget of 30
	decisions := strings.Repeat("y", 200)   // 50 tokens, over decision budget of 20

	b.Enforce(workNotes, decisions, "")

	logOutput := buf.String()
	if !strings.Contains(logOutput, "trimming") {
		t.Errorf("expected warning log about trimming, got: %s", logOutput)
	}
}

func TestTrimToTokens_WordBoundary(t *testing.T) {
	text := "Line one\nLine two\nLine three\nLine four\nLine five"
	// Budget of 5 tokens = 20 bytes.
	result := orchestrator.TrimToTokens(text, 5)

	if !strings.HasSuffix(result, "[...truncated]") {
		t.Error("expected truncation marker")
	}
	// Should truncate at a line boundary.
	if strings.Contains(result, "Line four") {
		t.Error("should not contain content beyond the byte limit")
	}
}

func TestEstimateTokens_Sanity(t *testing.T) {
	text := "Hello world, this is a test."
	got := orchestrator.EstimateTokens(text)
	expected := len(text) / 4
	if got != expected {
		t.Errorf("EstimateTokens = %d, want %d (len/4)", got, expected)
	}
}

func TestBudgetFromConfig_Defaults(t *testing.T) {
	// Zero config should use all defaults.
	b := orchestrator.BudgetFromConfig(config.ContextConfig{}, nil)

	if b.TotalBudget != 8000 {
		t.Errorf("TotalBudget = %d, want 8000", b.TotalBudget)
	}
	if b.WorldStateBudget != 300 {
		t.Errorf("WorldStateBudget = %d, want 300", b.WorldStateBudget)
	}
	if b.DecisionRAGBudget != 1500 {
		t.Errorf("DecisionRAGBudget = %d, want 1500", b.DecisionRAGBudget)
	}
	if b.SummaryBudget != 3000 {
		t.Errorf("SummaryBudget = %d, want 3000", b.SummaryBudget)
	}
	if b.ReserveBudget != 3200 {
		t.Errorf("ReserveBudget = %d, want 3200", b.ReserveBudget)
	}
}

func TestBudgetFromConfig_Overrides(t *testing.T) {
	cfg := config.ContextConfig{
		TotalBudget: 10000,
		Summary:     4000,
	}
	b := orchestrator.BudgetFromConfig(cfg, nil)

	if b.TotalBudget != 10000 {
		t.Errorf("TotalBudget = %d, want 10000", b.TotalBudget)
	}
	if b.SummaryBudget != 4000 {
		t.Errorf("SummaryBudget = %d, want 4000", b.SummaryBudget)
	}
	// Non-overridden values should keep defaults.
	if b.WorldStateBudget != 300 {
		t.Errorf("WorldStateBudget = %d, want 300 (default)", b.WorldStateBudget)
	}
	if b.DecisionRAGBudget != 1500 {
		t.Errorf("DecisionRAGBudget = %d, want 1500 (default)", b.DecisionRAGBudget)
	}
}
