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
	if content != "" && o.firstContentAt[id].IsZero() {
		o.firstContentAt[id] = time.Now().UTC()
	}
	o.mu.Unlock()
}

// SetTestFirstContentAt sets the first non-empty capture timestamp for an agent.
func (o *Orchestrator) SetTestFirstContentAt(id types.AgentID, at time.Time) {
	o.mu.Lock()
	o.firstContentAt[id] = at
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

// SetReadyTimeout sets readiness timeout used by fallback dispatch gating.
func (o *Orchestrator) SetReadyTimeout(d time.Duration) {
	o.readyTimeout = d
}

// IsAgentReadyForDispatch exposes dispatch readiness checks for testing.
func (o *Orchestrator) IsAgentReadyForDispatch(id types.AgentID) bool {
	return o.isAgentReadyForDispatch(id)
}

// SaveSessionForTest exposes saveSession for tests.
func (o *Orchestrator) SaveSessionForTest() {
	o.saveSession()
}
