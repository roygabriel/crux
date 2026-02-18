// Package tui implements a read-only terminal dashboard using bubbletea.
package tui

import (
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/roygabriel/crux/pkg/types"
)

// AgentSnapshot is a point-in-time view of a single agent for display.
type AgentSnapshot struct {
	ID             types.AgentID    `json:"id"`
	Name           string           `json:"name"`
	Plugin         string           `json:"plugin"`
	Role           types.AgentRole  `json:"role"`
	Status         types.AgentStatus `json:"status"`
	PromptDisplay  string           `json:"prompt_display"`
	Task           string           `json:"task"`
	CommandsPerMin int              `json:"commands_per_min"`
	FilesSession   int              `json:"files_session"`
	Permission     string           `json:"permission"`
	Decisions      []string         `json:"decisions"`
	WorkNotesInfo  string           `json:"work_notes_info"`
}

// StateUpdate is a complete snapshot of orchestration state for the TUI.
type StateUpdate struct {
	Phase        types.PhaseID   `json:"phase"`
	PhaseName    string          `json:"phase_name"`
	Agents       []AgentSnapshot `json:"agents"`
	Progress     string          `json:"progress"`
	GatesPassed  int             `json:"gates_passed"`
	GatesPending int             `json:"gates_pending"`
	Timestamp    time.Time       `json:"timestamp"`
}

// StateUpdateMsg wraps a StateUpdate as a bubbletea message.
type StateUpdateMsg struct {
	State StateUpdate
}

// StateBridge is a non-blocking channel bridge between the orchestrator and TUI.
// It uses a buffered channel with drain-and-replace semantics to avoid blocking
// the orchestrator if the TUI falls behind.
type StateBridge struct {
	mu sync.Mutex
	ch chan StateUpdate
}

// NewStateBridge creates a StateBridge with the given buffer size. A size less
// than 1 is clamped to 1.
func NewStateBridge(bufSize int) *StateBridge {
	if bufSize < 1 {
		bufSize = 1
	}
	return &StateBridge{
		ch: make(chan StateUpdate, bufSize),
	}
}

// Push sends an update to the bridge. If the buffer is full, the oldest entry
// is drained and the new one is enqueued. Push never blocks.
func (b *StateBridge) Push(update StateUpdate) {
	b.mu.Lock()
	defer b.mu.Unlock()

	select {
	case b.ch <- update:
	default:
		// Drain the oldest entry, then push the new one.
		select {
		case <-b.ch:
		default:
		}
		b.ch <- update
	}
}

// Subscribe returns a receive-only channel that delivers state updates.
func (b *StateBridge) Subscribe() <-chan StateUpdate {
	return b.ch
}

// WaitForUpdate returns a tea.Cmd that blocks on the bridge channel and
// delivers the next StateUpdate as a StateUpdateMsg.
func WaitForUpdate(bridge *StateBridge) tea.Cmd {
	if bridge == nil {
		return nil
	}
	return func() tea.Msg {
		update := <-bridge.Subscribe()
		return StateUpdateMsg{State: update}
	}
}
