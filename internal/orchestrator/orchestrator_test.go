package orchestrator_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/roygabriel/crux/internal/agent"
	"github.com/roygabriel/crux/internal/config"
	"github.com/roygabriel/crux/internal/memory/journal"
	"github.com/roygabriel/crux/internal/memory/session"
	"github.com/roygabriel/crux/internal/memory/worknotes"
	"github.com/roygabriel/crux/internal/orchestrator"
	"github.com/roygabriel/crux/internal/phase"
	"github.com/roygabriel/crux/internal/plugin"
	"github.com/roygabriel/crux/internal/tmux"
	"github.com/roygabriel/crux/pkg/types"
)

// --- Fake tmux Commander ---

type fakeCommander struct{}

func (f *fakeCommander) Run(_ context.Context, args ...string) (string, error) {
	if len(args) > 0 {
		switch args[0] {
		case "split-window":
			return "%1", nil
		case "capture-pane":
			return "", nil
		case "has-session":
			return "", nil
		case "new-session":
			return "", nil
		}
	}
	return "", nil
}

// --- Test helpers ---

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Create a minimal phase spec.
	specDir := filepath.Join(dir, "phases")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	specContent := `# Phase 1A: Foundation

## Status

planned

## Depends On

None

## Exit Criteria

- [ ] ` + "`go build ./...`" + ` exit 0
`
	if err := os.WriteFile(filepath.Join(specDir, "PHASE1A.md"), []byte(specContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create sessions dir.
	sessDir := filepath.Join(dir, "sessions")
	if err := os.MkdirAll(sessDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create notes dir.
	notesDir := filepath.Join(dir, "notes")
	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	return dir
}

func buildTestOrchestrator(t *testing.T, dir string) *orchestrator.Orchestrator {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cmd := &fakeCommander{}
	sm := tmux.NewSessionManager(cmd, logger)
	pm := tmux.NewPaneManager(cmd, logger)

	// Plugin registry with a mock plugin.
	pluginReg := plugin.NewRegistry()
	_ = pluginReg.Register("claude", func() plugin.AgentPlugin {
		return &mockPlugin{
			name: "claude",
			caps: []plugin.Capability{plugin.CapCodeGen, plugin.CapShellExec},
		}
	})

	registry := agent.NewRegistry(sm, pm, pluginReg, logger)
	watcher := tmux.NewWatcher(pm, 100*time.Millisecond, logger)

	specDir := filepath.Join(dir, "phases")
	gateRunner := phase.NewGateRunner(dir, 10*time.Second, logger)
	engine, err := phase.NewEngine(specDir, gateRunner, nil, logger)
	if err != nil {
		t.Fatal(err)
	}

	completion := phase.NewCompletionHandler(engine, gateRunner, &noopDecisionRecorder{}, &noopWorkNotes{}, logger)
	contextBld := phase.NewContextBuilder(&noopDecisionSearcher{}, &noopWorkNotes{}, &noopBankSummarizer{}, logger)
	tracker := phase.NewTracker(engine, logger)

	sessDir := filepath.Join(dir, "sessions")
	sessionMgr := session.NewManager(sessDir, nil, logger)

	notesDir := filepath.Join(dir, "notes")
	notesMgr := worknotes.NewManager(notesDir, logger)

	messenger := agent.NewMessenger(pm, registry, logger)

	cfg := &config.Config{
		Project: config.ProjectConfig{
			Name:     "test",
			Root:     dir,
			StateDir: dir,
		},
		Agents: map[string]config.AgentConfig{
			"claude-1": {Plugin: "claude", Role: "engineer", Permission: "standard"},
		},
	}

	return orchestrator.New(
		cfg, registry, engine, completion, contextBld, tracker,
		watcher, messenger, sessionMgr, notesMgr, nil, logger,
	)
}

// --- Noop mocks for phase dependencies ---

type noopDecisionRecorder struct{}

func (n *noopDecisionRecorder) Record(_ context.Context, _ types.Decision) error { return nil }

type noopDecisionSearcher struct{}

func (n *noopDecisionSearcher) SemanticSearch(_ context.Context, _ string, _ int) ([]types.Decision, error) {
	return nil, nil
}

type noopWorkNotes struct{}

func (n *noopWorkNotes) Read(_ string) (*worknotes.WorkNotes, error) {
	return &worknotes.WorkNotes{}, nil
}
func (n *noopWorkNotes) Init(_, _ string) error                                    { return nil }
func (n *noopWorkNotes) AppendDecision(_, _, _ string) error                       { return nil }
func (n *noopWorkNotes) AppendSession(_ string, _ worknotes.SessionLogEntry) error { return nil }
func (n *noopWorkNotes) UpdatePromptProgress(_ string, _ int, _ bool) error        { return nil }
func (n *noopWorkNotes) UpdateStatus(_, _ string) error                            { return nil }
func (n *noopWorkNotes) Render(_ *worknotes.WorkNotes) string                      { return "" }

type noopBankSummarizer struct{}

func (n *noopBankSummarizer) Summary() (string, error) { return "", nil }

// --- Tests ---

func TestOrchestrator_RunAndStop(t *testing.T) {
	dir := setupTestDir(t)
	orch := buildTestOrchestrator(t, dir)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Run should start and then stop when context expires.
	err := orch.Run(ctx)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestOrchestrator_GracefulShutdown(t *testing.T) {
	dir := setupTestDir(t)
	orch := buildTestOrchestrator(t, dir)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- orch.Run(ctx)
	}()

	// Let the loop run a few ticks.
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error after cancel = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not return within 5s of context cancellation")
	}

	// Verify session was saved.
	sessDir := filepath.Join(dir, "sessions")
	entries, err := os.ReadDir(sessDir)
	if err != nil {
		t.Fatalf("reading sessions dir: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected at least one session file after shutdown")
	}
}

func TestOrchestrator_New_NilJournal(t *testing.T) {
	dir := setupTestDir(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cmd := &fakeCommander{}
	sm := tmux.NewSessionManager(cmd, logger)
	pm := tmux.NewPaneManager(cmd, logger)
	pluginReg := plugin.NewRegistry()
	registry := agent.NewRegistry(sm, pm, pluginReg, logger)
	watcher := tmux.NewWatcher(pm, time.Second, logger)

	specDir := filepath.Join(dir, "phases")
	gateRunner := phase.NewGateRunner(dir, 10*time.Second, logger)
	engine, _ := phase.NewEngine(specDir, gateRunner, nil, logger)
	completion := phase.NewCompletionHandler(engine, gateRunner, &noopDecisionRecorder{}, &noopWorkNotes{}, logger)
	contextBld := phase.NewContextBuilder(&noopDecisionSearcher{}, &noopWorkNotes{}, &noopBankSummarizer{}, logger)
	tracker := phase.NewTracker(engine, logger)
	sessionMgr := session.NewManager(filepath.Join(dir, "sessions"), nil, logger)
	notesMgr := worknotes.NewManager(filepath.Join(dir, "notes"), logger)
	messenger := agent.NewMessenger(pm, registry, logger)

	cfg := &config.Config{
		Project: config.ProjectConfig{Name: "test", Root: dir, StateDir: dir},
	}

	// Should not panic with nil journal.
	orch := orchestrator.New(cfg, registry, engine, completion, contextBld, tracker,
		watcher, messenger, sessionMgr, notesMgr, nil, logger)
	if orch == nil {
		t.Fatal("New() returned nil")
	}
}

// TestNewDecisionRAG_NilJournal ensures DecisionRAG handles nil journal gracefully.
func TestNewDecisionRAG_NilJournal(t *testing.T) {
	// journal.Journal satisfies JournalSearcher, but we test with nil.
	_ = journal.NewJournal(nil, nil)
}

// --- mockIdlePlugin always reports agent as ready/idle ---

type mockIdlePlugin struct {
	mockPlugin
}

func (m *mockIdlePlugin) DetectReady(_ string) bool { return true }

type mockBusyPlugin struct {
	mockPlugin
}

func (m *mockBusyPlugin) DetectBusy(_ string) bool { return true }

func TestOrchestrator_DispatchGracePeriod(t *testing.T) {
	dir := setupTestDir(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cmd := &fakeCommander{}
	sm := tmux.NewSessionManager(cmd, logger)
	pm := tmux.NewPaneManager(cmd, logger)

	// Plugin registry with a mock plugin that always reports idle.
	pluginReg := plugin.NewRegistry()
	_ = pluginReg.Register("claude", func() plugin.AgentPlugin {
		return &mockIdlePlugin{
			mockPlugin: mockPlugin{
				name: "claude",
				caps: []plugin.Capability{plugin.CapCodeGen, plugin.CapShellExec},
			},
		}
	})

	registry := agent.NewRegistry(sm, pm, pluginReg, logger)
	watcher := tmux.NewWatcher(pm, 100*time.Millisecond, logger)

	specDir := filepath.Join(dir, "phases")
	gateRunner := phase.NewGateRunner(dir, 10*time.Second, logger)
	engine, err := phase.NewEngine(specDir, gateRunner, nil, logger)
	if err != nil {
		t.Fatal(err)
	}

	completion := phase.NewCompletionHandler(engine, gateRunner, &noopDecisionRecorder{}, &noopWorkNotes{}, logger)
	contextBld := phase.NewContextBuilder(&noopDecisionSearcher{}, &noopWorkNotes{}, &noopBankSummarizer{}, logger)
	tracker := phase.NewTracker(engine, logger)
	sessionMgr := session.NewManager(filepath.Join(dir, "sessions"), nil, logger)
	notesMgr := worknotes.NewManager(filepath.Join(dir, "notes"), logger)
	messenger := agent.NewMessenger(pm, registry, logger)

	cfg := &config.Config{
		Project: config.ProjectConfig{Name: "test", Root: dir, StateDir: dir},
		Agents: map[string]config.AgentConfig{
			"claude-1": {Plugin: "claude", Role: "engineer", Permission: "standard"},
		},
	}

	orch := orchestrator.New(cfg, registry, engine, completion, contextBld, tracker,
		watcher, messenger, sessionMgr, notesMgr, nil, logger)

	// Spawn the agent directly so it's in the registry.
	a := types.Agent{
		ID:         "claude-1",
		Name:       "claude-1",
		Plugin:     "claude",
		Role:       "engineer",
		Permission: "standard",
		SessionID:  "test",
	}
	if err := registry.Spawn(context.Background(), a); err != nil {
		t.Fatal(err)
	}

	// Mark the agent as Busy in the registry (simulate post-assignment state).
	if err := registry.UpdateStatus("claude-1", types.StatusBusy); err != nil {
		t.Fatal(err)
	}

	// Also mark Busy in world state (as AssignNext would do).
	orch.WorldState().UpdateAgent("claude-1", orchestrator.AgentState{
		Status:     types.StatusBusy,
		LastActive: time.Now().UTC(),
	})

	// Simulate a recent dispatch: set lastDispatchTime to now, prevStatus to Busy.
	orch.SetTestDispatchState("claude-1", time.Now())

	// Set a long grace period so it definitely covers this test.
	orch.SetDispatchGrace(10 * time.Second)

	// Inject pane content — the idle plugin will detect this as "ready" (idle).
	orch.SetTestPaneContent("claude-1", "$ ")

	// Run a tick — the plugin sees the agent as idle, but the grace period should
	// suppress the Busy→Idle transition.
	if err := orch.Tick(context.Background()); err != nil {
		t.Fatalf("Tick() error = %v", err)
	}

	// The agent should still be Busy in the world state.
	ws := orch.WorldState()
	agentState, ok := ws.GetAgent("claude-1")
	if !ok {
		t.Fatal("agent not found in world state after tick")
	}
	if agentState.Status != types.StatusBusy {
		t.Errorf("agent status = %q after grace period tick, want %q", agentState.Status, types.StatusBusy)
	}
}

func TestOrchestrator_ReadyFallbackAfterTimeout(t *testing.T) {
	dir := setupTestDir(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cmd := &fakeCommander{}
	sm := tmux.NewSessionManager(cmd, logger)
	pm := tmux.NewPaneManager(cmd, logger)

	pluginReg := plugin.NewRegistry()
	_ = pluginReg.Register("claude", func() plugin.AgentPlugin {
		return &mockPlugin{
			name: "claude",
			caps: []plugin.Capability{plugin.CapCodeGen},
		}
	})

	registry := agent.NewRegistry(sm, pm, pluginReg, logger)
	watcher := tmux.NewWatcher(pm, 100*time.Millisecond, logger)
	specDir := filepath.Join(dir, "phases")
	gateRunner := phase.NewGateRunner(dir, 10*time.Second, logger)
	engine, err := phase.NewEngine(specDir, gateRunner, nil, logger)
	if err != nil {
		t.Fatal(err)
	}
	completion := phase.NewCompletionHandler(engine, gateRunner, &noopDecisionRecorder{}, &noopWorkNotes{}, logger)
	contextBld := phase.NewContextBuilder(&noopDecisionSearcher{}, &noopWorkNotes{}, &noopBankSummarizer{}, logger)
	tracker := phase.NewTracker(engine, logger)
	sessionMgr := session.NewManager(filepath.Join(dir, "sessions"), nil, logger)
	notesMgr := worknotes.NewManager(filepath.Join(dir, "notes"), logger)
	messenger := agent.NewMessenger(pm, registry, logger)

	cfg := &config.Config{
		Project: config.ProjectConfig{Name: "test", Root: dir, StateDir: dir},
	}
	orch := orchestrator.New(cfg, registry, engine, completion, contextBld, tracker,
		watcher, messenger, sessionMgr, notesMgr, nil, logger)
	orch.SetReadyTimeout(50 * time.Millisecond)

	a := types.Agent{
		ID:         "claude-1",
		Name:       "claude-1",
		Plugin:     "claude",
		Role:       "engineer",
		Permission: "standard",
		SessionID:  "test",
	}
	if err := registry.Spawn(context.Background(), a); err != nil {
		t.Fatal(err)
	}

	orch.SetTestPaneContent("claude-1", "startup banner")
	orch.SetTestFirstContentAt("claude-1", time.Now().Add(-time.Second))

	if !orch.IsAgentReadyForDispatch("claude-1") {
		t.Fatal("expected dispatch readiness fallback to allow agent after timeout")
	}
}

func TestOrchestrator_ReadyFallbackBlockedByBusy(t *testing.T) {
	dir := setupTestDir(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cmd := &fakeCommander{}
	sm := tmux.NewSessionManager(cmd, logger)
	pm := tmux.NewPaneManager(cmd, logger)

	pluginReg := plugin.NewRegistry()
	_ = pluginReg.Register("claude", func() plugin.AgentPlugin {
		return &mockBusyPlugin{
			mockPlugin: mockPlugin{
				name: "claude",
				caps: []plugin.Capability{plugin.CapCodeGen},
			},
		}
	})

	registry := agent.NewRegistry(sm, pm, pluginReg, logger)
	watcher := tmux.NewWatcher(pm, 100*time.Millisecond, logger)
	specDir := filepath.Join(dir, "phases")
	gateRunner := phase.NewGateRunner(dir, 10*time.Second, logger)
	engine, err := phase.NewEngine(specDir, gateRunner, nil, logger)
	if err != nil {
		t.Fatal(err)
	}
	completion := phase.NewCompletionHandler(engine, gateRunner, &noopDecisionRecorder{}, &noopWorkNotes{}, logger)
	contextBld := phase.NewContextBuilder(&noopDecisionSearcher{}, &noopWorkNotes{}, &noopBankSummarizer{}, logger)
	tracker := phase.NewTracker(engine, logger)
	sessionMgr := session.NewManager(filepath.Join(dir, "sessions"), nil, logger)
	notesMgr := worknotes.NewManager(filepath.Join(dir, "notes"), logger)
	messenger := agent.NewMessenger(pm, registry, logger)

	cfg := &config.Config{
		Project: config.ProjectConfig{Name: "test", Root: dir, StateDir: dir},
	}
	orch := orchestrator.New(cfg, registry, engine, completion, contextBld, tracker,
		watcher, messenger, sessionMgr, notesMgr, nil, logger)
	orch.SetReadyTimeout(50 * time.Millisecond)

	a := types.Agent{
		ID:         "claude-1",
		Name:       "claude-1",
		Plugin:     "claude",
		Role:       "engineer",
		Permission: "standard",
		SessionID:  "test",
	}
	if err := registry.Spawn(context.Background(), a); err != nil {
		t.Fatal(err)
	}

	orch.SetTestPaneContent("claude-1", "startup banner")
	orch.SetTestFirstContentAt("claude-1", time.Now().Add(-time.Second))

	if orch.IsAgentReadyForDispatch("claude-1") {
		t.Fatal("expected busy agent to remain non-dispatchable during fallback")
	}
}

func TestOrchestrator_SaveSessionPersistsAgentState(t *testing.T) {
	dir := setupTestDir(t)
	orch := buildTestOrchestrator(t, dir)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := orch.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	orch.WorldState().UpdateAgent("claude-1", orchestrator.AgentState{
		Status:        types.StatusBusy,
		PromptDisplay: "Phase 1A P1",
		Task:          "implement parser",
		LastActive:    time.Now().UTC(),
	})
	orch.SaveSessionForTest()

	sessionMgr := session.NewManager(filepath.Join(dir, "sessions"), nil, nil)
	sc, err := sessionMgr.ResumeLatest()
	if err != nil {
		t.Fatalf("ResumeLatest() error = %v", err)
	}
	got, ok := sc.Agents["claude-1"]
	if !ok {
		t.Fatal("expected claude-1 in persisted session agents")
	}
	if got.Status != string(types.StatusBusy) {
		t.Fatalf("persisted status = %q, want %q", got.Status, types.StatusBusy)
	}
	if got.CurrentTask != "implement parser" {
		t.Fatalf("persisted current_task = %q, want %q", got.CurrentTask, "implement parser")
	}
}
