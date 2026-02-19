package orchestrator

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/roygabriel/crux/internal/phase"
)

func TestReconcileFiles_AllPresent(t *testing.T) {
	root := initGitRepo(t)
	writeFile(t, filepath.Join(root, "go.mod"), "module x\n")
	writeFile(t, filepath.Join(root, "main.go"), "package main\n")

	spec := phase.PhaseSpec{
		FilesNew: []string{"go.mod", "main.go"},
	}
	ev, err := ReconcileFiles(context.Background(), root, spec, "")
	if err != nil {
		t.Fatalf("ReconcileFiles: %v", err)
	}
	if !ev.IsComplete() {
		t.Fatalf("expected complete evidence: %#v", ev)
	}
	if got := len(ev.Missing); got != 0 {
		t.Fatalf("missing count = %d, want 0", got)
	}
}

func TestReconcileFiles_Missing(t *testing.T) {
	root := initGitRepo(t)
	writeFile(t, filepath.Join(root, "go.mod"), "module x\n")
	spec := phase.PhaseSpec{
		FilesNew: []string{"go.mod", "cmd/main.go"},
	}

	ev, err := ReconcileFiles(context.Background(), root, spec, "")
	if err != nil {
		t.Fatalf("ReconcileFiles: %v", err)
	}
	if ev.IsComplete() {
		t.Fatalf("expected incomplete evidence: %#v", ev)
	}
	if len(ev.Missing) != 1 || ev.Missing[0] != "cmd/main.go" {
		t.Fatalf("missing = %v, want [cmd/main.go]", ev.Missing)
	}
}

func TestReconcileFiles_NoExpectedFiles(t *testing.T) {
	root := initGitRepo(t)
	spec := phase.PhaseSpec{}

	ev, err := ReconcileFiles(context.Background(), root, spec, "")
	if err != nil {
		t.Fatalf("ReconcileFiles: %v", err)
	}
	if !ev.IsComplete() {
		t.Fatalf("expected complete evidence when no expected files: %#v", ev)
	}
}

func TestReconcileFiles_NonexistentRoot(t *testing.T) {
	spec := phase.PhaseSpec{FilesNew: []string{"main.go"}}
	_, err := ReconcileFiles(context.Background(), filepath.Join(t.TempDir(), "missing"), spec, "")
	if err == nil {
		t.Fatal("expected error for missing root")
	}
}

func TestReconcileFiles_EmptySinceCommit(t *testing.T) {
	root := initGitRepo(t)
	writeFile(t, filepath.Join(root, "main.go"), "package main\n")

	spec := phase.PhaseSpec{FilesNew: []string{"main.go"}}
	ev, err := ReconcileFiles(context.Background(), root, spec, "")
	if err != nil {
		t.Fatalf("ReconcileFiles: %v", err)
	}
	if len(ev.GitEvidence) == 0 {
		t.Fatalf("expected git evidence with empty sinceCommit")
	}
}

func TestFilesystemEvidenceSummary(t *testing.T) {
	ev := &FilesystemEvidence{
		Expected:    []string{"a.go", "b.go", "c.go"},
		Missing:     []string{"b.go", "c.go"},
		GitEvidence: []string{"a.go"},
	}
	s := ev.Summary()
	if !strings.Contains(s, "Missing 2 of 3 expected files") {
		t.Fatalf("unexpected summary: %q", s)
	}
	if !strings.Contains(s, "b.go, c.go") {
		t.Fatalf("missing list not included: %q", s)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	run(t, root, "git", "init")
	run(t, root, "git", "config", "user.email", "test@example.com")
	run(t, root, "git", "config", "user.name", "Test")
	writeFile(t, filepath.Join(root, ".gitignore"), ".DS_Store\n")
	run(t, root, "git", "add", ".gitignore")
	run(t, root, "git", "commit", "-m", "init")
	return root
}

func run(t *testing.T, dir string, cmd string, args ...string) {
	t.Helper()
	c := exec.Command(cmd, args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", cmd, args, err, string(out))
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
