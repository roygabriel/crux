package store_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/roygabriel/crux/internal/memory/store"
	"github.com/roygabriel/crux/pkg/types"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}
	if err := s.Migrate(); err != nil {
		t.Fatalf("Migrate() error: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func makeDecision(id string, ts time.Time, phaseID, agentID string) types.Decision {
	return types.Decision{
		ID:        id,
		Timestamp: ts,
		PhaseID:   types.PhaseID(phaseID),
		PromptNum: 1,
		AgentID:   types.AgentID(agentID),
		Context:   "test context for " + id,
		Rationale: "test rationale for " + id,
		Action:    "test action for " + id,
		Outcome:   "test outcome for " + id,
	}
}

func TestMigrateIdempotent(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)

	// Second migrate should not error.
	if err := s.Migrate(); err != nil {
		t.Fatalf("second Migrate() error: %v", err)
	}
}

func TestRecordAndQueryDecision(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ctx := context.Background()

	d := makeDecision("d1", time.Now(), "phase-1", "agent-1")
	if err := s.RecordDecision(ctx, d); err != nil {
		t.Fatalf("RecordDecision() error: %v", err)
	}

	results, err := s.QueryDecisions(ctx, store.DecisionFilter{})
	if err != nil {
		t.Fatalf("QueryDecisions() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(results))
	}
	if results[0].ID != "d1" {
		t.Errorf("expected ID d1, got %q", results[0].ID)
	}
	if results[0].Context != d.Context {
		t.Errorf("expected context %q, got %q", d.Context, results[0].Context)
	}
}

func TestFilterByPhase(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now()

	s.RecordDecision(ctx, makeDecision("d1", now, "phase-1", "agent-1"))
	s.RecordDecision(ctx, makeDecision("d2", now.Add(time.Second), "phase-2", "agent-1"))
	s.RecordDecision(ctx, makeDecision("d3", now.Add(2*time.Second), "phase-1", "agent-2"))

	results, err := s.QueryDecisions(ctx, store.DecisionFilter{PhaseID: "phase-1"})
	if err != nil {
		t.Fatalf("QueryDecisions() error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 decisions for phase-1, got %d", len(results))
	}
}

func TestFilterByAgent(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now()

	s.RecordDecision(ctx, makeDecision("d1", now, "phase-1", "agent-1"))
	s.RecordDecision(ctx, makeDecision("d2", now.Add(time.Second), "phase-1", "agent-2"))

	results, err := s.QueryDecisions(ctx, store.DecisionFilter{AgentID: "agent-2"})
	if err != nil {
		t.Fatalf("QueryDecisions() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 decision for agent-2, got %d", len(results))
	}
	if results[0].ID != "d2" {
		t.Errorf("expected ID d2, got %q", results[0].ID)
	}
}

func TestFilterBySince(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ctx := context.Background()

	t1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)

	s.RecordDecision(ctx, makeDecision("d1", t1, "p1", "a1"))
	s.RecordDecision(ctx, makeDecision("d2", t2, "p1", "a1"))
	s.RecordDecision(ctx, makeDecision("d3", t3, "p1", "a1"))

	results, err := s.QueryDecisions(ctx, store.DecisionFilter{Since: t2})
	if err != nil {
		t.Fatalf("QueryDecisions() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 decision after t2, got %d", len(results))
	}
	if results[0].ID != "d3" {
		t.Errorf("expected ID d3, got %q", results[0].ID)
	}
}

func TestFilterByLimit(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now()

	for i := 0; i < 5; i++ {
		s.RecordDecision(ctx, makeDecision(
			"d"+string(rune('0'+i)),
			now.Add(time.Duration(i)*time.Second),
			"p1", "a1",
		))
	}

	results, err := s.QueryDecisions(ctx, store.DecisionFilter{Limit: 2})
	if err != nil {
		t.Fatalf("QueryDecisions() error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 decisions with limit, got %d", len(results))
	}
}

func TestReverseChronological(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ctx := context.Background()

	t1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)

	// Insert out of order.
	s.RecordDecision(ctx, makeDecision("d2", t2, "p1", "a1"))
	s.RecordDecision(ctx, makeDecision("d1", t1, "p1", "a1"))
	s.RecordDecision(ctx, makeDecision("d3", t3, "p1", "a1"))

	results, err := s.QueryDecisions(ctx, store.DecisionFilter{})
	if err != nil {
		t.Fatalf("QueryDecisions() error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 decisions, got %d", len(results))
	}

	// Most recent first.
	if results[0].ID != "d3" {
		t.Errorf("expected first result d3, got %q", results[0].ID)
	}
	if results[1].ID != "d2" {
		t.Errorf("expected second result d2, got %q", results[1].ID)
	}
	if results[2].ID != "d1" {
		t.Errorf("expected third result d1, got %q", results[2].ID)
	}
}

