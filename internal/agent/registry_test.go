package agent_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/roygabriel/crux/internal/agent"
	"github.com/roygabriel/crux/internal/plugin"
	"github.com/roygabriel/crux/internal/tmux"
	"github.com/roygabriel/crux/pkg/types"
)

// mockCommander implements tmux.Commander for testing, dispatching
// calls to a configurable function.
type mockCommander struct {
	fn func(ctx context.Context, args ...string) (string, error)
}

func (m *mockCommander) Run(ctx context.Context, args ...string) (string, error) {
	return m.fn(ctx, args...)
}

// stubPlugin implements plugin.AgentPlugin with minimal behavior.
type stubPlugin struct {
	name string
}

func (s *stubPlugin) Name() string { return s.name }

func (s *stubPlugin) LaunchCmd(cfg plugin.AgentConfig) (string, []string, error) {
	return "stubcli", []string{"--agent", string(cfg.ID)}, nil
}

func (s *stubPlugin) DetectReady(_ string) bool                          { return false }
func (s *stubPlugin) DetectBusy(_ string) bool                           { return false }
func (s *stubPlugin) DetectError(_ string) (string, bool)                { return "", false }
func (s *stubPlugin) DetectRateLimit(_ string) (time.Duration, bool)     { return 0, false }
func (s *stubPlugin) FormatMessage(_ types.Message) string               { return "" }
func (s *stubPlugin) ParseOutput(_ string) (plugin.AgentOutput, error)   { return plugin.AgentOutput{}, nil }
func (s *stubPlugin) Capabilities() []plugin.Capability                  { return nil }

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// successCommander returns a Commander that succeeds for all commands,
// returning paneID for split-window and empty string for everything else.
func successCommander(paneID string) *mockCommander {
	return &mockCommander{fn: func(_ context.Context, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "split-window" {
			return paneID, nil
		}
		return "", nil
	}}
}

// newPluginRegistry creates a plugin.Registry with the given plugin
// registered. If p is nil, the registry is empty.
func newPluginRegistry(p plugin.AgentPlugin) *plugin.Registry {
	plugins := plugin.NewRegistry()
	if p != nil {
		_ = plugins.Register(p.Name(), func() plugin.AgentPlugin { return p })
	}
	return plugins
}

func newTestRegistry(cmd tmux.Commander) *agent.Registry {
	logger := newTestLogger()
	sm := tmux.NewSessionManager(cmd, logger)
	pm := tmux.NewPaneManager(cmd, logger)
	plugins := newPluginRegistry(&stubPlugin{name: "claude"})
	return agent.NewRegistry(sm, pm, plugins, logger)
}

func validAgent(id string) types.Agent {
	return types.Agent{
		ID:         types.AgentID(id),
		Name:       "test-agent",
		Plugin:     "claude",
		Role:       types.RoleEngineer,
		Permission: types.PermStandard,
		SessionID:  "test-session",
	}
}

func TestRegistrySpawnAndGet(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry(successCommander("%1"))
	ctx := context.Background()

	if err := reg.Spawn(ctx, validAgent("agent-1")); err != nil {
		t.Fatalf("Spawn: %v", err)
	}

	inst, err := reg.Get("agent-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if inst.Agent.ID != "agent-1" {
		t.Errorf("ID = %q, want %q", inst.Agent.ID, "agent-1")
	}
	if inst.Agent.Status != types.StatusIdle {
		t.Errorf("Status = %q, want %q", inst.Agent.Status, types.StatusIdle)
	}
	if inst.Agent.PaneID != "%1" {
		t.Errorf("PaneID = %q, want %%1", inst.Agent.PaneID)
	}
	if inst.Plugin == nil {
		t.Error("Plugin is nil")
	}
	if inst.LaunchedAt.IsZero() {
		t.Error("LaunchedAt is zero")
	}
}

