package phase_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/roygabriel/crux/internal/phase"
	"github.com/roygabriel/crux/pkg/types"
)

func TestEngine_LoadAll(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "PHASE1.md", "# Phase 1: First\n\n## Status\nPlanned\n\n## Depends On\nNone\n\n## Exit Criteria\n- [ ] `true` exits 0\n")
	writeSpec(t, dir, "PHASE2.md", "# Phase 2: Second\n\n## Status\nPlanned\n\n## Depends On\nPhase 1\n\n## Exit Criteria\n- [ ] `true` exits 0\n")
	writeSpec(t, dir, "PHASE3.md", "# Phase 3: Third\n\n## Status\nPlanned\n\n## Depends On\nPhase 2\n\n## Exit Criteria\n- [ ] `true` exits 0\n")

	runner := phase.NewGateRunner(dir, 5*time.Second, slog.Default())
	engine, err := phase.NewEngine(dir, runner, nil, slog.Default())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadAll(); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	progress := engine.Progress()
	if len(progress) != 3 {
		t.Fatalf("len(progress) = %d, want 3", len(progress))
	}
	for _, id := range []types.PhaseID{"1", "2", "3"} {
		if _, ok := progress[id]; !ok {
			t.Errorf("phase %s not loaded", id)
		}
	}
}

func TestEngine_TopoSortOrder(t *testing.T) {
	dir := t.TempDir()
	// Create 3 phases: 3→2→1 dependency chain.
	writeSpec(t, dir, "PHASE3.md", "# Phase 3: Third\n\n## Status\nPlanned\n\n## Depends On\nPhase 2\n\n## Exit Criteria\n- [ ] `true` exits 0\n")
	writeSpec(t, dir, "PHASE1.md", "# Phase 1: First\n\n## Status\nPlanned\n\n## Depends On\nNone\n\n## Exit Criteria\n- [ ] `true` exits 0\n")
	writeSpec(t, dir, "PHASE2.md", "# Phase 2: Second\n\n## Status\nPlanned\n\n## Depends On\nPhase 1\n\n## Exit Criteria\n- [ ] `true` exits 0\n")
	writePromptDoc(t, dir, "PHASE1-PROMPT.md", "# Phase 1 Implementation Prompts\n\n## Prompt 1 of 1: Do\n\n### Task\n\nDo.\n\n### Verification\n```bash\ntrue\n```\n")

	runner := phase.NewGateRunner(dir, 5*time.Second, slog.Default())
	engine, err := phase.NewEngine(dir, runner, nil, slog.Default())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadAll(); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	// Phase 1 should be current (no deps, first in topo order).
	current := engine.CurrentPhase()
	if current == nil {
		t.Fatal("expected current phase")
	}
	if current.ID != "1" {
		t.Errorf("CurrentPhase().ID = %q, want %q", current.ID, "1")
	}
}

func TestEngine_TopoSortCycle(t *testing.T) {
	// Create a temp dir with two specs that form a cycle.
	dir := t.TempDir()
	writeSpec(t, dir, "PHASEA.md", "# Phase A: Alpha\n\n## Status\nPlanned\n\n## Depends On\nPhase B\n\n## Exit Criteria\n- [ ] `true` exits 0\n")
	writeSpec(t, dir, "PHASEB.md", "# Phase B: Beta\n\n## Status\nPlanned\n\n## Depends On\nPhase A\n\n## Exit Criteria\n- [ ] `true` exits 0\n")

	runner := phase.NewGateRunner(dir, 5*time.Second, slog.Default())
	engine, err := phase.NewEngine(dir, runner, nil, slog.Default())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	err = engine.LoadAll()
	if err == nil {
		t.Fatal("expected cycle detection error")
	}
	if !contains(err.Error(), "cycle") {
		t.Errorf("error = %v, want cycle detection", err)
	}
}

func TestEngine_UnknownDependency(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "PHASEX.md", "# Phase X: Unknown Dep\n\n## Status\nPlanned\n\n## Depends On\nPhase Z\n\n## Exit Criteria\n- [ ] `true` exits 0\n")

	runner := phase.NewGateRunner(dir, 5*time.Second, slog.Default())
	engine, err := phase.NewEngine(dir, runner, nil, slog.Default())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	err = engine.LoadAll()
	if err == nil {
		t.Fatal("expected unknown dependency error")
	}
	if !contains(err.Error(), "unknown phase") {
		t.Errorf("error = %v, want unknown phase error", err)
	}
}

