package docgen

import (
	"fmt"
	"log/slog"
)

// ModelPricing holds per-million-token pricing for a model.
type ModelPricing struct {
	// InputPerMillion is the cost in USD per million input tokens.
	InputPerMillion float64 `json:"input_per_million"`
	// OutputPerMillion is the cost in USD per million output tokens.
	OutputPerMillion float64 `json:"output_per_million"`
	// BatchDiscount is the fractional discount applied in batch mode (e.g. 0.5 = 50% off).
	BatchDiscount float64 `json:"batch_discount"`
}

// KnownPricing maps model identifiers to their pricing. Prices are in USD
// per million tokens as of the knowledge cutoff.
var KnownPricing = map[string]ModelPricing{
	"claude-sonnet-4-5-20250929": {
		InputPerMillion:  3.00,
		OutputPerMillion: 15.00,
		BatchDiscount:    0.50,
	},
	"claude-opus-4-20250514": {
		InputPerMillion:  15.00,
		OutputPerMillion: 75.00,
		BatchDiscount:    0.50,
	},
	"claude-haiku-3-5-20241022": {
		InputPerMillion:  0.80,
		OutputPerMillion: 4.00,
		BatchDiscount:    0.50,
	},
}

// defaultModelLimits maps model identifiers to their maximum input token limits.
var defaultModelLimits = map[string]int{
	"claude-sonnet-4-5-20250929": 200000,
	"claude-opus-4-20250514":    200000,
	"claude-haiku-3-5-20241022": 200000,
}

// TokenBudget tracks token allocation for a model and output configuration.
type TokenBudget struct {
	// MaxInputTokens is the model's maximum input context window.
	MaxInputTokens int `json:"max_input_tokens"`
	// MaxOutputTokens is the configured maximum output tokens per request.
	MaxOutputTokens int `json:"max_output_tokens"`
	// SystemPromptTokens is the estimated token count of the system prompt.
	SystemPromptTokens int `json:"system_prompt_tokens"`
	// ReserveTokens is a safety margin subtracted from available input.
	ReserveTokens int `json:"reserve_tokens"`
}

// defaultReserveTokens is the safety margin for prompt overhead.
const defaultReserveTokens = 1000

// NewTokenBudget creates a TokenBudget for the given model and output limit.
// It looks up model-specific limits from defaultModelLimits, falling back to
// 200000 for unknown models.
func NewTokenBudget(model string, maxOutput int, logger *slog.Logger) *TokenBudget {
	if logger == nil {
		logger = slog.Default()
	}

	maxInput, ok := defaultModelLimits[model]
	if !ok {
		maxInput = 200000
		logger.Warn("unknown model for token budget, using default limit",
			"model", model,
			"default_max_input", maxInput,
		)
	}

	return &TokenBudget{
		MaxInputTokens:  maxInput,
		MaxOutputTokens: maxOutput,
		ReserveTokens:   defaultReserveTokens,
	}
}

// Available returns the number of input tokens remaining for user content
// after accounting for the system prompt and reserve.
func (tb *TokenBudget) Available() int {
	avail := tb.MaxInputTokens - tb.SystemPromptTokens - tb.ReserveTokens
	if avail < 0 {
		return 0
	}
	return avail
}

// Fits reports whether the estimated token count of content is within the
// available budget.
func (tb *TokenBudget) Fits(content string) bool {
	return EstimateTokens(content) <= tb.Available()
}

// EstimateTokens returns a rough token count estimate using the len/4
// heuristic. This is intentionally conservative for budget checking.
func EstimateTokens(s string) int {
	if len(s) == 0 {
		return 0
	}
	est := len(s) / 4
	if est == 0 {
		return 1
	}
	return est
}

// CostEstimate summarises the projected cost for a generation run.
type CostEstimate struct {
	// NumRequests is the number of API requests.
	NumRequests int `json:"num_requests"`
	// EstInputTokens is the estimated total input tokens.
	EstInputTokens int `json:"est_input_tokens"`
	// EstOutputTokens is the estimated total output tokens.
	EstOutputTokens int `json:"est_output_tokens"`
	// EstCostUSD is the estimated total cost in USD.
	EstCostUSD float64 `json:"est_cost_usd"`
	// Model is the model identifier.
	Model string `json:"model"`
	// Mode is the generation mode (batch or stream).
	Mode GenerateMode `json:"mode"`
}

// String formats the cost estimate as a human-readable summary.
func (ce CostEstimate) String() string {
	return fmt.Sprintf(
		"%d request(s), ~%d input + ~%d output tokens, model=%s, mode=%s, est. $%.4f USD",
		ce.NumRequests, ce.EstInputTokens, ce.EstOutputTokens, ce.Model, ce.Mode, ce.EstCostUSD,
	)
}

// EstimateCost projects the cost for a set of requests using the given model
// and mode. If the model has no known pricing, the estimate uses zero cost.
func (tb *TokenBudget) EstimateCost(requests []DocRequest, model string, mode GenerateMode) CostEstimate {
	pricing, ok := KnownPricing[model]
	if !ok {
		return CostEstimate{
			NumRequests: len(requests),
			Model:       model,
			Mode:        mode,
		}
	}

	var totalInput, totalOutput int
	for _, req := range requests {
		inputEst := EstimateTokens(req.Description) + EstimateTokens(req.Context) + tb.SystemPromptTokens + tb.ReserveTokens
		totalInput += inputEst
		totalOutput += tb.MaxOutputTokens
	}

	inputCost := float64(totalInput) / 1_000_000 * pricing.InputPerMillion
	outputCost := float64(totalOutput) / 1_000_000 * pricing.OutputPerMillion

	totalCost := inputCost + outputCost
	if mode == GenerateModeBatch {
		totalCost *= (1 - pricing.BatchDiscount)
	}

	return CostEstimate{
		NumRequests:     len(requests),
		EstInputTokens:  totalInput,
		EstOutputTokens: totalOutput,
		EstCostUSD:      totalCost,
		Model:           model,
		Mode:            mode,
	}
}
