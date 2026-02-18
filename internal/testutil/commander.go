// Package testutil provides reusable test doubles and a wiring harness for
// integration testing the crux orchestrator without real tmux processes.
package testutil

import (
	"context"
	"fmt"
	"sync"
)

// ResponseStep defines a single scripted response for a pane capture.
type ResponseStep struct {
	Content string
	Err     error
}

// CommandCall records the arguments of a single Commander.Run invocation.
type CommandCall struct {
	Args []string
}

// MockCommander is a thread-safe tmux.Commander that returns scripted
// responses for capture-pane calls and records all invocations.
type MockCommander struct {
	mu             sync.Mutex
	scripts        map[string][]ResponseStep // paneID → ordered steps
	idx            map[string]int            // current index per paneID
	defaultContent string
	calls          []CommandCall
}

// NewMockCommander creates a MockCommander that returns defaultContent
// when a pane's script is exhausted or unregistered.
func NewMockCommander(defaultContent string) *MockCommander {
	return &MockCommander{
		scripts:        make(map[string][]ResponseStep),
		idx:            make(map[string]int),
		defaultContent: defaultContent,
	}
}

// AddScript registers an ordered sequence of responses for a given paneID.
func (m *MockCommander) AddScript(paneID string, steps []ResponseStep) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scripts[paneID] = steps
	m.idx[paneID] = 0
}

// Run implements tmux.Commander. It dispatches based on the tmux subcommand:
//   - "capture-pane": returns the next scripted step for the target pane
//   - "split-window": returns "%1" (mock pane ID)
//   - all others: returns "", nil
func (m *MockCommander) Run(_ context.Context, args ...string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls = append(m.calls, CommandCall{Args: append([]string{}, args...)})

	if len(args) == 0 {
		return "", nil
	}

	switch args[0] {
	case "capture-pane":
		paneID := extractFlag(args, "-t")
		if paneID == "" {
			return m.defaultContent, nil
		}
		steps, ok := m.scripts[paneID]
		if !ok {
			return m.defaultContent, nil
		}
		i := m.idx[paneID]
		if i >= len(steps) {
			return m.defaultContent, nil
		}
		m.idx[paneID] = i + 1
		return steps[i].Content, steps[i].Err

	case "split-window":
		return "%1", nil

	default:
		return "", nil
	}
}

// Calls returns a copy of all recorded invocations.
func (m *MockCommander) Calls() []CommandCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]CommandCall, len(m.calls))
	copy(out, m.calls)
	return out
}

// SetDefaultContent changes the content returned when a pane's script
// is exhausted or unregistered.
func (m *MockCommander) SetDefaultContent(content string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultContent = content
}

// Reset clears all scripts, indices, and recorded calls.
func (m *MockCommander) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scripts = make(map[string][]ResponseStep)
	m.idx = make(map[string]int)
	m.calls = nil
}

// SetError configures the next capture for paneID to return an error.
func (m *MockCommander) SetError(paneID string, err error) {
	m.AddScript(paneID, []ResponseStep{{Err: err}})
}

// CallCount returns the total number of Run invocations.
func (m *MockCommander) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

// CallsForSubcommand returns recorded calls matching the given subcommand.
func (m *MockCommander) CallsForSubcommand(sub string) []CommandCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []CommandCall
	for _, c := range m.calls {
		if len(c.Args) > 0 && c.Args[0] == sub {
			out = append(out, c)
		}
	}
	return out
}

// extractFlag returns the value following flag in args, or "" if not found.
func extractFlag(args []string, flag string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag {
			return args[i+1]
		}
	}
	return ""
}

// Verify ensures scripts were fully consumed. Returns an error listing
// unconsumed panes.
func (m *MockCommander) Verify() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var unconsumed []string
	for paneID, steps := range m.scripts {
		i := m.idx[paneID]
		if i < len(steps) {
			unconsumed = append(unconsumed, fmt.Sprintf("%s: %d/%d consumed", paneID, i, len(steps)))
		}
	}
	if len(unconsumed) > 0 {
		return fmt.Errorf("unconsumed mock steps: %v", unconsumed)
	}
	return nil
}
