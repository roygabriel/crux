package orchestrator_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/roygabriel/crux/internal/agent"
	"github.com/roygabriel/crux/internal/memory/worknotes"
	"github.com/roygabriel/crux/internal/orchestrator"
	"github.com/roygabriel/crux/pkg/types"
)

// --- Mock PhaseValidator ---

type mockPhaseValidator struct {
	err error
}

func (m *mockPhaseValidator) ValidateParallelism(_ []types.PhaseID) error {
	return m.err
}

// --- Mock GitDiffer ---

type mockGitDiffer struct {
	files []string
	err   error
}

func (m *mockGitDiffer) DiffNames(_ context.Context) ([]string, error) {
	return m.files, m.err
}

// --- Mock WorkNotesAppender ---

type mockWorkNotesAppender struct {
	entries []worknotes.SessionLogEntry
}

func (m *mockWorkNotesAppender) AppendSession(_ string, entry worknotes.SessionLogEntry) error {
	m.entries = append(m.entries, entry)
	return nil
}

// --- Mock DecisionRecorder ---

type mockDecisionRecorder struct {
	decisions []types.Decision
}

func (m *mockDecisionRecorder) Record(_ context.Context, d types.Decision) error {
	m.decisions = append(m.decisions, d)
	return nil
}

// --- Tests ---

func TestCheckBeforeAssign_NoConflict(t *testing.T) {
	validator := &mockPhaseValidator{err: nil}
	cd := orchestrator.NewConflictDetector(
		validator, &mockGitDiffer{}, &mockAgentLister{}, orchestrator.NewWorldState(""),
		&mockWorkNotesAppender{}, &mockDecisionRecorder{}, time.Second, nil,
	)

	err := cd.CheckBeforeAssign("1A", "2A")
	if err != nil {
		t.Fatalf("CheckBeforeAssign() error = %v, want nil", err)
	}
}

func TestCheckBeforeAssign_Conflict(t *testing.T) {
	validator := &mockPhaseValidator{
		err: fmt.Errorf("file conflicts detected: config.go: phases 1A and 2A"),
	}
	cd := orchestrator.NewConflictDetector(
		validator, &mockGitDiffer{}, &mockAgentLister{}, orchestrator.NewWorldState(""),
		&mockWorkNotesAppender{}, &mockDecisionRecorder{}, time.Second, nil,
	)

	err := cd.CheckBeforeAssign("1A", "2A")
	if err == nil {
		t.Fatal("CheckBeforeAssign() expected error for overlapping phases")
	}
	if !containsStr(err.Error(), "config.go") {
		t.Errorf("error should mention conflicting file, got: %s", err.Error())
	}
}

func TestMonitorRuntime_DetectsConflict(t *testing.T) {
	differ := &mockGitDiffer{files: []string{"shared.go"}}
	ws := orchestrator.NewWorldState("sess-1")
	cd := orchestrator.NewConflictDetector(
		&mockPhaseValidator{}, differ, &mockAgentLister{}, ws,
		&mockWorkNotesAppender{}, &mockDecisionRecorder{}, 50*time.Millisecond, nil,
	)

	cd.TrackAssignment("agent-1", "1A", []string{"shared.go", "foo.go"})
	cd.TrackAssignment("agent-2", "2A", []string{"shared.go", "bar.go"})

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	ch := cd.MonitorRuntime(ctx)

	var events []orchestrator.ConflictEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) == 0 {
		t.Fatal("expected at least one conflict event")
	}

	ev := events[0]
	if len(ev.ConflictingFiles) == 0 {
		t.Fatal("expected conflicting files in event")
	}
	if ev.ConflictingFiles[0] != "shared.go" {
		t.Errorf("conflicting file = %q, want %q", ev.ConflictingFiles[0], "shared.go")
	}
}

func TestMonitorRuntime_NoConflict(t *testing.T) {
	differ := &mockGitDiffer{files: []string{"unrelated.go"}}
	ws := orchestrator.NewWorldState("sess-1")
	cd := orchestrator.NewConflictDetector(
		&mockPhaseValidator{}, differ, &mockAgentLister{}, ws,
		&mockWorkNotesAppender{}, &mockDecisionRecorder{}, 50*time.Millisecond, nil,
	)

	cd.TrackAssignment("agent-1", "1A", []string{"foo.go"})
	cd.TrackAssignment("agent-2", "2A", []string{"bar.go"})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	ch := cd.MonitorRuntime(ctx)

	var events []orchestrator.ConflictEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) != 0 {
		t.Errorf("expected no conflict events, got %d", len(events))
	}
}

