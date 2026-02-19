package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/roygabriel/crux/internal/memory/session"
	"github.com/roygabriel/crux/internal/phase"
	"github.com/roygabriel/crux/pkg/types"
)

func TestSessionReconciler_AllCompletedPromptsPass(t *testing.T) {
	root := initReconcileRepo(t)
	writeText(t, filepath.Join(root, "main.go"), "package main\n")
	engine, runner := buildReconcileEngine(t, root, "test -f main.go")
	sessions := &stubSessionResumer{
		sc: &session.SessionContext{CurrentPhase: "1A", PromptProgress: 2},
	}

	r := NewSessionReconciler(engine, runner, sessions, nil, nil)
	got, err := r.Reconcile(context.Background(), root)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if got.RolledBack {
		t.Fatalf("expected no rollback, got %+v", got)
	}
	if got.VerifiedPhase != "1A" || got.VerifiedPrompt != 2 {
		t.Fatalf("verified cursor = %s:%d, want 1A:2", got.VerifiedPhase, got.VerifiedPrompt)
	}
}

func TestSessionReconciler_RollsBackOnMissingFiles(t *testing.T) {
	root := initReconcileRepo(t)
	engine, runner := buildReconcileEngine(t, root, "test -f main.go")
	sessions := &stubSessionResumer{
		sc: &session.SessionContext{CurrentPhase: "1A", PromptProgress: 2},
	}

	r := NewSessionReconciler(engine, runner, sessions, nil, nil)
	got, err := r.Reconcile(context.Background(), root)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if !got.RolledBack {
		t.Fatalf("expected rollback, got %+v", got)
	}
	if got.VerifiedPhase != "1A" || got.VerifiedPrompt != 1 {
		t.Fatalf("verified cursor = %s:%d, want 1A:1", got.VerifiedPhase, got.VerifiedPrompt)
	}
	if len(got.MissingFiles) == 0 {
		t.Fatal("expected missing files in reconcile result")
	}
}

func TestSessionReconciler_RollsBackOnGateFailure(t *testing.T) {
	root := initReconcileRepo(t)
	writeText(t, filepath.Join(root, "main.go"), "package main\n")
	engine, runner := buildReconcileEngine(t, root, "false")
	sessions := &stubSessionResumer{
		sc: &session.SessionContext{CurrentPhase: "1A", PromptProgress: 2},
	}

	r := NewSessionReconciler(engine, runner, sessions, nil, nil)
	got, err := r.Reconcile(context.Background(), root)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if !got.RolledBack {
		t.Fatalf("expected rollback, got %+v", got)
	}
	if len(got.FailedGates) == 0 {
		t.Fatal("expected failed gate details")
	}
}

func TestSessionReconciler_FreshSession(t *testing.T) {
	root := initReconcileRepo(t)
	engine, runner := buildReconcileEngine(t, root, "true")
	sessions := &stubSessionResumer{err: session.ErrSessionNotFound}

	r := NewSessionReconciler(engine, runner, sessions, nil, nil)
	got, err := r.Reconcile(context.Background(), root)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if got.ClaimedPhase != "" || got.VerifiedPhase != "" {
		t.Fatalf("expected empty result for fresh session, got %+v", got)
	}
}

func TestSessionReconciler_WritesJournalEntryOnRollback(t *testing.T) {
	root := initReconcileRepo(t)
	engine, runner := buildReconcileEngine(t, root, "false")
	sessions := &stubSessionResumer{
		sc: &session.SessionContext{CurrentPhase: "1A", PromptProgress: 2},
	}
	rec := &stubDecisionRecorder{}

	r := NewSessionReconciler(engine, runner, sessions, rec, nil)
	got, err := r.Reconcile(context.Background(), root)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if !got.RolledBack {
		t.Fatalf("expected rollback, got %+v", got)
	}
	if len(rec.decisions) != 1 {
		t.Fatalf("journal decisions = %d, want 1", len(rec.decisions))
	}
	if !strings.Contains(rec.decisions[0].Rationale, "rolled back") {
		t.Fatalf("unexpected journal rationale: %q", rec.decisions[0].Rationale)
	}
}

type stubSessionResumer struct {
	sc  *session.SessionContext
	err error
}

func (s *stubSessionResumer) ResumeLatest() (*session.SessionContext, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.sc == nil {
		return nil, session.ErrSessionNotFound
	}
	return s.sc, nil
}

type stubDecisionRecorder struct {
	decisions []types.Decision
}

func (s *stubDecisionRecorder) Record(_ context.Context, d types.Decision) error {
	s.decisions = append(s.decisions, d)
	return nil
}

func buildReconcileEngine(t *testing.T, root, gateCmd string) (*phase.Engine, *phase.GateRunner) {
	t.Helper()
	specDir := t.TempDir()
	tick := "`"
	specDoc := strings.Join([]string{
		"# Phase 1A: Reconcile Test",
		"## Status",
		"planned",
		"## Depends On",
		"none",
		"## Design Rationale",
		"test",
		"## Tasks",
		"### Prompt 1",
		"- do work",
		"### Prompt 2",
		"- do more",
		"## Files",
		"### New",
		"- main.go",
		"### Modified",
		"- none",
		"### Referenced",
		"- none",
		"## Exit Criteria",
		"- [ ] " + tick + "true" + tick,
	}, "\n")
	writeText(t, filepath.Join(specDir, "PHASE1A.md"), specDoc)
	fence := "```"
	promptDoc := fmt.Sprintf(strings.Join([]string{
		"# Phase 1A Prompts",
		"",
		"## Prompt 1 of 2: First",
		"### Required Reading",
		"- internal/x.go",
		"### Task",
		"1. Do first thing",
		"### Verification",
		fence + "bash",
		"%s",
		fence,
		"### Acceptance Criteria",
		"- First done",
		"",
		"## Prompt 2 of 2: Second",
		"### Required Reading",
		"- internal/y.go",
		"### Task",
		"1. Do second thing",
		"### Verification",
		fence + "bash",
		"true",
		fence,
		"### Acceptance Criteria",
		"- Second done",
	}, "\n"), gateCmd)
	writeText(t, filepath.Join(specDir, "PHASE1A-PROMPT.md"), promptDoc)

	runner := phase.NewGateRunner(root, 5*time.Second, nil)
	engine, err := phase.NewEngine(specDir, runner, nil, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := engine.LoadAll(); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	return engine, runner
}

func initReconcileRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runCmdReconcile(t, root, "git", "init")
	runCmdReconcile(t, root, "git", "config", "user.email", "test@example.com")
	runCmdReconcile(t, root, "git", "config", "user.name", "Test")
	writeText(t, filepath.Join(root, ".gitignore"), ".DS_Store\n")
	runCmdReconcile(t, root, "git", "add", ".gitignore")
	runCmdReconcile(t, root, "git", "commit", "-m", "init")
	return root
}

func runCmdReconcile(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, string(out))
	}
}

func writeText(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimLeft(content, "\n")), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