func TestSearchFTS5(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now()

	d1 := makeDecision("d1", now, "p1", "a1")
	d1.Context = "refactoring the authentication module"
	d1.Rationale = "reduce complexity and improve testability"
	s.RecordDecision(ctx, d1)

	d2 := makeDecision("d2", now.Add(time.Second), "p1", "a1")
	d2.Context = "adding database migration scripts"
	d2.Rationale = "schema changes for new feature"
	s.RecordDecision(ctx, d2)

	results, err := s.SearchDecisions(ctx, "authentication", 10)
	if err != nil {
		t.Fatalf("SearchDecisions() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 FTS result, got %d", len(results))
	}
	if results[0].ID != "d1" {
		t.Errorf("expected search result d1, got %q", results[0].ID)
	}
}

func TestSearchNoResults(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ctx := context.Background()

	results, err := s.SearchDecisions(ctx, "nonexistent", 10)
	if err != nil {
		t.Fatalf("SearchDecisions() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestRecordAndQueryEvent(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ctx := context.Background()

	e := store.Event{
		ID:        "e1",
		Type:      "agent_started",
		AgentID:   "agent-1",
		PhaseID:   "phase-1",
		Data:      `{"task":"build"}`,
		Timestamp: time.Now(),
	}
	if err := s.RecordEvent(ctx, e); err != nil {
		t.Fatalf("RecordEvent() error: %v", err)
	}

	results, err := s.QueryEvents(ctx, store.EventFilter{})
	if err != nil {
		t.Fatalf("QueryEvents() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 event, got %d", len(results))
	}
	if results[0].ID != "e1" {
		t.Errorf("expected event ID e1, got %q", results[0].ID)
	}
	if results[0].Data != `{"task":"build"}` {
		t.Errorf("expected event data preserved, got %q", results[0].Data)
	}
}

func TestFilterEventsByType(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now()

	s.RecordEvent(ctx, store.Event{ID: "e1", Type: "agent_started", Timestamp: now})
	s.RecordEvent(ctx, store.Event{ID: "e2", Type: "phase_completed", Timestamp: now.Add(time.Second)})
	s.RecordEvent(ctx, store.Event{ID: "e3", Type: "agent_started", Timestamp: now.Add(2 * time.Second)})

	results, err := s.QueryEvents(ctx, store.EventFilter{Type: "agent_started"})
	if err != nil {
		t.Fatalf("QueryEvents() error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 agent_started events, got %d", len(results))
	}
}

func TestPruneRemovesOld(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ctx := context.Background()

	old := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	recent := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	cutoff := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	s.RecordDecision(ctx, makeDecision("d-old", old, "p1", "a1"))
	s.RecordDecision(ctx, makeDecision("d-new", recent, "p1", "a1"))
	s.RecordEvent(ctx, store.Event{ID: "e-old", Type: "test", Timestamp: old})
	s.RecordEvent(ctx, store.Event{ID: "e-new", Type: "test", Timestamp: recent})

	pruned, err := s.Prune(ctx, cutoff)
	if err != nil {
		t.Fatalf("Prune() error: %v", err)
	}
	if pruned != 2 {
		t.Errorf("expected 2 pruned rows, got %d", pruned)
	}

	decisions, _ := s.QueryDecisions(ctx, store.DecisionFilter{})
	if len(decisions) != 1 {
		t.Errorf("expected 1 remaining decision, got %d", len(decisions))
	}

	events, _ := s.QueryEvents(ctx, store.EventFilter{})
	if len(events) != 1 {
		t.Errorf("expected 1 remaining event, got %d", len(events))
	}
}

func TestPrunePreservesRecent(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ctx := context.Background()

	recent := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	cutoff := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	s.RecordDecision(ctx, makeDecision("d1", recent, "p1", "a1"))

	pruned, err := s.Prune(ctx, cutoff)
	if err != nil {
		t.Fatalf("Prune() error: %v", err)
	}
	if pruned != 0 {
		t.Errorf("expected 0 pruned rows, got %d", pruned)
	}

	decisions, _ := s.QueryDecisions(ctx, store.DecisionFilter{})
	if len(decisions) != 1 {
		t.Errorf("expected 1 decision preserved, got %d", len(decisions))
	}
}
