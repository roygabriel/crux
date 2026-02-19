package orchestrator

import (
	"context"
	"time"

	"github.com/roygabriel/crux/pkg/types"
)

// Tick exposes tick() for testing.
func (o *Orchestrator) Tick(ctx context.Context) error {
	return o.tick(ctx)
}

// SetTestPaneContent injects pane content for testing.
func (o *Orchestrator) SetTestPaneContent(id types.AgentID, content string) {
	o.mu.Lock()
	o.paneContent[id] = content
	o.mu.Unlock()
}

// SetTestDispatchState simulates a recent dispatch for grace period testing.
func (o *Orchestrator) SetTestDispatchState(id types.AgentID, dispatchTime time.Time) {
	o.lastDispatchTime[id] = dispatchTime
	o.prevStatus[id] = types.StatusBusy
}

// SetDispatchGrace sets the dispatch grace period for testing.
func (o *Orchestrator) SetDispatchGrace(d time.Duration) {
	o.dispatchGrace = d
}
