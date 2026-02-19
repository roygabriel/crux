package phase_test

import (
	"testing"
	"time"

	"github.com/roygabriel/crux/internal/phase"
)

func TestEngineSetPosition_MarksPriorPhasesComplete(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "PHASE1.md", "# Phase 1: First\n\n## Status\nPlanned\n\n## Depends On\nNone\n\n## Exit Criteria\n- [ ] `true` exits 0\n")
	writePromptDoc(t, dir, "PHASE1-PROMPT.md", "# Phase 1 Prompts\n\n## Prompt 1 of 1: One\n\n### Task\n\nDo.\n\n### Verification\n```bash\ntrue\n```\n")
	writeSpec(t, dir, "PHASE2.md", "# Phase 2: Second\n\n## Status\nPlanned\n\n## Depends On\nPhase 1\n\n## Exit Criteria\n- [ ] `true` exits 0\n")
	writePromptDoc(t, dir, "PHASE2-PROMPT.md", "# Phase 2 Prompts\n\n## Prompt 1 of 1: Two\n\n### Task\n\nDo.\n\n### Verification\n```bash\ntrue\n```\n")

	runner := phase.NewGateRunner(dir, 5*time.Second, nil)
	engine, err := phase.NewEngine(dir, runner, nil, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadAll(); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	if err := engine.SetPosition("2", 1); err != nil {
		t.Fatalf("SetPosition: %v", err)
	}
	progress := engine.Progress()
	if progress["1"].CompletedPrompts != 1 {
		t.Fatalf("phase 1 completed = %d, want 1", progress["1"].CompletedPrompts)
	}
	if progress["2"].CompletedPrompts != 0 {
		t.Fatalf("phase 2 completed = %d, want 0", progress["2"].CompletedPrompts)
	}
}

func TestEngineSetPosition_AllComplete(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "PHASE1.md", "# Phase 1: First\n\n## Status\nPlanned\n\n## Depends On\nNone\n\n## Exit Criteria\n- [ ] `true` exits 0\n")
	writePromptDoc(t, dir, "PHASE1-PROMPT.md", "# Phase 1 Prompts\n\n## Prompt 1 of 1: One\n\n### Task\n\nDo.\n\n### Verification\n```bash\ntrue\n```\n")

	runner := phase.NewGateRunner(dir, 5*time.Second, nil)
	engine, err := phase.NewEngine(dir, runner, nil, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadAll(); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	if err := engine.SetPosition("1", 0); err != nil {
		t.Fatalf("SetPosition: %v", err)
	}
	if got := engine.CurrentPhase(); got != nil {
		t.Fatalf("CurrentPhase = %v, want nil when all complete", got.ID)
	}
}