func TestEngine_CurrentPhase(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "PHASE1.md", "# Phase 1: First\n\n## Status\nPlanned\n\n## Depends On\nNone\n\n## Exit Criteria\n- [ ] `true` exits 0\n")
	writePromptDoc(t, dir, "PHASE1-PROMPT.md", "# Phase 1 Implementation Prompts\n\n## Prompt 1 of 1: Do thing\n\n### Task\n\nDo it.\n\n### Verification\n```bash\ntrue\n```\n")

	runner := phase.NewGateRunner(dir, 5*time.Second, slog.Default())
	engine, err := phase.NewEngine(dir, runner, nil, slog.Default())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadAll(); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	current := engine.CurrentPhase()
	if current == nil {
		t.Fatal("expected current phase, got nil")
	}
	if current.ID != "1" {
		t.Errorf("CurrentPhase().ID = %q, want %q", current.ID, "1")
	}
}

func TestEngine_AdvancePassing(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "PHASE1.md", "# Phase 1: First\n\n## Status\nPlanned\n\n## Depends On\nNone\n\n## Exit Criteria\n- [ ] `true` exits 0\n")
	writePromptDoc(t, dir, "PHASE1-PROMPT.md", "# Phase 1 Implementation Prompts\n\n## Prompt 1 of 2: Step A\n\n### Task\n\nDo A.\n\n### Verification\n```bash\ntrue\n```\n\n---\n\n## Prompt 2 of 2: Step B\n\n### Task\n\nDo B.\n\n### Verification\n```bash\ntrue\n```\n")

	runner := phase.NewGateRunner(dir, 5*time.Second, slog.Default())
	engine, err := phase.NewEngine(dir, runner, nil, slog.Default())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadAll(); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	// Advance prompt 1.
	if err := engine.Advance(context.Background()); err != nil {
		t.Fatalf("Advance prompt 1: %v", err)
	}

	progress := engine.Progress()
	prog := progress["1"]
	if prog.CompletedPrompts != 1 {
		t.Errorf("CompletedPrompts = %d, want 1", prog.CompletedPrompts)
	}

	// Current prompt should now be prompt 2.
	prompt := engine.CurrentPrompt()
	if prompt == nil {
		t.Fatal("expected current prompt after first advance")
	}
	if prompt.PromptNumber != 2 {
		t.Errorf("CurrentPrompt().PromptNumber = %d, want 2", prompt.PromptNumber)
	}
}

func TestEngine_AdvanceFailing(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "PHASE1.md", "# Phase 1: First\n\n## Status\nPlanned\n\n## Depends On\nNone\n\n## Exit Criteria\n- [ ] `true` exits 0\n")
	writePromptDoc(t, dir, "PHASE1-PROMPT.md", "# Phase 1 Implementation Prompts\n\n## Prompt 1 of 1: Fail\n\n### Task\n\nFail.\n\n### Verification\n```bash\nfalse\n```\n")

	runner := phase.NewGateRunner(dir, 5*time.Second, slog.Default())
	engine, err := phase.NewEngine(dir, runner, nil, slog.Default())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadAll(); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	err = engine.Advance(context.Background())
	if err == nil {
		t.Fatal("expected advance to fail")
	}
	if !errors.Is(err, types.ErrGateFailed) {
		t.Errorf("error = %v, want ErrGateFailed", err)
	}

	// Should not have advanced.
	progress := engine.Progress()
	if progress["1"].CompletedPrompts != 0 {
		t.Errorf("CompletedPrompts = %d, want 0 (should not advance on failure)", progress["1"].CompletedPrompts)
	}
}

func TestEngine_ForceAdvance(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "PHASE1.md", "# Phase 1: First\n\n## Status\nPlanned\n\n## Depends On\nNone\n\n## Exit Criteria\n- [ ] `true` exits 0\n")
	writePromptDoc(t, dir, "PHASE1-PROMPT.md", "# Phase 1 Implementation Prompts\n\n## Prompt 1 of 1: Skip\n\n### Task\n\nSkip.\n\n### Verification\n```bash\nfalse\n```\n")

	runner := phase.NewGateRunner(dir, 5*time.Second, slog.Default())
	engine, err := phase.NewEngine(dir, runner, nil, slog.Default())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadAll(); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	// ForceAdvance bypasses gates.
	if err := engine.ForceAdvance(context.Background(), "1"); err != nil {
		t.Fatalf("ForceAdvance: %v", err)
	}

	progress := engine.Progress()
	if progress["1"].CompletedPrompts != 1 {
		t.Errorf("CompletedPrompts = %d, want 1", progress["1"].CompletedPrompts)
	}
}

