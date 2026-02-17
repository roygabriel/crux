package orchestrator_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/roygabriel/crux/internal/orchestrator"
	"github.com/roygabriel/crux/pkg/types"
)

// mockJournal implements JournalSearcher for testing.
type mockJournal struct {
	semanticResults []types.Decision
	semanticErr     error
	searchResults   []types.Decision
	searchErr       error
}

func (m *mockJournal) SemanticSearch(_ context.Context, _ string, _ int) ([]types.Decision, error) {
	return m.semanticResults, m.semanticErr
}

func (m *mockJournal) Search(_ context.Context, _ string, _ int) ([]types.Decision, error) {
	return m.searchResults, m.searchErr
}

func TestBeforeDecision_WithResults(t *testing.T) {
	j := &mockJournal{
		semanticResults: []types.Decision{
			{PhaseID: "2A", PromptNum: 3, Action: "Chose chi router", Rationale: "lightweight", AgentID: "claude-1"},
		},
		searchResults: []types.Decision{
			{PhaseID: "3A", Context: "SQLite WASM driver had import issues"},
		},
	}

	rag := orchestrator.NewDecisionRAG(j, nil)
	ctx := context.Background()

	result, err := rag.BeforeDecision(ctx, "choosing HTTP framework")
	if err != nil {
		t.Fatalf("BeforeDecision() error = %v", err)
	}

	if !strings.Contains(result, "Chose chi router") {
		t.Error("result should contain decision text")
	}
	if !strings.Contains(result, "lightweight") {
		t.Error("result should contain rationale")
	}
	if !strings.Contains(result, "claude-1") {
		t.Error("result should contain agent ID")
	}
	if !strings.Contains(result, "SQLite WASM driver") {
		t.Error("result should contain blocker context")
	}
}

func TestBeforeDecision_NoResults(t *testing.T) {
	j := &mockJournal{}

	rag := orchestrator.NewDecisionRAG(j, nil)
	ctx := context.Background()

	result, err := rag.BeforeDecision(ctx, "some situation")
	if err != nil {
		t.Fatalf("BeforeDecision() error = %v", err)
	}

	if !strings.Contains(result, "<decision_context>") {
		t.Error("result should contain opening XML tag")
	}
	if !strings.Contains(result, "</decision_context>") {
		t.Error("result should contain closing XML tag")
	}
}

func TestBeforeDecision_JournalError(t *testing.T) {
	j := &mockJournal{
		semanticErr: errors.New("vector index unavailable"),
		searchErr:   errors.New("FTS5 error"),
	}

	rag := orchestrator.NewDecisionRAG(j, nil)
	ctx := context.Background()

	result, err := rag.BeforeDecision(ctx, "some situation")
	if err != nil {
		t.Fatalf("BeforeDecision() should not return error on journal failure, got %v", err)
	}

	if !strings.Contains(result, "<decision_context>") {
		t.Error("result should still contain XML tags on error")
	}
}

func TestBeforeDecision_Format(t *testing.T) {
	j := &mockJournal{
		semanticResults: []types.Decision{
			{PhaseID: "1A", PromptNum: 1, Action: "used slog", Rationale: "structured logging"},
		},
		searchResults: []types.Decision{
			{PhaseID: "2B", Context: "CGO linking failed"},
		},
	}

	rag := orchestrator.NewDecisionRAG(j, nil)
	ctx := context.Background()

	result, err := rag.BeforeDecision(ctx, "logging choice")
	if err != nil {
		t.Fatalf("BeforeDecision() error = %v", err)
	}

	if !strings.HasPrefix(result, "<decision_context>") {
		t.Error("result should start with <decision_context>")
	}
	if !strings.HasSuffix(result, "</decision_context>") {
		t.Error("result should end with </decision_context>")
	}
	if !strings.Contains(result, "Relevant past decisions:") {
		t.Error("result should contain decisions header")
	}
	if !strings.Contains(result, "Related blockers:") {
		t.Error("result should contain blockers header")
	}
}
