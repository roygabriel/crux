package agent_test

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/roygabriel/crux/internal/agent"
	"github.com/roygabriel/crux/internal/tmux"
	"github.com/roygabriel/crux/pkg/types"
)

// messengerPlugin extends stubPlugin with configurable FormatMessage
// and DetectBusy behavior for messenger tests.
type messengerPlugin struct {
	stubPlugin
	formatResult string
	detectBusyFn func(string) bool
}

type dispatchGateStub struct {
	dispatchCalls int
	messageCalls  int
	dispatchErr   error
	messageErr    error
}

func (g *dispatchGateStub) GateDispatch(_ types.AgentID, _ types.Permission, _ string) error {
	g.dispatchCalls++
	return g.dispatchErr
}

func (g *dispatchGateStub) GateMessage(_ types.AgentID, _ types.Permission, _, _ string) error {
	g.messageCalls++
	return g.messageErr
}

type legacyMessageGateStub struct {
	calls int
	err   error
}

func (g *legacyMessageGateStub) GateMessage(_ types.AgentID, _ types.Permission, _, _ string) error {
	g.calls++
	return g.err
}

func (p *messengerPlugin) FormatMessage(_ types.Message) string {
	return p.formatResult
}

func (p *messengerPlugin) DetectBusy(content string) bool {
	if p.detectBusyFn != nil {
		return p.detectBusyFn(content)
	}
	return false
}

// spawnTestAgent creates a registry with a spawned agent using the given
// commander and plugin. Returns the registry, messenger, and any error.
func spawnTestAgent(cmd tmux.Commander, p *messengerPlugin) (*agent.Registry, *agent.Messenger, error) {
	logger := newTestLogger()
	sm := tmux.NewSessionManager(cmd, logger)
	pm := tmux.NewPaneManager(cmd, logger)
	plugins := newPluginRegistry(p)
	reg := agent.NewRegistry(sm, pm, plugins, logger)

	cfg := types.Agent{
		ID:         "agent-1",
		Name:       "test",
		Plugin:     "test-plugin",
		Role:       types.RoleEngineer,
		Permission: types.PermStandard,
		SessionID:  "test-session",
	}

	if err := reg.Spawn(context.Background(), cfg); err != nil {
		return nil, nil, err
	}

	m := agent.NewMessenger(pm, reg, logger)
	m.SetPollInterval(5 * time.Millisecond)
	return reg, m, nil
}

