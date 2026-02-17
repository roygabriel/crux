package tmux

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// testInterval is a short polling interval for tests.
const testInterval = 10 * time.Millisecond

func TestWatcherCallsCallbackOnChange(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32
	var mu sync.Mutex
	responses := []string{"content-1", "content-2", "content-3"}
	idx := 0

	mock := &mockCommander{
		runFunc: func(_ context.Context, _ ...string) (string, error) {
			mu.Lock()
			defer mu.Unlock()
			if idx >= len(responses) {
				return responses[len(responses)-1], nil
			}
			r := responses[idx]
			idx++
			return r, nil
		},
	}

	pm := NewPaneManager(mock, newTestLogger())
	w := NewWatcher(pm, testInterval, newTestLogger())

	var received []string
	var receivedMu sync.Mutex

	w.Watch(context.Background(), "%0", func(content string) {
		callCount.Add(1)
		receivedMu.Lock()
		received = append(received, content)
		receivedMu.Unlock()
	})

	// Wait for enough polls to see all three distinct values.
	time.Sleep(100 * time.Millisecond)
	w.Stop()

	count := callCount.Load()
	if count < 3 {
		t.Errorf("expected at least 3 callbacks, got %d", count)
	}

	receivedMu.Lock()
	defer receivedMu.Unlock()
	if len(received) < 3 {
		t.Fatalf("expected at least 3 received values, got %d: %v", len(received), received)
	}
	if received[0] != "content-1" || received[1] != "content-2" || received[2] != "content-3" {
		t.Errorf("unexpected received order: %v", received[:3])
	}
}

func TestWatcherDeduplicatesContent(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	mock := &mockCommander{
		runFunc: func(_ context.Context, _ ...string) (string, error) {
			return "same-content", nil
		},
	}

	pm := NewPaneManager(mock, newTestLogger())
	w := NewWatcher(pm, testInterval, newTestLogger())

	w.Watch(context.Background(), "%0", func(_ string) {
		callCount.Add(1)
	})

	// Let several polls run.
	time.Sleep(100 * time.Millisecond)
	w.Stop()

	count := callCount.Load()
	if count != 1 {
		t.Errorf("expected exactly 1 callback for identical content, got %d", count)
	}
}

func TestWatcherStopCancelsGoroutine(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	mock := &mockCommander{
		runFunc: func(_ context.Context, _ ...string) (string, error) {
			return "content", nil
		},
	}

	pm := NewPaneManager(mock, newTestLogger())
	w := NewWatcher(pm, testInterval, newTestLogger())

	w.Watch(context.Background(), "%0", func(_ string) {
		callCount.Add(1)
	})

	// Let at least one poll fire.
	time.Sleep(50 * time.Millisecond)
	w.Stop()

	countAtStop := callCount.Load()

	// Wait to verify no more callbacks fire after Stop.
	time.Sleep(50 * time.Millisecond)
	countAfterWait := callCount.Load()

	if countAtStop != countAfterWait {
		t.Errorf("callbacks continued after Stop: %d at stop, %d after wait", countAtStop, countAfterWait)
	}
}

func TestWatcherContextCancellationStops(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	mock := &mockCommander{
		runFunc: func(_ context.Context, _ ...string) (string, error) {
			return "content", nil
		},
	}

	pm := NewPaneManager(mock, newTestLogger())
	w := NewWatcher(pm, testInterval, newTestLogger())

	ctx, cancel := context.WithCancel(context.Background())

	w.Watch(ctx, "%0", func(_ string) {
		callCount.Add(1)
	})

	// Let at least one poll fire.
	time.Sleep(50 * time.Millisecond)
	cancel()

	countAtCancel := callCount.Load()

	// Wait to verify no more callbacks fire after context cancellation.
	time.Sleep(50 * time.Millisecond)
	countAfterWait := callCount.Load()

	if countAtCancel != countAfterWait {
		t.Errorf("callbacks continued after context cancel: %d at cancel, %d after wait", countAtCancel, countAfterWait)
	}

	// Clean up — Stop should return immediately since goroutine already exited.
	w.Stop()
}

func TestWatcherMultiplePanes(t *testing.T) {
	t.Parallel()

	var count1, count2 atomic.Int32

	mock := &mockCommander{
		runFunc: func(_ context.Context, args ...string) (string, error) {
			// Return different content per pane based on -t arg.
			for i, a := range args {
				if a == "-t" && i+1 < len(args) {
					return "content-" + args[i+1], nil
				}
			}
			return "", nil
		},
	}

	pm := NewPaneManager(mock, newTestLogger())
	w := NewWatcher(pm, testInterval, newTestLogger())

	w.Watch(context.Background(), "%0", func(_ string) {
		count1.Add(1)
	})
	w.Watch(context.Background(), "%1", func(_ string) {
		count2.Add(1)
	})

	time.Sleep(50 * time.Millisecond)
	w.Stop()

	if count1.Load() < 1 {
		t.Error("expected at least 1 callback for pane %0")
	}
	if count2.Load() < 1 {
		t.Error("expected at least 1 callback for pane %1")
	}
}
