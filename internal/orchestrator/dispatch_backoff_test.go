package orchestrator

import (
	"log/slog"
	"testing"
	"time"

	"github.com/roygabriel/crux/internal/phase"
	"github.com/roygabriel/crux/pkg/types"
)

func TestPromptBackoffActivatesAfterThreshold(t *testing.T) {
	o := &Orchestrator{logger: slog.Default()}
	key := makePromptKey("2B", 2)

	o.notePromptFailure(key.phaseID, key.promptNum, "first")
	if _, cooling := o.promptCooldown(key); cooling {
		t.Fatal("cooldown activated before threshold")
	}

	o.notePromptFailure(key.phaseID, key.promptNum, "second")
	until, cooling := o.promptCooldown(key)
	if !cooling {
		t.Fatal("expected cooldown to activate at threshold")
	}
	if !until.After(time.Now()) {
		t.Fatalf("cooldown_until = %v, want future time", until)
	}
	if got := o.promptFailCount[key]; got != 2 {
		t.Fatalf("failure count = %d, want 2", got)
	}
}

func TestClearPromptFailureResetsState(t *testing.T) {
	o := &Orchestrator{logger: slog.Default()}
	key := makePromptKey("1A", 1)
	o.notePromptFailure(key.phaseID, key.promptNum, "first")
	o.notePromptFailure(key.phaseID, key.promptNum, "second")

	o.clearPromptFailure(key.phaseID, key.promptNum)

	if _, ok := o.promptFailCount[key]; ok {
		t.Fatal("promptFailCount key should be cleared")
	}
	if _, ok := o.promptCooldownUntil[key]; ok {
		t.Fatal("promptCooldownUntil key should be cleared")
	}
	if _, ok := o.promptCooldownLogAt[key]; ok {
		t.Fatal("promptCooldownLogAt key should be cleared")
	}
}

func TestBumpDispatchRepeat(t *testing.T) {
	o := &Orchestrator{logger: slog.Default()}
	fp := dispatchFingerprint{phaseID: "1A", promptNum: 1, promptHash: "a", filesHash: "f"}
	fp2 := dispatchFingerprint{phaseID: "1A", promptNum: 2, promptHash: "b", filesHash: "f2"}

	if got := o.bumpDispatchRepeat("engineer-1", fp, "pane-a"); got != 1 {
		t.Fatalf("repeat count #1 = %d, want 1", got)
	}
	if got := o.bumpDispatchRepeat("engineer-1", fp, "pane-a"); got != 2 {
		t.Fatalf("repeat count #2 = %d, want 2", got)
	}
	if got := o.bumpDispatchRepeat("engineer-1", fp2, "pane-b"); got != 1 {
		t.Fatalf("repeat count after fingerprint change = %d, want 1", got)
	}
}

func TestBumpDispatchRepeat_ResetsOnPaneProgress(t *testing.T) {
	o := &Orchestrator{logger: slog.Default()}
	fp := dispatchFingerprint{phaseID: "1A", promptNum: 1, promptHash: "a", filesHash: "f"}

	if got := o.bumpDispatchRepeat("engineer-1", fp, "pane-a"); got != 1 {
		t.Fatalf("repeat count #1 = %d, want 1", got)
	}
	if got := o.bumpDispatchRepeat("engineer-1", fp, "pane-b"); got != 1 {
		t.Fatalf("repeat count after pane progress = %d, want 1", got)
	}
}

func TestBuildDispatchFingerprintStableFileOrder(t *testing.T) {
	specA := &phase.PhaseSpec{
		ID:            "2A",
		FilesNew:      []string{"b.go", "a.go"},
		FilesModified: []string{"c.go"},
	}
	specB := &phase.PhaseSpec{
		ID:            "2A",
		FilesNew:      []string{"a.go", "b.go"},
		FilesModified: []string{"c.go"},
	}
	prompt := &phase.PromptContract{
		PhaseID:      types.PhaseID("2A"),
		PromptNumber: 3,
	}

	a := buildDispatchFingerprint(specA, prompt, "rendered")
	b := buildDispatchFingerprint(specB, prompt, "rendered")
	if a != b {
		t.Fatalf("fingerprints differ: %#v != %#v", a, b)
	}
}

func TestPromptCooldownExpiryClearsFailureCount(t *testing.T) {
	o := &Orchestrator{logger: slog.Default()}
	key := makePromptKey("3A", 4)
	o.ensureDispatchMaps()
	o.promptFailCount[key] = 9
	o.promptCooldownUntil[key] = time.Now().Add(-time.Second)
	o.promptCooldownLogAt[key] = time.Now().Add(-time.Second)

	if _, cooling := o.promptCooldown(key); cooling {
		t.Fatal("promptCooldown should have expired")
	}
	if _, ok := o.promptFailCount[key]; ok {
		t.Fatal("failure count should be cleared after cooldown expiry")
	}
}
