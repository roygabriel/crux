package instruct

import (
	"strings"
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want int
	}{
		{"empty", "", 0},
		{"short", "hi", 0},
		{"four_bytes", "abcd", 1},
		{"typical", strings.Repeat("word ", 100), 125},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := EstimateTokens(tt.text); got != tt.want {
				t.Errorf("EstimateTokens() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCountTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		text    string
		wantMin int
		wantMax int
	}{
		{"empty", "", 0, 0},
		{"hello", "hello world", 1, 5},
		{"paragraph", "The quick brown fox jumps over the lazy dog.", 5, 15},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := CountTokens(tt.text)
			if err != nil {
				t.Fatalf("CountTokens() error: %v", err)
			}
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("CountTokens() = %d, want between %d and %d", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestCountTokensVsEstimate(t *testing.T) {
	t.Parallel()

	// Verify that estimate and precise count are within reasonable range.
	text := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 50)
	precise, err := CountTokens(text)
	if err != nil {
		t.Fatalf("CountTokens() error: %v", err)
	}
	estimate := EstimateTokens(text)

	// Allow 50% tolerance for estimate vs precise.
	lower := float64(precise) * 0.5
	upper := float64(precise) * 1.5
	if float64(estimate) < lower || float64(estimate) > upper {
		t.Errorf("estimate %d is too far from precise %d", estimate, precise)
	}
}

func TestTruncateToTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		text      string
		maxTokens int
		wantMax   int // max bytes expected
		hasSuffix string
	}{
		{"under_budget", "short text", 100, 10, ""},
		{"over_budget", strings.Repeat("word\n", 200), 50, 220, "[...truncated]"},
		{"zero_budget", "hello", 0, 0, ""},
		{"negative", "hello", -1, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := TruncateToTokens(tt.text, tt.maxTokens)
			if len(got) > tt.wantMax {
				t.Errorf("TruncateToTokens() length %d > max %d", len(got), tt.wantMax)
			}
			if tt.hasSuffix != "" && !strings.HasSuffix(got, tt.hasSuffix) {
				t.Errorf("TruncateToTokens() should end with %q", tt.hasSuffix)
			}
		})
	}
}

func TestBudgetConstants(t *testing.T) {
	t.Parallel()

	if BudgetClaude <= 0 {
		t.Error("BudgetClaude should be positive")
	}
	if BudgetCodex <= BudgetClaude {
		t.Error("BudgetCodex should be larger than BudgetClaude")
	}
	if BudgetGemini <= BudgetCodex {
		t.Error("BudgetGemini should be larger than BudgetCodex")
	}
	if BudgetCopilot <= BudgetClaude || BudgetCopilot >= BudgetCodex {
		t.Error("BudgetCopilot should be between BudgetClaude and BudgetCodex")
	}
}
