package orchestrator

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/roygabriel/crux/internal/agent"
	"github.com/roygabriel/crux/internal/config"
	"github.com/roygabriel/crux/internal/runner"
	"github.com/roygabriel/crux/pkg/types"
)

type fakeTaskRunner struct {
	result runner.Result
	err    error
}

func (f *fakeTaskRunner) Name() string { return "fake" }

func (f *fakeTaskRunner) Run(_ context.Context, _ runner.Request) (runner.Result, error) {
	return f.result, f.err
}

func TestDispatchDeterministic_NoRunner(t *testing.T) {
	o := &Orchestrator{
		cfg: &config.Config{
			Project: config.ProjectConfig{Root: t.TempDir(), StateDir: t.TempDir()},
		},
		logger:               slog.Default(),
		worldState:           NewWorldState("sess-1"),
		deterministicRuns:    make(map[types.AgentID]deterministicRunState),
		deterministicResults: make(chan deterministicRunResult, 1),
	}
	inst := &agent.AgentInstance{
		Agent: types.Agent{
			ID:     "engineer-1",
			Plugin: "codex",
			Role:   types.RoleEngineer,
		},
	}
	err := o.dispatchDeterministic(context.Background(), inst, makePromptKey("1A", 1), "do work")
	if !errors.Is(err, errNoDeterministicRunner) {
		t.Fatalf("dispatchDeterministic() error = %v, want %v", err, errNoDeterministicRunner)
	}
}

func TestDispatchDeterministic_WritesEnvelopeAndLedger(t *testing.T) {
	dir := t.TempDir()
	reg := runner.NewRegistry(slog.Default())
	reg.Register("codex", &fakeTaskRunner{
		result: runner.Result{
			Output:     "done",
			StartedAt:  time.Now().UTC(),
			FinishedAt: time.Now().UTC(),
		},
	})

	o := &Orchestrator{
		cfg: &config.Config{
			Project: config.ProjectConfig{
				Root:     dir,
				StateDir: dir,
			},
		},
		logger:               slog.Default(),
		worldState:           NewWorldState("sess-1"),
		runners:              reg,
		deterministicRuns:    make(map[types.AgentID]deterministicRunState),
		deterministicResults: make(chan deterministicRunResult, 1),
		envelopeDir:          filepath.Join(dir, "evl", "envelopes"),
		ledgerPath:           filepath.Join(dir, "evl", "progress-ledger.jsonl"),
	}
	inst := &agent.AgentInstance{
		Agent: types.Agent{
			ID:     "engineer-1",
			Plugin: "codex",
			Role:   types.RoleEngineer,
		},
	}
	key := makePromptKey("1A", 1)

	if err := o.dispatchDeterministic(context.Background(), inst, key, "implement feature"); err != nil {
		t.Fatalf("dispatchDeterministic() error = %v", err)
	}
	if !o.isDeterministicRunActive("engineer-1") {
		t.Fatal("expected deterministic run state to be active")
	}

	select {
	case got := <-o.deterministicResults:
		if got.AgentID != "engineer-1" {
			t.Fatalf("result agent = %s, want engineer-1", got.AgentID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for deterministic result")
	}

	envelopes, err := os.ReadDir(o.envelopeDir)
	if err != nil {
		t.Fatalf("ReadDir(envelopes) error = %v", err)
	}
	if len(envelopes) != 1 {
		t.Fatalf("envelope file count = %d, want 1", len(envelopes))
	}

	ledgerData, err := os.ReadFile(o.ledgerPath)
	if err != nil {
		t.Fatalf("ReadFile(ledger) error = %v", err)
	}
	if len(ledgerData) == 0 {
		t.Fatal("expected ledger entry")
	}
}

func TestMonitorDeterministicRuns_ForceClearsStaleRun(t *testing.T) {
	o := &Orchestrator{
		logger:               slog.Default(),
		deterministicRuns:    make(map[types.AgentID]deterministicRunState),
		deterministicResults: make(chan deterministicRunResult, 1),
	}
	o.deterministicRuns["engineer-1"] = deterministicRunState{
		RunID:     "run-1",
		Key:       makePromptKey("1A", 1),
		StartedAt: time.Now().UTC().Add(-10 * time.Minute),
		Deadline:  time.Now().UTC().Add(-time.Second),
	}

	o.monitorDeterministicRuns(context.Background())
	if o.isDeterministicRunActive("engineer-1") {
		t.Fatal("expected stale deterministic run to be cleared")
	}
}

func TestHandleDeterministicResult_IgnoresLateResult(t *testing.T) {
	o := &Orchestrator{
		logger:               slog.Default(),
		deterministicRuns:    map[types.AgentID]deterministicRunState{},
		deterministicResults: make(chan deterministicRunResult, 1),
	}
	done := deterministicRunResult{
		AgentID: "engineer-1",
		State: deterministicRunState{
			RunID: "run-late",
			Key:   makePromptKey("1A", 1),
		},
		Result: runner.Result{TerminationReason: "completed"},
	}

	// Should return without panic even though run state no longer exists.
	o.handleDeterministicResult(context.Background(), done)
}
