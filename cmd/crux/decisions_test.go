package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/roygabriel/crux/internal/memory/journal"
	"github.com/roygabriel/crux/internal/memory/store"
	"github.com/roygabriel/crux/pkg/types"
)

func TestPrintDecisions_Format(t *testing.T) {
	decisions := []types.Decision{
		{
			ID:        "d1",
			Timestamp: time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC),
			PhaseID:   "1A",
			AgentID:   "eng-1",
			Action:    "refactor",
			Rationale: "reduce complexity",
		},
		{
			ID:        "d2",
			Timestamp: time.Date(2025, 6, 15, 11, 0, 0, 0, time.UTC),
			PhaseID:   "2A",
			AgentID:   "eng-2",
			Action:    "add tests",
			Rationale: "improve coverage for edge cases in the validation layer",
		},
	}

	// Smoke test: should not panic.
	printDecisions(decisions)
}

func TestPrintDecision_Format(t *testing.T) {
	d := types.Decision{
		ID:        "abc-123",
		Timestamp: time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC),
		PhaseID:   "1A",
		PromptNum: 2,
		AgentID:   "eng-1",
		Context:   "test context",
		Rationale: "test rationale",
		Action:    "test action",
		Outcome:   "test outcome",
	}

	// Smoke test: should not panic.
	printDecision(d)
}

func TestDecisionsExport_JSONL(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	st, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	decisions := []types.Decision{
		{
			ID:        "d1",
			Timestamp: time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC),
			PhaseID:   "1A",
			AgentID:   "eng-1",
			Context:   "ctx1",
			Rationale: "rat1",
			Action:    "act1",
		},
		{
			ID:        "d2",
			Timestamp: time.Date(2025, 6, 15, 11, 0, 0, 0, time.UTC),
			PhaseID:   "2A",
			AgentID:   "eng-2",
			Context:   "ctx2",
			Rationale: "rat2",
			Action:    "act2",
		},
	}

	for _, d := range decisions {
		if err := st.RecordDecision(ctx, d); err != nil {
			t.Fatal(err)
		}
	}

	j := journal.NewJournal(st, nil)

	var buf bytes.Buffer
	if err := j.Export(ctx, &buf); err != nil {
		t.Fatal(err)
	}

	// Verify valid JSONL.
	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("expected 2 JSONL lines, got %d", len(lines))
	}

	for i, line := range lines {
		var d types.Decision
		if err := json.Unmarshal(line, &d); err != nil {
			t.Fatalf("line %d: invalid JSON: %v", i, err)
		}
		if d.ID == "" {
			t.Errorf("line %d: missing ID", i)
		}
	}
}

func TestDecisionsRecent_EmptyDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "empty.db")

	st, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}

	j := journal.NewJournal(st, nil)

	ctx := context.Background()
	decisions, err := j.Recent(ctx, 20)
	if err != nil {
		t.Fatal(err)
	}

	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions, got %d", len(decisions))
	}
}

func TestPrintDecisionsJSON_EmptySlice(t *testing.T) {
	// Should produce "[]" not "null".
	if err := printDecisionsJSON(nil); err != nil {
		t.Fatal(err)
	}
}

func TestGetEditor(t *testing.T) {
	tests := []struct {
		name   string
		editor string
		visual string
		want   string
	}{
		{"EDITOR set", "nano", "", "nano"},
		{"VISUAL fallback", "", "code", "code"},
		{"default vi", "", "", "vi"},
		{"EDITOR takes precedence", "emacs", "code", "emacs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore env.
			origEditor := os.Getenv("EDITOR")
			origVisual := os.Getenv("VISUAL")
			defer func() {
				os.Setenv("EDITOR", origEditor)
				os.Setenv("VISUAL", origVisual)
			}()

			if tt.editor != "" {
				os.Setenv("EDITOR", tt.editor)
			} else {
				os.Unsetenv("EDITOR")
			}
			if tt.visual != "" {
				os.Setenv("VISUAL", tt.visual)
			} else {
				os.Unsetenv("VISUAL")
			}

			got := getEditor()
			if got != tt.want {
				t.Errorf("getEditor() = %q, want %q", got, tt.want)
			}
		})
	}
}
