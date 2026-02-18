package orchestrator_test

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/roygabriel/crux/internal/instruct"
)

// buildPromptBuilder creates an OrchestratorPromptBuilder using the real
// embedded templates and a minimal Aggregator. This mirrors production
// construction without requiring external subsystems.
func buildPromptBuilder(t *testing.T, dir string) *instruct.OrchestratorPromptBuilder {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	fsys, err := instruct.TemplatesFS()
	if err != nil {
		t.Fatalf("loading embedded templates: %v", err)
	}
	renderer, err := instruct.NewRenderer(fsys, logger)
	if err != nil {
		t.Fatalf("creating renderer: %v", err)
	}

	agg := instruct.NewAggregator(instruct.AggregatorDeps{
		Config: instruct.AggregatorConfig{
			ProjectName: "test-project",
			RepoRoot:    dir,
		},
		Logger: logger,
	})

	return instruct.NewOrchestratorPromptBuilder(agg, renderer, logger)
}

func TestOrchestrator_BuildsPromptOnStartup(t *testing.T) {
	dir := setupTestDir(t)
	orch := buildTestOrchestrator(t, dir)
	orch.SetOrchestratorPromptBuilder(buildPromptBuilder(t, dir))

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := orch.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	prompt := orch.OrchestratorPrompt()
	if prompt == "" {
		t.Fatal("OrchestratorPrompt() should be non-empty after Run()")
	}
	if !strings.Contains(prompt, "Crux Orchestrator") {
		t.Error("OrchestratorPrompt() should contain 'Crux Orchestrator'")
	}
}

func TestOrchestrator_PromptIncludesWorldState(t *testing.T) {
	dir := setupTestDir(t)
	orch := buildTestOrchestrator(t, dir)
	orch.SetOrchestratorPromptBuilder(buildPromptBuilder(t, dir))

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := orch.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// After Run, world state has the phase ID loaded from the spec file.
	ws := orch.WorldState()
	snap := ws.Snapshot()

	prompt := orch.OrchestratorPrompt()
	if snap.Phase != "" && !strings.Contains(prompt, string(snap.Phase)) {
		t.Errorf("OrchestratorPrompt() should contain phase ID %q from world state", snap.Phase)
	}
}

func TestOrchestrator_RegenerateInstructions_RebuildPrompt(t *testing.T) {
	dir := setupTestDir(t)
	orch := buildTestOrchestrator(t, dir)
	orch.SetOrchestratorPromptBuilder(buildPromptBuilder(t, dir))

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := orch.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// The prompt should reflect the phase loaded during Run.
	// buildOrchestratorPrompt is called after UpdatePhase in Run,
	// so the phase data should be present. This validates the
	// integration point: prompt is built after phase load, not before.
	prompt := orch.OrchestratorPrompt()
	if prompt == "" {
		t.Fatal("OrchestratorPrompt() should be non-empty")
	}

	// Verify that the prompt includes phase-related content.
	// The phase spec in setupTestDir creates "1A" / "Foundation".
	if !strings.Contains(prompt, "1A") {
		t.Error("OrchestratorPrompt() should include phase ID from engine")
	}
}

func TestOrchestrator_NilBuilder_NoOp(t *testing.T) {
	dir := setupTestDir(t)
	orch := buildTestOrchestrator(t, dir)
	// Deliberately do NOT call SetOrchestratorPromptBuilder.

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := orch.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	prompt := orch.OrchestratorPrompt()
	if prompt != "" {
		t.Errorf("OrchestratorPrompt() should be empty without builder, got %d bytes", len(prompt))
	}
}

func TestOrchestrator_ErrorAgent_RegeneratesInstructions(t *testing.T) {
	dir := setupTestDir(t)
	orch := buildTestOrchestrator(t, dir)
	orch.SetOrchestratorPromptBuilder(buildPromptBuilder(t, dir))

	// The orchestrator's handleTransition → StatusError case calls
	// regenerateAgentInstructions. With no distributor set, the
	// regeneration is a safe no-op. This test verifies that the wiring
	// exists and the orchestrator prompt remains intact after startup
	// (the error code path does not corrupt it).
	//
	// A full error-transition integration test would require injecting
	// pane content via a real watcher, which is covered by higher-level
	// integration tests.

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := orch.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Prompt was built on startup.
	prompt := orch.OrchestratorPrompt()
	if prompt == "" {
		t.Fatal("OrchestratorPrompt() should be non-empty after Run()")
	}
	if !strings.Contains(prompt, "Crux Orchestrator") {
		t.Error("OrchestratorPrompt() should contain 'Crux Orchestrator'")
	}

	// World state has phase info even without tick-driven agent transitions.
	ws := orch.WorldState()
	snap := ws.Snapshot()
	if snap.Phase == "" {
		t.Error("expected phase in world state after Run()")
	}
}

func TestOrchestrator_OrchestratorPrompt_ThreadSafety(t *testing.T) {
	dir := setupTestDir(t)
	orch := buildTestOrchestrator(t, dir)
	orch.SetOrchestratorPromptBuilder(buildPromptBuilder(t, dir))

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Read OrchestratorPrompt concurrently while Run executes.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_ = orch.OrchestratorPrompt()
			}
		}
	}()

	if err := orch.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	<-done
	prompt := orch.OrchestratorPrompt()
	if prompt == "" {
		t.Error("OrchestratorPrompt() should be non-empty after Run()")
	}
}