func TestMessengerSend(t *testing.T) {
	t.Parallel()

	var sentTexts []string
	cmd := &mockCommander{fn: func(_ context.Context, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "split-window" {
			return "%1", nil
		}
		// Literal sends have -l at args[1]; capture the text at args[5]
		// because "--" delimiter is inserted at args[4].
		if len(args) >= 6 && args[0] == "send-keys" && args[1] == "-l" {
			sentTexts = append(sentTexts, args[5])
		}
		return "", nil
	}}

	p := &messengerPlugin{
		stubPlugin:   stubPlugin{name: "test-plugin"},
		formatResult: "implement the auth module",
	}

	_, m, err := spawnTestAgent(cmd, p)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Clear sentTexts after spawn (which sends the launch command).
	sentTexts = nil

	msg := types.Message{
		ID:   "msg-1",
		Type: types.MessageTask,
	}

	if err := m.Send(context.Background(), "agent-1", msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if len(sentTexts) != 1 {
		t.Fatalf("expected 1 send-keys call, got %d", len(sentTexts))
	}
	if sentTexts[0] != "implement the auth module" {
		t.Errorf("sent text = %q, want %q", sentTexts[0], "implement the auth module")
	}
}

func TestMessengerSendChunkedMessage(t *testing.T) {
	t.Parallel()

	var sentTexts []string
	cmd := &mockCommander{fn: func(_ context.Context, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "split-window" {
			return "%1", nil
		}
		if len(args) >= 6 && args[0] == "send-keys" && args[1] == "-l" {
			sentTexts = append(sentTexts, args[5])
		}
		return "", nil
	}}

	p := &messengerPlugin{
		stubPlugin:   stubPlugin{name: "test-plugin"},
		formatResult: "line one\nline two\nline three",
	}

	_, m, err := spawnTestAgent(cmd, p)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	sentTexts = nil

	msg := types.Message{ID: "msg-2", Type: types.MessageTask}
	if err := m.Send(context.Background(), "agent-1", msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if len(sentTexts) != 1 {
		t.Fatalf("expected 1 chunk, got %d: %v", len(sentTexts), sentTexts)
	}
	if sentTexts[0] != "line one\nline two\nline three" {
		t.Errorf("chunk[0] = %q, want full multiline payload", sentTexts[0])
	}
}

func TestMessengerSendLongLineChunked(t *testing.T) {
	t.Parallel()

	var sentTexts []string
	cmd := &mockCommander{fn: func(_ context.Context, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "split-window" {
			return "%1", nil
		}
		if len(args) >= 6 && args[0] == "send-keys" && args[1] == "-l" {
			sentTexts = append(sentTexts, args[5])
		}
		return "", nil
	}}

	// Create a single line longer than MaxInputLength (4096).
	longLine := strings.Repeat("a", tmux.MaxInputLength+100)
	p := &messengerPlugin{
		stubPlugin:   stubPlugin{name: "test-plugin"},
		formatResult: longLine,
	}

	_, m, err := spawnTestAgent(cmd, p)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	sentTexts = nil

	msg := types.Message{ID: "msg-3", Type: types.MessageTask}
	if err := m.Send(context.Background(), "agent-1", msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if len(sentTexts) != 2 {
		t.Fatalf("expected 2 chunks for long line, got %d", len(sentTexts))
	}
	if len(sentTexts[0]) != tmux.MaxInputLength {
		t.Errorf("chunk[0] len = %d, want %d", len(sentTexts[0]), tmux.MaxInputLength)
	}
	if len(sentTexts[1]) != 100 {
		t.Errorf("chunk[1] len = %d, want 100", len(sentTexts[1]))
	}
}

func TestMessengerSendMarkdownWithCodeFences(t *testing.T) {
	t.Parallel()

	var sentTexts []string
	cmd := &mockCommander{fn: func(_ context.Context, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "split-window" {
			return "%1", nil
		}
		if len(args) >= 6 && args[0] == "send-keys" && args[1] == "-l" {
			sentTexts = append(sentTexts, args[5])
		}
		return "", nil
	}}

	p := &messengerPlugin{
		stubPlugin:   stubPlugin{name: "test-plugin"},
		formatResult: "```go\nfmt.Println(\"hello\")\n```\nRun: go build && go test",
	}

	_, m, err := spawnTestAgent(cmd, p)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	sentTexts = nil

	msg := types.Message{ID: "msg-md", Type: types.MessageTask}
	if err := m.Send(context.Background(), "agent-1", msg); err != nil {
		t.Fatalf("Send: expected no error for markdown content, got %v", err)
	}

	if len(sentTexts) != 1 {
		t.Fatalf("expected 1 chunk, got %d: %v", len(sentTexts), sentTexts)
	}
	if sentTexts[0] != "```go\nfmt.Println(\"hello\")\n```\nRun: go build && go test" {
		t.Errorf("chunk payload mismatch: %q", sentTexts[0])
	}
}

func TestMessengerSendEmptyFormat(t *testing.T) {
	t.Parallel()

	cmd := successCommander("%1")
	p := &messengerPlugin{
		stubPlugin:   stubPlugin{name: "test-plugin"},
		formatResult: "",
	}

	_, m, err := spawnTestAgent(cmd, p)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	msg := types.Message{ID: "msg-4", Type: types.MessageTask}
	if err := m.Send(context.Background(), "agent-1", msg); err != nil {
		t.Fatalf("Send: expected no error for empty message, got %v", err)
	}
}

func TestMessengerSendAgentNotFound(t *testing.T) {
	t.Parallel()

	logger := newTestLogger()
	cmd := successCommander("%1")
	sm := tmux.NewSessionManager(cmd, logger)
	pm := tmux.NewPaneManager(cmd, logger)
	reg := agent.NewRegistry(sm, pm, newPluginRegistry(nil), logger)
	m := agent.NewMessenger(pm, reg, logger)

	msg := types.Message{ID: "msg-5", Type: types.MessageTask}
	err := m.Send(context.Background(), "nonexistent", msg)
	if err == nil {
		t.Fatal("Send: expected error for nonexistent agent")
	}
	if !errors.Is(err, agent.ErrAgentNotFound) {
		t.Errorf("error = %v, want wrapping %v", err, agent.ErrAgentNotFound)
	}
}

func TestMessengerSendPaneError(t *testing.T) {
	t.Parallel()

	cmd := &mockCommander{fn: func(_ context.Context, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "split-window" {
			return "%1", nil
		}
		// Literal sends (from Messenger.Send) have -l flag; fail those.
		if len(args) > 1 && args[0] == "send-keys" && args[1] == "-l" {
			return "", errors.New("pane defunct")
		}
		return "", nil
	}}

	p := &messengerPlugin{
		stubPlugin:   stubPlugin{name: "test-plugin"},
		formatResult: "do the task",
	}

	_, m, err := spawnTestAgent(cmd, p)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	msg := types.Message{ID: "msg-6", Type: types.MessageTask}
	err = m.Send(context.Background(), "agent-1", msg)
	if err == nil {
		t.Fatal("Send: expected error on pane send failure")
	}
}

func TestMessengerSend_UsesTypedDispatchGateWhenAvailable(t *testing.T) {
	t.Parallel()

	cmd := successCommander("%1")
	p := &messengerPlugin{
		stubPlugin:   stubPlugin{name: "test-plugin"},
		formatResult: "do work",
	}
	_, m, err := spawnTestAgent(cmd, p)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	gate := &dispatchGateStub{}
	m.SetMessageGate(gate)

	msg := types.Message{ID: "msg-gate-1", Type: types.MessageTask}
	if err := m.Send(context.Background(), "agent-1", msg); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gate.dispatchCalls != 1 {
		t.Fatalf("dispatch calls = %d, want 1", gate.dispatchCalls)
	}
	if gate.messageCalls != 0 {
		t.Fatalf("message calls = %d, want 0", gate.messageCalls)
	}
}

func TestMessengerSend_FallsBackToLegacyMessageGate(t *testing.T) {
	t.Parallel()

	cmd := successCommander("%1")
	p := &messengerPlugin{
		stubPlugin:   stubPlugin{name: "test-plugin"},
		formatResult: "do work",
	}
	_, m, err := spawnTestAgent(cmd, p)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	gate := &legacyMessageGateStub{}
	m.SetMessageGate(gate)

	msg := types.Message{ID: "msg-gate-2", Type: types.MessageTask}
	if err := m.Send(context.Background(), "agent-1", msg); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gate.calls != 1 {
		t.Fatalf("legacy gate calls = %d, want 1", gate.calls)
	}
}

func TestMessengerWaitForResponse(t *testing.T) {
	t.Parallel()

	var captureCount atomic.Int32
	cmd := &mockCommander{fn: func(_ context.Context, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "split-window" {
			return "%1", nil
		}
		if len(args) > 0 && args[0] == "capture-pane" {
			n := captureCount.Add(1)
			if n < 3 {
				return "⠋ Working...\n", nil
			}
			return "Done\n>\n", nil
		}
		return "", nil
	}}

	p := &messengerPlugin{
		stubPlugin: stubPlugin{name: "test-plugin"},
		detectBusyFn: func(content string) bool {
			return strings.Contains(content, "⠋")
		},
	}

	_, m, err := spawnTestAgent(cmd, p)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	content, err := m.WaitForResponse(context.Background(), "agent-1", 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForResponse: %v", err)
	}
	if content != "Done\n>\n" {
		t.Errorf("content = %q, want %q", content, "Done\n>\n")
	}
}

func TestMessengerWaitForResponseImmediateReady(t *testing.T) {
	t.Parallel()

	cmd := &mockCommander{fn: func(_ context.Context, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "split-window" {
			return "%1", nil
		}
		if len(args) > 0 && args[0] == "capture-pane" {
			return "All done\n>\n", nil
		}
		return "", nil
	}}

	p := &messengerPlugin{
		stubPlugin:   stubPlugin{name: "test-plugin"},
		detectBusyFn: func(_ string) bool { return false },
	}

	_, m, err := spawnTestAgent(cmd, p)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	content, err := m.WaitForResponse(context.Background(), "agent-1", time.Second)
	if err != nil {
		t.Fatalf("WaitForResponse: %v", err)
	}
	if content != "All done\n>\n" {
		t.Errorf("content = %q, want %q", content, "All done\n>\n")
	}
}

func TestMessengerWaitForResponseTimeout(t *testing.T) {
	t.Parallel()

	cmd := &mockCommander{fn: func(_ context.Context, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "split-window" {
			return "%1", nil
		}
		if len(args) > 0 && args[0] == "capture-pane" {
			return "⠋ Still working...\n", nil
		}
		return "", nil
	}}

	p := &messengerPlugin{
		stubPlugin:   stubPlugin{name: "test-plugin"},
		detectBusyFn: func(_ string) bool { return true },
	}

	_, m, err := spawnTestAgent(cmd, p)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err = m.WaitForResponse(context.Background(), "agent-1", 50*time.Millisecond)
	if err == nil {
		t.Fatal("WaitForResponse: expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want wrapping %v", err, context.DeadlineExceeded)
	}
}

func TestMessengerWaitForResponseContextCanceled(t *testing.T) {
	t.Parallel()

	cmd := &mockCommander{fn: func(_ context.Context, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "split-window" {
			return "%1", nil
		}
		if len(args) > 0 && args[0] == "capture-pane" {
			return "⠋ busy\n", nil
		}
		return "", nil
	}}

	p := &messengerPlugin{
		stubPlugin:   stubPlugin{name: "test-plugin"},
		detectBusyFn: func(_ string) bool { return true },
	}

	_, m, err := spawnTestAgent(cmd, p)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after a short delay.
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	_, err = m.WaitForResponse(ctx, "agent-1", 10*time.Second)
	if err == nil {
		t.Fatal("WaitForResponse: expected cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want wrapping %v", err, context.Canceled)
	}
}

func TestMessengerWaitForResponseAgentNotFound(t *testing.T) {
	t.Parallel()

	logger := newTestLogger()
	cmd := successCommander("%1")
	sm := tmux.NewSessionManager(cmd, logger)
	pm := tmux.NewPaneManager(cmd, logger)
	reg := agent.NewRegistry(sm, pm, newPluginRegistry(nil), logger)
	m := agent.NewMessenger(pm, reg, logger)

	_, err := m.WaitForResponse(context.Background(), "nonexistent", time.Second)
	if err == nil {
		t.Fatal("WaitForResponse: expected error for nonexistent agent")
	}
	if !errors.Is(err, agent.ErrAgentNotFound) {
		t.Errorf("error = %v, want wrapping %v", err, agent.ErrAgentNotFound)
	}
}
