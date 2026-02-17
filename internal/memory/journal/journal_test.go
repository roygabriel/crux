package journal_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/roygabriel/crux/internal/memory/journal"
	"github.com/roygabriel/crux/internal/memory/store"
	"github.com/roygabriel/crux/pkg/types"
)

func newTestJournal(t *testing.T) *journal.Journal {
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
	return journal.NewJournal(s)
}

func validDecision() types.Decision {
	return types.Decision{
		PhaseID:   "phase-1",
		PromptNum: 1,
		AgentID:   "agent-1",
		Context:   "test context",
		Rationale: "test rationale",
		Action:    "test action",
	}
}

func TestRecordGeneratesID(t *testing.T) {
	t.Parallel()
	j := newTestJournal(t)
	ctx := context.Background()

	d := validDecision()
	if err := j.Record(ctx, d); err != nil {
		t.Fatalf("Record() error: %v", err)
	}

	results, err := j.Recent(ctx, 1)
	if err != nil {
		t.Fatalf("Recent() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID == "" {
		t.Error("expected generated ID, got empty string")
	}
}

func TestRecordSetsTimestamp(t *testing.T) {
	t.Parallel()
	j := newTestJournal(t)
	ctx := context.Background()

	before := time.Now().UTC().Add(-time.Second)
	d := validDecision()
	if err := j.Record(ctx, d); err != nil {
		t.Fatalf("Record() error: %v", err)
	}
	after := time.Now().UTC().Add(time.Second)

	results, err := j.Recent(ctx, 1)
	if err != nil {
		t.Fatalf("Recent() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	ts := results[0].Timestamp
	if ts.Before(before) || ts.After(after) {
		t.Errorf("timestamp %v not in expected range [%v, %v]", ts, before, after)
	}
}

func TestRecordValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		modify  func(*types.Decision)
		wantErr error
	}{
		{
			name:    "missing context",
			modify:  func(d *types.Decision) { d.Context = "" },
			wantErr: journal.ErrMissingContext,
		},
		{
			name:    "missing rationale",
			modify:  func(d *types.Decision) { d.Rationale = "" },
			wantErr: journal.ErrMissingRationale,
		},
		{
			name:    "missing action",
			modify:  func(d *types.Decision) { d.Action = "" },
			wantErr: journal.ErrMissingAction,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			j := newTestJournal(t)
			ctx := context.Background()

			d := validDecision()
			tt.modify(&d)

			err := j.Record(ctx, d)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Record() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestSearchReturnsRelevant(t *testing.T) {
	t.Parallel()
	j := newTestJournal(t)
	ctx := context.Background()

	d1 := validDecision()
	d1.Context = "refactoring authentication module"
	j.Record(ctx, d1)

	d2 := validDecision()
	d2.Context = "adding database indexes"
	j.Record(ctx, d2)

	results, err := j.Search(ctx, "authentication", 10)
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(results))
	}
	if results[0].Context != d1.Context {
		t.Errorf("expected context %q, got %q", d1.Context, results[0].Context)
	}
}

func TestByPhase(t *testing.T) {
	t.Parallel()
	j := newTestJournal(t)
	ctx := context.Background()

	d1 := validDecision()
	d1.PhaseID = "phase-1"
	j.Record(ctx, d1)

	d2 := validDecision()
	d2.PhaseID = "phase-2"
	j.Record(ctx, d2)

	results, err := j.ByPhase(ctx, "phase-1")
	if err != nil {
		t.Fatalf("ByPhase() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for phase-1, got %d", len(results))
	}
}

func TestByAgent(t *testing.T) {
	t.Parallel()
	j := newTestJournal(t)
	ctx := context.Background()

	d1 := validDecision()
	d1.AgentID = "agent-1"
	j.Record(ctx, d1)

	d2 := validDecision()
	d2.AgentID = "agent-2"
	j.Record(ctx, d2)

	results, err := j.ByAgent(ctx, "agent-2")
	if err != nil {
		t.Fatalf("ByAgent() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for agent-2, got %d", len(results))
	}
}

func TestRecentLimit(t *testing.T) {
	t.Parallel()
	j := newTestJournal(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		d := validDecision()
		d.Context = "decision " + string(rune('A'+i))
		j.Record(ctx, d)
	}

	results, err := j.Recent(ctx, 3)
	if err != nil {
		t.Fatalf("Recent() error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 recent results, got %d", len(results))
	}
}

func TestRecentReverseChronological(t *testing.T) {
	t.Parallel()
	j := newTestJournal(t)
	ctx := context.Background()

	d1 := validDecision()
	d1.Timestamp = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	d1.Context = "first decision"
	j.Record(ctx, d1)

	d2 := validDecision()
	d2.Timestamp = time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	d2.Context = "second decision"
	j.Record(ctx, d2)

	results, err := j.Recent(ctx, 10)
	if err != nil {
		t.Fatalf("Recent() error: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}

	// Most recent first.
	if results[0].Timestamp.Before(results[1].Timestamp) {
		t.Error("expected reverse chronological order")
	}
}

func TestExportJSONL(t *testing.T) {
	t.Parallel()
	j := newTestJournal(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		d := validDecision()
		d.Context = "export decision " + string(rune('A'+i))
		j.Record(ctx, d)
	}

	var buf bytes.Buffer
	if err := j.Export(ctx, &buf); err != nil {
		t.Fatalf("Export() error: %v", err)
	}

	// Verify each line is valid JSON.
	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) != 3 {
		t.Fatalf("expected 3 JSONL lines, got %d", len(lines))
	}
	for i, line := range lines {
		var d types.Decision
		if err := json.Unmarshal(line, &d); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
		}
		if d.ID == "" {
			t.Errorf("line %d has empty ID", i)
		}
	}
}

func TestExportEmpty(t *testing.T) {
	t.Parallel()
	j := newTestJournal(t)
	ctx := context.Background()

	var buf bytes.Buffer
	if err := j.Export(ctx, &buf); err != nil {
		t.Fatalf("Export() error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty export, got %d bytes", buf.Len())
	}
}
