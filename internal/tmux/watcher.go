package tmux

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// CaptureLines is the default number of scrollback lines to capture per poll.
const CaptureLines = 500

// Watcher polls tmux panes at a configurable interval and invokes callbacks
// when the captured content changes. Each Watch call spawns a dedicated
// goroutine that runs until the watcher is stopped or the context is cancelled.
type Watcher struct {
	pm       *PaneManager
	interval time.Duration
	logger   *slog.Logger

	mu     sync.Mutex
	cancel context.CancelFunc
	ctx    context.Context
	wg     sync.WaitGroup
}

// NewWatcher creates a Watcher that polls panes using pm at the given interval.
func NewWatcher(pm *PaneManager, interval time.Duration, logger *slog.Logger) *Watcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &Watcher{
		pm:       pm,
		interval: interval,
		logger:   logger,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Watch starts a goroutine that polls the given pane and calls callback
// whenever the captured content differs from the previous poll. The goroutine
// stops when Stop is called or when ctx is cancelled. The caller-provided ctx
// is intersected with the watcher's internal context so either cancellation
// source will halt polling.
func (w *Watcher) Watch(ctx context.Context, paneID string, callback func(content string)) {
	// Merge the caller's context with the watcher's internal context so
	// either cancellation path stops the goroutine.
	mergedCtx, mergedCancel := context.WithCancel(ctx)

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		defer mergedCancel()

		var lastContent string
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-w.ctx.Done():
				return
			case <-mergedCtx.Done():
				return
			case <-ticker.C:
				content, err := w.pm.Capture(mergedCtx, paneID, CaptureLines)
				if err != nil {
					w.logger.Debug("capture failed during watch", "pane_id", paneID, "error", err)
					continue
				}

				if content != lastContent {
					lastContent = content
					callback(content)
				}
			}
		}
	}()
}

// Stop cancels all active watch goroutines and waits for them to exit.
func (w *Watcher) Stop() {
	w.mu.Lock()
	w.cancel()
	w.mu.Unlock()
	w.wg.Wait()
}