func TestHandleConflict_HaltsLaterAgent(t *testing.T) {
	ws := orchestrator.NewWorldState("sess-1")
	now := time.Now().UTC()
	ws.UpdateAgent("agent-1", orchestrator.AgentState{
		Status:     types.StatusBusy,
		PhaseID:    "1A",
		AssignedAt: now.Add(-time.Minute),
	})
	ws.UpdateAgent("agent-2", orchestrator.AgentState{
		Status:     types.StatusBusy,
		PhaseID:    "2A",
		AssignedAt: now,
	})

	lister := &mockAgentLister{
		instances: []*agent.AgentInstance{},
	}
	// The lister just needs to accept UpdateStatus calls.
	notesAppender := &mockWorkNotesAppender{}
	recorder := &mockDecisionRecorder{}
	logger := slog.Default()

	cd := orchestrator.NewConflictDetector(
		&mockPhaseValidator{}, &mockGitDiffer{}, lister, ws,
		notesAppender, recorder, time.Second, logger,
	)

	event := orchestrator.ConflictEvent{
		AgentA:           "agent-1",
		AgentB:           "agent-2",
		PhaseA:           "1A",
		PhaseB:           "2A",
		ConflictingFiles: []string{"shared.go"},
		DetectedAt:       now,
	}

	err := cd.HandleConflict(context.Background(), event)
	if err != nil {
		t.Fatalf("HandleConflict() error = %v", err)
	}

	// agent-2 was assigned later, so it should be halted.
	if !lister.updateCalled {
		t.Fatal("expected UpdateStatus to be called")
	}
	if lister.updateID != "agent-2" {
		t.Errorf("halted agent = %q, want %q", lister.updateID, "agent-2")
	}
	if lister.updateStatus != types.StatusError {
		t.Errorf("halted status = %q, want %q", lister.updateStatus, types.StatusError)
	}

	// Work notes should have entries for both phases.
	if len(notesAppender.entries) < 2 {
		t.Errorf("expected at least 2 work notes entries, got %d", len(notesAppender.entries))
	}

	// Decision should be recorded.
	if len(recorder.decisions) == 0 {
		t.Error("expected at least one decision recorded")
	}
}

func TestTrackUntrack(t *testing.T) {
	differ := &mockGitDiffer{files: []string{"shared.go"}}
	ws := orchestrator.NewWorldState("sess-1")
	cd := orchestrator.NewConflictDetector(
		&mockPhaseValidator{}, differ, &mockAgentLister{}, ws,
		&mockWorkNotesAppender{}, &mockDecisionRecorder{}, 50*time.Millisecond, nil,
	)

	// Track two agents on same file, then untrack one.
	cd.TrackAssignment("agent-1", "1A", []string{"shared.go"})
	cd.TrackAssignment("agent-2", "2A", []string{"shared.go"})
	cd.UntrackAssignment("agent-2")

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	ch := cd.MonitorRuntime(ctx)

	var events []orchestrator.ConflictEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) != 0 {
		t.Errorf("expected no conflict events after untrack, got %d", len(events))
	}
}

func TestExecGitDiffer_NoHEAD(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// git init with no commits — HEAD is unborn.
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %s: %v", out, err)
	}

	differ := orchestrator.NewExecGitDiffer(dir)
	files, err := differ.DiffNames(context.Background())
	if err != nil {
		t.Fatalf("DiffNames() error = %v, want nil", err)
	}
	if files != nil {
		t.Errorf("DiffNames() = %v, want nil", files)
	}
}

func TestExecGitDiffer_WithCommitAndChanges(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Initialize repo and create a commit.
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}

	// Create and commit a file.
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "main.go"},
		{"commit", "-m", "initial"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}

	// Modify the file to create a diff.
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	differ := orchestrator.NewExecGitDiffer(dir)
	files, err := differ.DiffNames(context.Background())
	if err != nil {
		t.Fatalf("DiffNames() error = %v", err)
	}
	if len(files) != 1 || files[0] != "main.go" {
		t.Errorf("DiffNames() = %v, want [main.go]", files)
	}
}