func TestEngine_ValidateParallelismNoConflict(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "PHASEA.md", "# Phase A: Alpha\n\n## Status\nPlanned\n\n## Depends On\nNone\n\n## Files\n\n### New\n- a.go\n\n## Exit Criteria\n- [ ] `true` exits 0\n")
	writeSpec(t, dir, "PHASEB.md", "# Phase B: Beta\n\n## Status\nPlanned\n\n## Depends On\nNone\n\n## Files\n\n### New\n- b.go\n\n## Exit Criteria\n- [ ] `true` exits 0\n")

	runner := phase.NewGateRunner(dir, 5*time.Second, slog.Default())
	engine, err := phase.NewEngine(dir, runner, nil, slog.Default())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadAll(); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	if err := engine.ValidateParallelism([]types.PhaseID{"A", "B"}); err != nil {
		t.Errorf("expected no conflict, got: %v", err)
	}
}

func TestEngine_ValidateParallelismConflict(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "PHASEA.md", "# Phase A: Alpha\n\n## Status\nPlanned\n\n## Depends On\nNone\n\n## Files\n\n### New\n- shared.go\n\n## Exit Criteria\n- [ ] `true` exits 0\n")
	writeSpec(t, dir, "PHASEB.md", "# Phase B: Beta\n\n## Status\nPlanned\n\n## Depends On\nNone\n\n## Files\n\n### New\n- shared.go\n\n## Exit Criteria\n- [ ] `true` exits 0\n")

	runner := phase.NewGateRunner(dir, 5*time.Second, slog.Default())
	engine, err := phase.NewEngine(dir, runner, nil, slog.Default())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadAll(); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	err = engine.ValidateParallelism([]types.PhaseID{"A", "B"})
	if err == nil {
		t.Fatal("expected conflict error")
	}
	if !contains(err.Error(), "shared.go") {
		t.Errorf("error = %v, want mention of shared.go", err)
	}
}

func TestEngine_DependencyOrdering(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "PHASE2.md", "# Phase 2: Second\n\n## Status\nPlanned\n\n## Depends On\nPhase 1\n\n## Exit Criteria\n- [ ] `true` exits 0\n")
	writeSpec(t, dir, "PHASE1.md", "# Phase 1: First\n\n## Status\nPlanned\n\n## Depends On\nNone\n\n## Exit Criteria\n- [ ] `true` exits 0\n")
	writePromptDoc(t, dir, "PHASE1-PROMPT.md", "# Phase 1 Implementation Prompts\n\n## Prompt 1 of 1: Do\n\n### Task\n\nDo.\n\n### Verification\n```bash\ntrue\n```\n")
	writePromptDoc(t, dir, "PHASE2-PROMPT.md", "# Phase 2 Implementation Prompts\n\n## Prompt 1 of 1: Do\n\n### Task\n\nDo.\n\n### Verification\n```bash\ntrue\n```\n")

	runner := phase.NewGateRunner(dir, 5*time.Second, slog.Default())
	engine, err := phase.NewEngine(dir, runner, nil, slog.Default())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadAll(); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	// Phase 1 should be current since Phase 2 depends on it.
	current := engine.CurrentPhase()
	if current == nil {
		t.Fatal("expected current phase")
	}
	if current.ID != "1" {
		t.Errorf("CurrentPhase().ID = %q, want %q (dependency ordering)", current.ID, "1")
	}

	// Phase 2 should not be reachable until Phase 1 is complete.
	if err := engine.Advance(context.Background()); err != nil {
		t.Fatalf("Advance phase 1: %v", err)
	}

	current = engine.CurrentPhase()
	if current == nil {
		t.Fatal("expected current phase after advancing")
	}
	if current.ID != "2" {
		t.Errorf("CurrentPhase().ID = %q, want %q", current.ID, "2")
	}
}

func writeSpec(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
}

func writePromptDoc(t *testing.T, dir, name, content string) {
	t.Helper()
	writeSpec(t, dir, name, content)
}
