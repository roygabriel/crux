//go:build integration

package orchestrator_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/roygabriel/crux/internal/testutil"
	"github.com/roygabriel/crux/pkg/types"
)

// TestFullOrchestrationLoop verifies the primary happy path: an agent
// receives work, transitions through Idle → Busy → Idle, completion
// handler advances the engine, and the session is persisted.
func TestFullOrchestrationLoop(t *testing.T) {
	h := testutil.NewTestHarness(t)

	// Script a full lifecycle for agent pane "%1":
	//   ""  → ">" → "⠋ thinking..." → ">"   (prompt 1 of phase A)
	//   then "⠋ thinking..." → ">"           (prompt 2 of phase A)
	//   then "⠋ thinking..." → ">"           (prompt 1 of phase B)
	h.Commander.AddScript("%1", []testutil.ResponseStep{
		{Content: ""},                        // initial empty
		{Content: testutil.ContentReady},     // agent ready → idle, gets assigned
		{Content: testutil.ContentBusy},      // agent working
		{Content: testutil.ContentReady},     // done → completion for A/P1
		{Content: testutil.ContentBusy},      // assigned A/P2, working
		{Content: testutil.ContentReady},     // done → completion for A/P2
		{Content: testutil.ContentBusy},      // assigned B/P1, working
		{Content: testutil.ContentReady},     // done → completion for B/P1
	})

	orch, tickCh := h.BuildOrchestrator()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- orch.Run(ctx)
	}()

	// Wait for enough ticks to process the full lifecycle.
	// Each tick is 2s, watcher polls at 50ms. We need at least 8 ticks
	// to process all script steps (one per watcher cycle that delivers
	// new content, plus tick processing time).
	got := testutil.WaitForNTicks(tickCh, 10, 25*time.Second)
	if got < 4 {
		t.Fatalf("only received %d ticks, need at least 4", got)
	}

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not return within 5s of cancellation")
	}

	// Assert session file exists.
	sessDir := filepath.Join(h.Dir, "sessions")
	entries, err := os.ReadDir(sessDir)
	if err != nil {
		t.Fatalf("reading sessions dir: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected at least one session file")
	}

	// Assert world state reflects activity.
	ws := orch.WorldState()
	if ws.SessionID == "" {
		t.Error("world state session ID is empty")
	}
}

// TestGateFailureHaltsProgress verifies that when a security gate denies
// verification commands, the engine does not advance.
func TestGateFailureHaltsProgress(t *testing.T) {
	h := testutil.NewTestHarness(t)

	// Script: agent goes idle → busy → idle (triggers completion).
	h.Commander.AddScript("%1", []testutil.ResponseStep{
		{Content: ""},
		{Content: testutil.ContentReady},
		{Content: testutil.ContentBusy},
		{Content: testutil.ContentReady},
	})

	orch, tickCh := h.BuildOrchestrator()
	orch.SetSecurityGate(testutil.NewDenyingSecurityGate(nil))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- orch.Run(ctx)
	}()

	testutil.WaitForNTicks(tickCh, 6, 15*time.Second)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not return within 5s of cancellation")
	}

	// World state phase should still be phase A since gate denied advancement.
	ws := orch.WorldState()
	if ws.Phase != "" && ws.Phase != "A" {
		t.Errorf("expected phase to be A (or empty), got %q", ws.Phase)
	}
}

// TestGracefulShutdown verifies that cancelling the context causes Run()
// to return promptly and save the session.
func TestGracefulShutdown(t *testing.T) {
	h := testutil.NewTestHarness(t)

	// Minimal script — just empty content.
	h.Commander.AddScript("%1", []testutil.ResponseStep{
		{Content: ""},
		{Content: ""},
	})

	orch, tickCh := h.BuildOrchestrator()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- orch.Run(ctx)
	}()

	// Wait for 2 ticks then cancel.
	testutil.WaitForNTicks(tickCh, 2, 10*time.Second)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not return within 5s of cancellation")
	}

	// Assert session file was saved.
	sessDir := filepath.Join(h.Dir, "sessions")
	entries, err := os.ReadDir(sessDir)
	if err != nil {
		t.Fatalf("reading sessions dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one session file after shutdown")
	}

	// Assert session file contains valid JSON with session ID.
	data, err := os.ReadFile(filepath.Join(sessDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("reading session file: %v", err)
	}
	var session struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &session); err != nil {
		t.Fatalf("unmarshaling session: %v", err)
	}
	if session.ID == "" {
		t.Error("session ID is empty")
	}
}

