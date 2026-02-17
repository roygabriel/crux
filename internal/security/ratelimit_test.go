package security

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/roygabriel/crux/pkg/types"
)

func TestRateLimiter_CommandUnderLimit(t *testing.T) {
	t.Parallel()
	rl := NewRateLimiter(10, 0, nil)

	for i := 0; i < 5; i++ {
		if err := rl.CheckCommand("agent-1"); err != nil {
			t.Fatalf("command %d: unexpected error: %v", i, err)
		}
		rl.RecordCommand("agent-1")
	}
}

func TestRateLimiter_CommandAtLimit(t *testing.T) {
	t.Parallel()
	rl := NewRateLimiter(3, 0, nil)

	for i := 0; i < 3; i++ {
		if err := rl.CheckCommand("agent-1"); err != nil {
			t.Fatalf("command %d: unexpected error: %v", i, err)
		}
		rl.RecordCommand("agent-1")
	}

	err := rl.CheckCommand("agent-1")
	if !errors.Is(err, types.ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
}

func TestRateLimiter_CommandWindowExpires(t *testing.T) {
	t.Parallel()
	rl := NewRateLimiter(2, 0, nil)

	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	rl.nowFunc = func() time.Time { return base }

	rl.RecordCommand("agent-1")
	rl.RecordCommand("agent-1")

	// At this point, limit is reached.
	if err := rl.CheckCommand("agent-1"); !errors.Is(err, types.ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}

	// Advance past the 60s window.
	rl.nowFunc = func() time.Time { return base.Add(61 * time.Second) }

	if err := rl.CheckCommand("agent-1"); err != nil {
		t.Errorf("expected nil after window expiry, got %v", err)
	}
}

func TestRateLimiter_CommandZeroDisabled(t *testing.T) {
	t.Parallel()
	rl := NewRateLimiter(0, 0, nil)

	for i := 0; i < 100; i++ {
		if err := rl.CheckCommand("agent-1"); err != nil {
			t.Fatalf("command %d: unexpected error: %v", i, err)
		}
		rl.RecordCommand("agent-1")
	}
}

func TestRateLimiter_FileUnderLimit(t *testing.T) {
	t.Parallel()
	rl := NewRateLimiter(0, 5, nil)

	for i := 0; i < 3; i++ {
		path := "file" + string(rune('a'+i)) + ".go"
		if err := rl.CheckFileModification("agent-1", path); err != nil {
			t.Fatalf("file %d: unexpected error: %v", i, err)
		}
		rl.RecordFileModification("agent-1", path)
	}
}

func TestRateLimiter_FileAtLimit(t *testing.T) {
	t.Parallel()
	rl := NewRateLimiter(0, 2, nil)

	rl.RecordFileModification("agent-1", "a.go")
	rl.RecordFileModification("agent-1", "b.go")

	err := rl.CheckFileModification("agent-1", "c.go")
	if !errors.Is(err, types.ErrFileLimit) {
		t.Errorf("expected ErrFileLimit, got %v", err)
	}
}

func TestRateLimiter_FileDuplicateNotCounted(t *testing.T) {
	t.Parallel()
	rl := NewRateLimiter(0, 2, nil)

	rl.RecordFileModification("agent-1", "a.go")
	rl.RecordFileModification("agent-1", "a.go")

	// Same file recorded twice — count should still be 1.
	if err := rl.CheckFileModification("agent-1", "b.go"); err != nil {
		t.Errorf("expected nil (duplicate not counted), got %v", err)
	}

	// Also re-checking the same file should pass.
	if err := rl.CheckFileModification("agent-1", "a.go"); err != nil {
		t.Errorf("expected nil for already-tracked file, got %v", err)
	}
}

func TestRateLimiter_FileZeroDisabled(t *testing.T) {
	t.Parallel()
	rl := NewRateLimiter(0, 0, nil)

	for i := 0; i < 100; i++ {
		path := "file" + string(rune('a'+i)) + ".go"
		if err := rl.CheckFileModification("agent-1", path); err != nil {
			t.Fatalf("file %d: unexpected error: %v", i, err)
		}
		rl.RecordFileModification("agent-1", path)
	}
}

func TestRateLimiter_ResetSession(t *testing.T) {
	t.Parallel()
	rl := NewRateLimiter(5, 5, nil)

	rl.RecordCommand("agent-1")
	rl.RecordCommand("agent-1")
	rl.RecordFileModification("agent-1", "a.go")

	cmds, files := rl.Stats("agent-1")
	if cmds != 2 || files != 1 {
		t.Fatalf("pre-reset: cmds=%d files=%d, want 2,1", cmds, files)
	}

	rl.ResetSession("agent-1")

	cmds, files = rl.Stats("agent-1")
	if cmds != 0 || files != 0 {
		t.Errorf("post-reset: cmds=%d files=%d, want 0,0", cmds, files)
	}
}

func TestRateLimiter_Stats(t *testing.T) {
	t.Parallel()
	rl := NewRateLimiter(10, 10, nil)

	rl.RecordCommand("agent-1")
	rl.RecordCommand("agent-1")
	rl.RecordCommand("agent-1")
	rl.RecordFileModification("agent-1", "a.go")
	rl.RecordFileModification("agent-1", "b.go")

	cmds, files := rl.Stats("agent-1")
	if cmds != 3 {
		t.Errorf("commands = %d, want 3", cmds)
	}
	if files != 2 {
		t.Errorf("files = %d, want 2", files)
	}
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	rl := NewRateLimiter(1000, 1000, nil)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			agent := types.AgentID("agent-1")
			for j := 0; j < 10; j++ {
				_ = rl.CheckCommand(agent)
				rl.RecordCommand(agent)
				_ = rl.CheckFileModification(agent, "file.go")
				rl.RecordFileModification(agent, "file.go")
			}
		}(i)
	}
	wg.Wait()
}

func TestRateLimiter_MultipleAgents(t *testing.T) {
	t.Parallel()
	rl := NewRateLimiter(2, 2, nil)

	rl.RecordCommand("agent-1")
	rl.RecordCommand("agent-1")
	rl.RecordFileModification("agent-1", "a.go")
	rl.RecordFileModification("agent-1", "b.go")

	// agent-1 is at limit.
	if err := rl.CheckCommand("agent-1"); !errors.Is(err, types.ErrRateLimited) {
		t.Errorf("agent-1 should be rate limited, got %v", err)
	}
	if err := rl.CheckFileModification("agent-1", "c.go"); !errors.Is(err, types.ErrFileLimit) {
		t.Errorf("agent-1 should be file limited, got %v", err)
	}

	// agent-2 should be independent and not limited.
	if err := rl.CheckCommand("agent-2"); err != nil {
		t.Errorf("agent-2 should not be limited: %v", err)
	}
	if err := rl.CheckFileModification("agent-2", "x.go"); err != nil {
		t.Errorf("agent-2 should not be file limited: %v", err)
	}
}
