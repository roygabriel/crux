package instruct

import (
	"context"
	"strings"
	"testing"
)

func buildOrchestratorBuilder(t *testing.T, deps AggregatorDeps) *OrchestratorPromptBuilder {
	t.Helper()

	agg := NewAggregator(deps)
	tfs, err := TemplatesFS()
	if err != nil {
		t.Fatalf("TemplatesFS() error: %v", err)
	}
	r, err := NewRenderer(tfs, nil)
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}
	return NewOrchestratorPromptBuilder(agg, r, nil)
}

func TestOrchestratorPromptBuild(t *testing.T) {
	t.Parallel()

	builder := buildOrchestratorBuilder(t, fullDeps())
	content, err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Should contain orchestrator identity.
	if !strings.Contains(content, "Crux Orchestrator") {
		t.Error("content missing orchestrator identity")
	}

	// Should contain agent registry with agents from fullDeps.
	if !strings.Contains(content, "eng-1") {
		t.Error("content missing agent eng-1 in registry")
	}
	if !strings.Contains(content, "rev-1") {
		t.Error("content missing agent rev-1 in registry")
	}

	// Should contain decision framework.
	if !strings.Contains(content, "Decision Framework") {
		t.Error("content missing Decision Framework section")
	}

	// Should contain hard constraints.
	if !strings.Contains(content, "Hard Constraints") {
		t.Error("content missing Hard Constraints section")
	}

	// Should contain project info.
	if !strings.Contains(content, "crux") {
		t.Error("content missing project name")
	}
}

func TestOrchestratorPromptEmptyAgentRegistry(t *testing.T) {
	t.Parallel()

	deps := fullDeps()
	deps.AgentReg = &mockAgentLister{agents: nil}

	builder := buildOrchestratorBuilder(t, deps)
	content, err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Should render without panic and indicate no agents.
	if !strings.Contains(content, "No agents registered") {
		t.Error("content should indicate no agents when registry is empty")
	}
}

func TestOrchestratorPromptWithWorldState(t *testing.T) {
	t.Parallel()

	builder := buildOrchestratorBuilder(t, fullDeps())
	worldJSON := `{"phase":"14A","agents":{"eng-1":{"status":"busy"}}}`

	content, err := builder.BuildWithWorldState(context.Background(), worldJSON)
	if err != nil {
		t.Fatalf("BuildWithWorldState() error: %v", err)
	}

	if !strings.Contains(content, "World State") {
		t.Error("content missing World State section heading")
	}
	if !strings.Contains(content, worldJSON) {
		t.Error("content missing world state JSON payload")
	}
}

func TestOrchestratorPromptWithoutWorldState(t *testing.T) {
	t.Parallel()

	builder := buildOrchestratorBuilder(t, fullDeps())
	content, err := builder.BuildWithWorldState(context.Background(), "")
	if err != nil {
		t.Fatalf("BuildWithWorldState() error: %v", err)
	}

	// World State heading should still appear (Custom map entry exists but is empty).
	// The key is that the output shouldn't contain stale or garbage data.
	if strings.Contains(content, `"phase"`) {
		t.Error("content should not contain world state JSON when empty string passed")
	}
}

func TestOrchestratorPromptDecisionFrameworkJSON(t *testing.T) {
	t.Parallel()

	builder := buildOrchestratorBuilder(t, fullDeps())
	content, err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// Decision JSON schema fields should be present.
	for _, field := range []string{"action", "agent", "reasoning", "details"} {
		if !strings.Contains(content, `"`+field+`"`) {
			t.Errorf("content missing decision schema field %q", field)
		}
	}

	// Decision action types should be present.
	if !strings.Contains(content, "assign | wait | escalate | complete") {
		t.Error("content missing decision action types")
	}
}

func TestOrchestratorPromptBudgetTruncation(t *testing.T) {
	t.Parallel()

	// Create deps with large memory content to exceed the 8000-token budget.
	deps := fullDeps()
	largeContent := strings.Repeat("This is a line of filler content to exceed the token budget. ", 1000)
	deps.Bank = &mockBank{
		files: map[string]string{
			"project-brief.md":   largeContent,
			"active-context.md":  largeContent,
			"tech-context.md":    largeContent,
			"system-patterns.md": largeContent,
		},
	}

	builder := buildOrchestratorBuilder(t, deps)
	content, err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	tokens := EstimateTokens(content)
	if tokens > BudgetOrchestrator {
		t.Errorf("content tokens %d exceeds budget %d", tokens, BudgetOrchestrator)
	}

	if !strings.Contains(content, "[...truncated]") {
		t.Error("truncated content should contain truncation marker")
	}
}

func TestOrchestratorPromptPreferences(t *testing.T) {
	t.Parallel()

	builder := buildOrchestratorBuilder(t, fullDeps())
	content, err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// fullDeps provides Testing and ErrorHandling prefs.
	if !strings.Contains(content, "Quality Standards") {
		t.Error("content missing Quality Standards section")
	}
	if !strings.Contains(content, "Table-driven tests") {
		t.Error("content missing testing preference text")
	}
	if !strings.Contains(content, "Wrap with context") {
		t.Error("content missing error handling preference text")
	}
}

func TestOrchestratorPromptHardConstraints(t *testing.T) {
	t.Parallel()

	builder := buildOrchestratorBuilder(t, fullDeps())
	content, err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	constraints := []string{
		"NEVER write code",
		"NEVER modify files",
		"NEVER skip exit criteria",
		"NEVER assign work outside",
		"NEVER ignore agent status",
		"NEVER fabricate progress",
		"NEVER bypass the decision framework",
	}

	for _, c := range constraints {
		if !strings.Contains(content, c) {
			t.Errorf("content missing hard constraint %q", c)
		}
	}
}