func TestRegistrySpawnDuplicate(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry(successCommander("%1"))
	ctx := context.Background()

	if err := reg.Spawn(ctx, validAgent("agent-1")); err != nil {
		t.Fatalf("first Spawn: %v", err)
	}

	err := reg.Spawn(ctx, validAgent("agent-1"))
	if err == nil {
		t.Fatal("second Spawn: expected error, got nil")
	}
	if !errors.Is(err, agent.ErrAgentAlreadyExists) {
		t.Errorf("error = %v, want wrapping %v", err, agent.ErrAgentAlreadyExists)
	}
}

func TestRegistrySpawnUnknownPlugin(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry(successCommander("%1"))
	ctx := context.Background()

	cfg := validAgent("agent-1")
	cfg.Plugin = "nonexistent"

	err := reg.Spawn(ctx, cfg)
	if err == nil {
		t.Fatal("Spawn: expected error for unknown plugin, got nil")
	}
	if !errors.Is(err, plugin.ErrPluginNotFound) {
		t.Errorf("error = %v, want wrapping %v", err, plugin.ErrPluginNotFound)
	}
}

func TestRegistrySpawnValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  types.Agent
	}{
		{
			name: "empty-id",
			cfg:  types.Agent{Plugin: "claude", SessionID: "test-session"},
		},
		{
			name: "empty-session-id",
			cfg:  types.Agent{ID: "agent-1", Plugin: "claude"},
		},
		{
			name: "empty-plugin",
			cfg:  types.Agent{ID: "agent-1", SessionID: "test-session"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reg := newTestRegistry(successCommander("%1"))
			err := reg.Spawn(context.Background(), tt.cfg)
			if err == nil {
				t.Error("Spawn: expected validation error, got nil")
			}
		})
	}
}

func TestRegistrySpawnPaneCreateFailure(t *testing.T) {
	t.Parallel()

	cmd := &mockCommander{fn: func(_ context.Context, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "split-window" {
			return "", errors.New("no room for pane")
		}
		return "", nil
	}}

	reg := newTestRegistry(cmd)
	err := reg.Spawn(context.Background(), validAgent("agent-1"))
	if err == nil {
		t.Fatal("Spawn: expected error on pane create failure, got nil")
	}

	// Agent should not be registered.
	_, getErr := reg.Get("agent-1")
	if !errors.Is(getErr, agent.ErrAgentNotFound) {
		t.Errorf("agent should not be registered after pane create failure, Get error = %v", getErr)
	}
}

func TestRegistrySpawnSendKeysFailure(t *testing.T) {
	t.Parallel()

	var killCalled bool
	cmd := &mockCommander{fn: func(_ context.Context, args ...string) (string, error) {
		if len(args) == 0 {
			return "", nil
		}
		switch args[0] {
		case "split-window":
			return "%1", nil
		case "send-keys":
			return "", errors.New("pane defunct")
		case "kill-pane":
			killCalled = true
			return "", nil
		}
		return "", nil
	}}

	reg := newTestRegistry(cmd)
	err := reg.Spawn(context.Background(), validAgent("agent-1"))
	if err == nil {
		t.Fatal("Spawn: expected error on send-keys failure, got nil")
	}
	if !killCalled {
		t.Error("expected pane cleanup (kill-pane) after send-keys failure")
	}

	// Agent should not be registered.
	_, getErr := reg.Get("agent-1")
	if !errors.Is(getErr, agent.ErrAgentNotFound) {
		t.Errorf("agent should not be registered after send-keys failure, Get error = %v", getErr)
	}
}

func TestRegistryGetNotFound(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry(successCommander("%1"))
	_, err := reg.Get("nonexistent")
	if err == nil {
		t.Fatal("Get: expected error, got nil")
	}
	if !errors.Is(err, agent.ErrAgentNotFound) {
		t.Errorf("error = %v, want wrapping %v", err, agent.ErrAgentNotFound)
	}
}

