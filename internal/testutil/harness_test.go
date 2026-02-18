package testutil_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/roygabriel/crux/internal/testutil"
)

func TestNewTestHarness(t *testing.T) {
	h := testutil.NewTestHarness(t)

	// Verify directory structure exists.
	for _, sub := range []string{"phases", "sessions", "notes"} {
		path := filepath.Join(h.Dir, sub)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected dir %s to exist: %v", sub, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", sub)
		}
	}

	// Verify phase specs were written.
	for _, f := range []string{"PHASEA.md", "PHASEA-PROMPT.md", "PHASEB.md", "PHASEB-PROMPT.md"} {
		path := filepath.Join(h.Dir, "phases", f)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s to exist: %v", f, err)
		}
	}

	// Verify components are initialized.
	if h.Commander == nil {
		t.Error("Commander is nil")
	}
	if h.Plugin == nil {
		t.Error("Plugin is nil")
	}
	if h.Config == nil {
		t.Error("Config is nil")
	}
	if h.Logger == nil {
		t.Error("Logger is nil")
	}
}

func TestHarness_BuildOrchestrator(t *testing.T) {
	h := testutil.NewTestHarness(t)
	orch, tickCh := h.BuildOrchestrator()

	if orch == nil {
		t.Fatal("BuildOrchestrator returned nil orchestrator")
	}
	if tickCh == nil {
		t.Fatal("BuildOrchestrator returned nil tickCh")
	}
}

func TestWaitForNTicks_Timeout(t *testing.T) {
	ch := make(chan struct{})

	// Asking for 5 ticks on an empty channel should return 0 quickly.
	got := testutil.WaitForNTicks(ch, 5, 50*time.Millisecond)
	if got != 0 {
		t.Errorf("got %d ticks, want 0", got)
	}
}

func TestWaitForNTicks_Receives(t *testing.T) {
	ch := make(chan struct{}, 3)
	ch <- struct{}{}
	ch <- struct{}{}
	ch <- struct{}{}

	got := testutil.WaitForNTicks(ch, 3, time.Second)
	if got != 3 {
		t.Errorf("got %d ticks, want 3", got)
	}
}
