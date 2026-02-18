package docgen

import (
	"strings"
	"testing"
)

func TestNewTokenBudget_KnownModel(t *testing.T) {
	t.Parallel()
	tb := NewTokenBudget("claude-sonnet-4-5-20250929", 8192, nil)

	if tb.MaxInputTokens != 200000 {
		t.Errorf("MaxInputTokens = %d, want 200000", tb.MaxInputTokens)
	}
	if tb.MaxOutputTokens != 8192 {
		t.Errorf("MaxOutputTokens = %d, want 8192", tb.MaxOutputTokens)
	}
	if tb.ReserveTokens != defaultReserveTokens {
		t.Errorf("ReserveTokens = %d, want %d", tb.ReserveTokens, defaultReserveTokens)
	}
}

func TestNewTokenBudget_UnknownModel(t *testing.T) {
	t.Parallel()
	tb := NewTokenBudget("claude-unknown-99", 4096, nil)

	if tb.MaxInputTokens != 200000 {
		t.Errorf("MaxInputTokens = %d, want 200000 (default)", tb.MaxInputTokens)
	}
	if tb.MaxOutputTokens != 4096 {
		t.Errorf("MaxOutputTokens = %d, want 4096", tb.MaxOutputTokens)
	}
}

func TestTokenBudget_Available(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		budget TokenBudget
		want   int
	}{
		{
			name: "normal budget",
			budget: TokenBudget{
				MaxInputTokens:     200000,
				SystemPromptTokens: 5000,
				ReserveTokens:      1000,
			},
			want: 194000,
		},
		{
			name: "system prompt fills budget",
			budget: TokenBudget{
				MaxInputTokens:     10000,
				SystemPromptTokens: 9500,
				ReserveTokens:      1000,
			},
			want: 0,
		},
		{
			name: "zero system prompt",
			budget: TokenBudget{
				MaxInputTokens:     200000,
				SystemPromptTokens: 0,
				ReserveTokens:      1000,
			},
			want: 199000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.budget.Available()
			if got != tc.want {
				t.Errorf("Available() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestTokenBudget_Fits(t *testing.T) {
	t.Parallel()

	tb := &TokenBudget{
		MaxInputTokens:     1000,
		SystemPromptTokens: 0,
		ReserveTokens:      0,
	}

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "empty string fits",
			content: "",
			want:    true,
		},
		{
			name:    "small content fits",
			content: "hello world",
			want:    true,
		},
		{
			name:    "exact boundary",
			content: strings.Repeat("x", 4000), // 4000/4 = 1000 tokens
			want:    true,
		},
		{
			name:    "over budget",
			content: strings.Repeat("x", 4004), // 4004/4 = 1001 tokens
			want:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tb.Fits(tc.content)
			if got != tc.want {
				t.Errorf("Fits(%q...) = %v, want %v", tc.content[:min(len(tc.content), 20)], got, tc.want)
			}
		})
	}
}

func TestEstimateTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty", "", 0},
		{"single char", "a", 1},
		{"short ASCII", "hello", 1},
		{"four chars", "abcd", 1},
		{"eight chars", "abcdefgh", 2},
		{"unicode", "こんにちは世界", 5}, // 21 bytes / 4 = 5
		{"long text", strings.Repeat("word ", 1000), 1250},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := EstimateTokens(tc.input)
			if got != tc.want {
				t.Errorf("EstimateTokens(%q) = %d, want %d", tc.input[:min(len(tc.input), 30)], got, tc.want)
			}
		})
	}
}

func TestEstimateCost_BatchDiscount(t *testing.T) {
	t.Parallel()

	tb := NewTokenBudget("claude-sonnet-4-5-20250929", 8192, nil)
	requests := []DocRequest{
		{PhaseID: "1", DocType: DocTypeSpec, Description: "Phase 1 spec"},
	}

	batch := tb.EstimateCost(requests, "claude-sonnet-4-5-20250929", GenerateModeBatch)
	stream := tb.EstimateCost(requests, "claude-sonnet-4-5-20250929", GenerateModeStream)

	if batch.NumRequests != 1 {
		t.Errorf("batch.NumRequests = %d, want 1", batch.NumRequests)
	}
	if stream.NumRequests != 1 {
		t.Errorf("stream.NumRequests = %d, want 1", stream.NumRequests)
	}

	// Batch should be cheaper due to 50% discount.
	if batch.EstCostUSD >= stream.EstCostUSD {
		t.Errorf("batch cost ($%.6f) should be less than stream cost ($%.6f)", batch.EstCostUSD, stream.EstCostUSD)
	}

	// Batch should be exactly 50% of stream for sonnet pricing.
	ratio := batch.EstCostUSD / stream.EstCostUSD
	if ratio < 0.49 || ratio > 0.51 {
		t.Errorf("batch/stream ratio = %.4f, want ~0.50", ratio)
	}
}

func TestEstimateCost_UnknownModel(t *testing.T) {
	t.Parallel()

	tb := NewTokenBudget("claude-unknown", 4096, nil)
	requests := []DocRequest{
		{PhaseID: "1", DocType: DocTypeSpec},
	}

	est := tb.EstimateCost(requests, "claude-unknown", GenerateModeBatch)
	if est.EstCostUSD != 0 {
		t.Errorf("EstCostUSD = %f, want 0 for unknown model", est.EstCostUSD)
	}
	if est.NumRequests != 1 {
		t.Errorf("NumRequests = %d, want 1", est.NumRequests)
	}
}

func TestEstimateCost_MultipleRequests(t *testing.T) {
	t.Parallel()

	tb := NewTokenBudget("claude-sonnet-4-5-20250929", 8192, nil)
	requests := []DocRequest{
		{PhaseID: "1", DocType: DocTypeSpec, Description: "Phase 1"},
		{PhaseID: "1", DocType: DocTypePrompt, Description: "Phase 1 prompts"},
		{PhaseID: "2", DocType: DocTypeSpec, Description: "Phase 2"},
	}

	est := tb.EstimateCost(requests, "claude-sonnet-4-5-20250929", GenerateModeStream)
	if est.NumRequests != 3 {
		t.Errorf("NumRequests = %d, want 3", est.NumRequests)
	}
	if est.EstInputTokens == 0 {
		t.Error("EstInputTokens should be > 0")
	}
	if est.EstOutputTokens != 3*8192 {
		t.Errorf("EstOutputTokens = %d, want %d", est.EstOutputTokens, 3*8192)
	}
	if est.EstCostUSD == 0 {
		t.Error("EstCostUSD should be > 0 for known model")
	}
}

func TestCostEstimate_String(t *testing.T) {
	t.Parallel()

	ce := CostEstimate{
		NumRequests:     2,
		EstInputTokens:  10000,
		EstOutputTokens: 16384,
		EstCostUSD:      0.2757,
		Model:           "claude-sonnet-4-5-20250929",
		Mode:            GenerateModeBatch,
	}

	s := ce.String()
	checks := []string{
		"2 request(s)",
		"~10000 input",
		"~16384 output",
		"claude-sonnet-4-5-20250929",
		"batch",
		"$0.2757",
	}

	for _, want := range checks {
		if !strings.Contains(s, want) {
			t.Errorf("String() = %q, want substring %q", s, want)
		}
	}
}