func TestRegistryList(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry(successCommander("%1"))
	ctx := context.Background()

	// Empty list.
	list := reg.List()
	if list == nil {
		t.Fatal("List() returned nil, want non-nil empty slice")
	}
	if len(list) != 0 {
		t.Errorf("List() returned %d items, want 0", len(list))
	}

	// Spawn two agents.
	if err := reg.Spawn(ctx, validAgent("agent-1")); err != nil {
		t.Fatalf("Spawn agent-1: %v", err)
	}
	if err := reg.Spawn(ctx, validAgent("agent-2")); err != nil {
		t.Fatalf("Spawn agent-2: %v", err)
	}

	list = reg.List()
	if len(list) != 2 {
		t.Errorf("List() returned %d items, want 2", len(list))
	}
}

func TestRegistryKill(t *testing.T) {
	t.Parallel()

	var sendKeysCalled, killPaneCalled bool
	cmd := &mockCommander{fn: func(_ context.Context, args ...string) (string, error) {
		if len(args) == 0 {
			return "", nil
		}
		switch args[0] {
		case "split-window":
			return "%1", nil
		case "send-keys":
			sendKeysCalled = true
			return "", nil
		case "kill-pane":
			killPaneCalled = true
			return "", nil
		}
		return "", nil
	}}

	reg := newTestRegistry(cmd)
	ctx := context.Background()

	if err := reg.Spawn(ctx, validAgent("agent-1")); err != nil {
		t.Fatalf("Spawn: %v", err)
	}

	// Reset flags after spawn (which also calls send-keys).
	sendKeysCalled = false
	killPaneCalled = false

	if err := reg.Kill(ctx, "agent-1"); err != nil {
		t.Fatalf("Kill: %v", err)
	}

	if !sendKeysCalled {
		t.Error("Kill did not send exit command")
	}
	if !killPaneCalled {
		t.Error("Kill did not destroy pane")
	}

	// Agent should no longer be registered.
	_, err := reg.Get("agent-1")
	if !errors.Is(err, agent.ErrAgentNotFound) {
		t.Errorf("agent should be removed after kill, Get error = %v", err)
	}
}

func TestRegistryKillNotFound(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry(successCommander("%1"))
	err := reg.Kill(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("Kill: expected error, got nil")
	}
	if !errors.Is(err, agent.ErrAgentNotFound) {
		t.Errorf("error = %v, want wrapping %v", err, agent.ErrAgentNotFound)
	}
}

func TestRegistryUpdateStatus(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry(successCommander("%1"))
	ctx := context.Background()

	if err := reg.Spawn(ctx, validAgent("agent-1")); err != nil {
		t.Fatalf("Spawn: %v", err)
	}

	if err := reg.UpdateStatus("agent-1", types.StatusBusy); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	inst, err := reg.Get("agent-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if inst.Agent.Status != types.StatusBusy {
		t.Errorf("Status = %q, want %q", inst.Agent.Status, types.StatusBusy)
	}
}

func TestRegistryUpdateStatusNotFound(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry(successCommander("%1"))
	err := reg.UpdateStatus("nonexistent", types.StatusBusy)
	if err == nil {
		t.Fatal("UpdateStatus: expected error, got nil")
	}
	if !errors.Is(err, agent.ErrAgentNotFound) {
		t.Errorf("error = %v, want wrapping %v", err, agent.ErrAgentNotFound)
	}
}

func TestRegistryConcurrentAccess(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry(successCommander("%1"))
	ctx := context.Background()
	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(n int) {
			defer wg.Done()

			id := fmt.Sprintf("agent-%d", n)
			cfg := validAgent(id)

			// Spawn may fail for races — that's expected.
			_ = reg.Spawn(ctx, cfg)
			_, _ = reg.Get(types.AgentID(id))
			_ = reg.List()
			_ = reg.UpdateStatus(types.AgentID(id), types.StatusBusy)
			_ = reg.Kill(ctx, types.AgentID(id))
		}(i)
	}

	wg.Wait()

	// Registry should be empty after all kills.
	list := reg.List()
	if len(list) != 0 {
		t.Errorf("List() returned %d items after concurrent kill-all, want 0", len(list))
	}
}
