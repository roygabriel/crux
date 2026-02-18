package instruct

import (
	"context"
	"fmt"
	"log/slog"
)

// BudgetOrchestrator is the default token budget for the orchestrator system prompt.
const BudgetOrchestrator = 8000

// OrchestratorPromptBuilder assembles the orchestrator's system prompt from
// aggregated instruction data and the orchestrator template.
type OrchestratorPromptBuilder struct {
	aggregator *Aggregator
	renderer   *Renderer
	logger     *slog.Logger
}

// NewOrchestratorPromptBuilder creates an OrchestratorPromptBuilder.
func NewOrchestratorPromptBuilder(agg *Aggregator, r *Renderer, logger *slog.Logger) *OrchestratorPromptBuilder {
	if logger == nil {
		logger = slog.Default()
	}
	return &OrchestratorPromptBuilder{
		aggregator: agg,
		renderer:   r,
		logger:     logger,
	}
}

// Build assembles and renders the orchestrator system prompt.
func (b *OrchestratorPromptBuilder) Build(ctx context.Context) (string, error) {
	data, err := b.aggregator.BuildForOrchestrator(ctx)
	if err != nil {
		return "", fmt.Errorf("aggregating orchestrator data: %w", err)
	}

	content, err := b.renderer.RenderTemplate("orchestrator", *data)
	if err != nil {
		return "", fmt.Errorf("rendering orchestrator prompt: %w", err)
	}

	if EstimateTokens(content) > BudgetOrchestrator {
		content = TruncateToTokens(content, BudgetOrchestrator)
	}

	return content, nil
}

// BuildWithWorldState assembles the orchestrator system prompt with injected
// world state JSON. The world state is available in the template as
// [[ index .Custom "World State" ]].
func (b *OrchestratorPromptBuilder) BuildWithWorldState(ctx context.Context, worldStateJSON string) (string, error) {
	data, err := b.aggregator.BuildForOrchestrator(ctx)
	if err != nil {
		return "", fmt.Errorf("aggregating orchestrator data: %w", err)
	}

	if data.Custom == nil {
		data.Custom = make(map[string]string)
	}
	data.Custom["World State"] = worldStateJSON

	content, err := b.renderer.RenderTemplate("orchestrator", *data)
	if err != nil {
		return "", fmt.Errorf("rendering orchestrator prompt: %w", err)
	}

	if EstimateTokens(content) > BudgetOrchestrator {
		content = TruncateToTokens(content, BudgetOrchestrator)
	}

	return content, nil
}
