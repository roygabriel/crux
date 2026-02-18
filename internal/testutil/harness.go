package testutil

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/roygabriel/crux/internal/agent"
	"github.com/roygabriel/crux/internal/config"
	"github.com/roygabriel/crux/internal/memory/session"
	"github.com/roygabriel/crux/internal/memory/worknotes"
	"github.com/roygabriel/crux/internal/orchestrator"
	"github.com/roygabriel/crux/internal/phase"
	"github.com/roygabriel/crux/internal/plugin"
	"github.com/roygabriel/crux/internal/tmux"
)

// TestHarness bundles the temp directory and wired test doubles for
// integration testing the orchestrator.
type TestHarness struct {
	Dir       string
	Commander *MockCommander
	Plugin    *ScenarioPlugin
	Config    *config.Config
	Logger    *slog.Logger
}

// NewTestHarness creates a temp directory with standard structure, writes
// a 2-phase project, and prepares mock dependencies. The temp directory
// is cleaned up via t.Cleanup.
func NewTestHarness(t *testing.T) *TestHarness {
	t.Helper()

	dir := t.TempDir()
	SetupDirs(t, dir)
	SetupTwoPhaseProject(t, dir)

	commander := NewMockCommander("")
	scenarioPlugin := NewScenarioPlugin("test-plugin")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cfg := &config.Config{
		Project: config.ProjectConfig{
			Name:     "test-project",
			Root:     dir,
			StateDir: dir,
		},
		Agents: map[string]config.AgentConfig{
			"agent-1": {Plugin: "test-plugin", Role: "engineer", Permission: "standard"},
		},
		Memory: config.MemoryConfig{
			SQLitePath:        filepath.Join(dir, "memory.db"),
			VectorDir:         filepath.Join(dir, "vectors"),
			EmbeddingProvider: "chromem-default",
		},
	}

	return &TestHarness{
		Dir:       dir,
		Commander: commander,
		Plugin:    scenarioPlugin,
		Config:    cfg,
		Logger:    logger,
	}
}

// BuildOrchestrator wires all dependencies and returns a ready-to-run
// orchestrator along with a tickCh that receives a value after each tick.
func (h *TestHarness) BuildOrchestrator() (*orchestrator.Orchestrator, <-chan struct{}) {
	sm := tmux.NewSessionManager(h.Commander, h.Logger)
	pm := tmux.NewPaneManager(h.Commander, h.Logger)

	pluginReg := plugin.NewRegistry()
	scenarioPlugin := h.Plugin
	_ = pluginReg.Register(scenarioPlugin.Name(), func() plugin.AgentPlugin {
		return scenarioPlugin
	})

	registry := agent.NewRegistry(sm, pm, pluginReg, h.Logger)
	watcher := tmux.NewWatcher(pm, 50*time.Millisecond, h.Logger)

	specDir := filepath.Join(h.Dir, "phases")
	gateRunner := phase.NewGateRunner(h.Dir, 10*time.Second, h.Logger)
	engine, err := phase.NewEngine(specDir, gateRunner, nil, h.Logger)
	if err != nil {
		// Panic is acceptable in test setup — NewTestHarness already validated dir.
		panic("testutil: NewEngine: " + err.Error())
	}

	completion := phase.NewCompletionHandler(engine, gateRunner, NoopDecisionRecorder{}, NoopWorkNotes{}, h.Logger)
	contextBld := phase.NewContextBuilder(NoopDecisionSearcher{}, NoopWorkNotes{}, NoopBankSummarizer{}, h.Logger)
	tracker := phase.NewTracker(engine, h.Logger)

	sessDir := filepath.Join(h.Dir, "sessions")
	sessionMgr := session.NewManager(sessDir, nil, h.Logger)

	notesDir := filepath.Join(h.Dir, "notes")
	notesMgr := worknotes.NewManager(notesDir, h.Logger)

	messenger := agent.NewMessenger(pm, registry, h.Logger)

	orch := orchestrator.New(
		h.Config, registry, engine, completion, contextBld, tracker,
		watcher, messenger, sessionMgr, notesMgr, nil, h.Logger,
	)

	tickCh := make(chan struct{}, 64)
	orch.SetTickHook(func() {
		tickCh <- struct{}{}
	})

	return orch, tickCh
}

// WaitForNTicks blocks until n ticks have been received or timeout expires.
// Returns the number of ticks actually received.
func WaitForNTicks(tickCh <-chan struct{}, n int, timeout time.Duration) int {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	received := 0
	for received < n {
		select {
		case <-tickCh:
			received++
		case <-timer.C:
			return received
		}
	}
	return received
}
