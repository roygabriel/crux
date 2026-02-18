package docgen

import (
	"fmt"
	"log/slog"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// DefaultModel is the model used when none is specified.
const DefaultModel = "claude-sonnet-4-5-20250929"

// defaultMaxTokens is the fallback output token limit.
const defaultMaxTokens = 8192

// Generator coordinates document generation using the Anthropic API.
type Generator struct {
	client  anthropic.Client
	model   string
	budget  *TokenBudget
	retryer *Retryer
	logger  *slog.Logger
	opts    GenerateOptions
}

// NewGenerator creates a Generator with the given API key and options.
// It validates the key, applies defaults, and initialises the SDK client,
// TokenBudget, and Retryer.
func NewGenerator(apiKey string, opts GenerateOptions, logger *slog.Logger, extraOpts ...option.RequestOption) (*Generator, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("docgen: API key is required")
	}
	if logger == nil {
		logger = slog.Default()
	}

	model := opts.Model
	if model == "" {
		model = DefaultModel
	}

	maxTokens := opts.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}

	if opts.Mode == "" {
		opts.Mode = GenerateModeBatch
	}

	clientOpts := append([]option.RequestOption{option.WithAPIKey(apiKey)}, extraOpts...)
	client := anthropic.NewClient(clientOpts...)

	budget := NewTokenBudget(model, maxTokens, logger)
	retryer := DefaultRetryer(logger)

	return &Generator{
		client:  client,
		model:   model,
		budget:  budget,
		retryer: retryer,
		logger:  logger,
		opts:    opts,
	}, nil
}

// Validate checks that a slice of DocRequests is well-formed: non-empty
// PhaseID, valid DocType, no duplicates, and content fits the token budget.
func (g *Generator) Validate(requests []DocRequest) error {
	if len(requests) == 0 {
		return fmt.Errorf("docgen: no requests provided")
	}

	seen := make(map[string]bool)
	for i, req := range requests {
		if req.PhaseID == "" {
			return fmt.Errorf("docgen: request[%d]: phase_id is required", i)
		}
		if req.DocType != DocTypeSpec && req.DocType != DocTypePrompt {
			return fmt.Errorf("docgen: request[%d]: invalid doc_type %q (want %q or %q)", i, req.DocType, DocTypeSpec, DocTypePrompt)
		}

		key := req.PhaseID + ":" + string(req.DocType)
		if seen[key] {
			return fmt.Errorf("docgen: request[%d]: duplicate request for phase %q doc_type %q", i, req.PhaseID, req.DocType)
		}
		seen[key] = true

		content := req.Description + req.Context
		if content != "" && !g.budget.Fits(content) {
			return fmt.Errorf("docgen: request[%d]: content exceeds token budget (%d available)", i, g.budget.Available())
		}
	}

	return nil
}