// TestSecurityGating verifies that the security gate blocks completion
// advancement when it denies file write operations.
func TestSecurityGating(t *testing.T) {
	h := testutil.NewTestHarness(t)

	// Agent completes work (Busy→Idle) but security gate denies.
	h.Commander.AddScript("%1", []testutil.ResponseStep{
		{Content: ""},
		{Content: testutil.ContentReady},
		{Content: testutil.ContentBusy},
		{Content: testutil.ContentReady},
		{Content: testutil.ContentReady}, // stays idle
	})

	orch, tickCh := h.BuildOrchestrator()
	orch.SetSecurityGate(testutil.NewDenyingSecurityGate(nil))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- orch.Run(ctx)
	}()

	testutil.WaitForNTicks(tickCh, 6, 15*time.Second)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not return within 5s of cancellation")
	}

	// Phase should not have advanced past A.
	ws := orch.WorldState()
	if ws.Phase == "B" {
		t.Error("phase should not have advanced to B with denying security gate")
	}
}

// TestAgentErrorDetection verifies that error sentinel content is detected
// and reflected in the world state.
func TestAgentErrorDetection(t *testing.T) {
	h := testutil.NewTestHarness(t)

	// Set default to error content so once the script is exhausted, the
	// watcher continues to see error content (not empty string).
	h.Commander.SetDefaultContent(testutil.ContentError)

	// Agent starts, gets assigned, then reports an error.
	h.Commander.AddScript("%1", []testutil.ResponseStep{
		{Content: ""},
		{Content: testutil.ContentReady},
		{Content: testutil.ContentError},
	})

	orch, tickCh := h.BuildOrchestrator()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- orch.Run(ctx)
	}()

	testutil.WaitForNTicks(tickCh, 5, 15*time.Second)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not return within 5s of cancellation")
	}

	// Assert world state shows agent in error status.
	ws := orch.WorldState()
	agentState, ok := ws.GetAgent("agent-1")
	if !ok {
		t.Fatal("agent-1 not found in world state")
	}
	if agentState.Status != types.StatusError {
		t.Errorf("agent status = %q, want %q", agentState.Status, types.StatusError)
	}
}

// TestPromptAutoResponse verifies that when an agent pane shows an
// interactive prompt, the orchestrator detects it, sends the auto-accept
// keys, and the agent transitions through prompted → ready.
func TestPromptAutoResponse(t *testing.T) {
	h := testutil.NewTestHarness(t)

	// Set default to prompted content so the watcher continues to see
	// it after the script is exhausted (watcher polls faster than ticks).
	h.Commander.SetDefaultContent(testutil.ContentPrompted)

	// Agent starts, then shows a trust prompt.
	h.Commander.AddScript("%1", []testutil.ResponseStep{
		{Content: ""},
		{Content: testutil.ContentPrompted},
	})

	orch, tickCh := h.BuildOrchestrator()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- orch.Run(ctx)
	}()

	testutil.WaitForNTicks(tickCh, 5, 15*time.Second)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not return within 5s of cancellation")
	}

	// Verify send-keys was called with "y" to accept the prompt.
	sendCalls := h.Commander.CallsForSubcommand("send-keys")
	found := false
	for _, c := range sendCalls {
		for _, arg := range c.Args {
			if arg == "y" {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Error("expected send-keys call with 'y' for prompt auto-accept, none found")
	}

	// Verify world state shows agent in prompted status.
	ws := orch.WorldState()
	agentState, ok := ws.GetAgent("agent-1")
	if !ok {
		t.Fatal("agent-1 not found in world state")
	}
	if agentState.Status != types.StatusPrompted {
		t.Errorf("agent status = %q, want %q", agentState.Status, types.StatusPrompted)
	}
}

// TestAgentRateLimitDetection verifies that rate-limit sentinel content
// is detected and reflected in the world state.
func TestAgentRateLimitDetection(t *testing.T) {
	h := testutil.NewTestHarness(t)

	// Set default to rate-limit content so the watcher continues to see
	// rate-limit after the script is exhausted.
	h.Commander.SetDefaultContent(testutil.ContentRateLimit)

	// Agent starts, gets assigned, then hits rate limit.
	h.Commander.AddScript("%1", []testutil.ResponseStep{
		{Content: ""},
		{Content: testutil.ContentReady},
		{Content: testutil.ContentRateLimit},
	})

	orch, tickCh := h.BuildOrchestrator()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- orch.Run(ctx)
	}()

	testutil.WaitForNTicks(tickCh, 5, 15*time.Second)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not return within 5s of cancellation")
	}

	// Assert world state shows agent as rate limited.
	ws := orch.WorldState()
	agentState, ok := ws.GetAgent("agent-1")
	if !ok {
		t.Fatal("agent-1 not found in world state")
	}
	if agentState.Status != types.StatusRateLimited {
		t.Errorf("agent status = %q, want %q", agentState.Status, types.StatusRateLimited)
	}
}
