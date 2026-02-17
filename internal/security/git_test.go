package security

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/roygabriel/crux/pkg/types"
)

// initTestGitRepo creates a temporary git repository with an initial commit.
func initTestGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runCmd(t, dir, "git", "init")
	runCmd(t, dir, "git", "config", "user.email", "test@test.com")
	runCmd(t, dir, "git", "config", "user.name", "Test")
	writeFile(t, filepath.Join(dir, "README.md"), "# test")
	runCmd(t, dir, "git", "add", ".")
	runCmd(t, dir, "git", "commit", "-m", "initial")
	return dir
}

// runCmd executes a command in dir and fails the test on error.
func runCmd(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}

// --- Validation tests (no git repo needed) ---

func TestGitGuard_ValidateNotProtected_Blocked(t *testing.T) {
	t.Parallel()
	g := NewGitGuard(t.TempDir(), nil)

	blocked := []string{"main", "master", "develop", "production", "release/v1.0"}
	for _, branch := range blocked {
		err := g.ValidateNotProtected(branch)
		if err == nil {
			t.Errorf("expected error for branch %q", branch)
		}
		if !errors.Is(err, types.ErrPermissionDenied) {
			t.Errorf("branch %q: expected ErrPermissionDenied, got %v", branch, err)
		}
	}
}

func TestGitGuard_ValidateNotProtected_Allowed(t *testing.T) {
	t.Parallel()
	g := NewGitGuard(t.TempDir(), nil)

	allowed := []string{"crux/agent-1/work", "fix/bug-123", "feature/my-feature"}
	for _, branch := range allowed {
		if err := g.ValidateNotProtected(branch); err != nil {
			t.Errorf("branch %q: unexpected error: %v", branch, err)
		}
	}
}

func TestGitGuard_PrePushCheck_CruxPrefix(t *testing.T) {
	t.Parallel()
	g := NewGitGuard(t.TempDir(), nil)

	if err := g.PrePushCheck(context.Background(), "crux/agent-1/work"); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestGitGuard_PrePushCheck_NoCruxPrefix(t *testing.T) {
	t.Parallel()
	g := NewGitGuard(t.TempDir(), nil)

	err := g.PrePushCheck(context.Background(), "feature/x")
	if err == nil {
		t.Fatal("expected error for non-crux prefix")
	}
	if !errors.Is(err, types.ErrPermissionDenied) {
		t.Errorf("expected ErrPermissionDenied, got %v", err)
	}
}

func TestGitGuard_PrePushCheck_Protected(t *testing.T) {
	t.Parallel()
	g := NewGitGuard(t.TempDir(), nil)

	err := g.PrePushCheck(context.Background(), "main")
	if err == nil {
		t.Fatal("expected error for protected branch")
	}
	if !errors.Is(err, types.ErrPermissionDenied) {
		t.Errorf("expected ErrPermissionDenied, got %v", err)
	}
}

// --- Git repo tests ---

func TestGitGuard_EnsureFeatureBranch_Creates(t *testing.T) {
	t.Parallel()
	dir := initTestGitRepo(t)
	g := NewGitGuard(dir, nil)

	branch, err := g.EnsureFeatureBranch(context.Background(), "agent-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "crux/agent-1/work" {
		t.Errorf("branch = %q, want %q", branch, "crux/agent-1/work")
	}

	// Verify we're on the correct branch.
	actual, err := g.currentBranch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if actual != "crux/agent-1/work" {
		t.Errorf("current branch = %q, want %q", actual, "crux/agent-1/work")
	}
}

func TestGitGuard_EnsureFeatureBranch_ChecksOut(t *testing.T) {
	t.Parallel()
	dir := initTestGitRepo(t)
	g := NewGitGuard(dir, nil)

	// Create the branch first.
	_, err := g.EnsureFeatureBranch(context.Background(), "agent-2")
	if err != nil {
		t.Fatal(err)
	}

	// Go back to main.
	runCmd(t, dir, "git", "checkout", "master")

	// EnsureFeatureBranch should check out the existing branch.
	branch, err := g.EnsureFeatureBranch(context.Background(), "agent-2")
	if err != nil {
		t.Fatalf("checkout existing: %v", err)
	}
	if branch != "crux/agent-2/work" {
		t.Errorf("branch = %q, want %q", branch, "crux/agent-2/work")
	}

	actual, err := g.currentBranch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if actual != "crux/agent-2/work" {
		t.Errorf("current branch = %q, want %q", actual, "crux/agent-2/work")
	}
}

func TestGitGuard_SafeCommit_OnFeatureBranch(t *testing.T) {
	t.Parallel()
	dir := initTestGitRepo(t)
	g := NewGitGuard(dir, nil)

	// Switch to a feature branch.
	_, err := g.EnsureFeatureBranch(context.Background(), "agent-1")
	if err != nil {
		t.Fatal(err)
	}

	// Create a file to commit.
	writeFile(t, filepath.Join(dir, "new.go"), "package main")

	hash, err := g.SafeCommit(context.Background(), "agent-1", "add new.go", []string{"new.go"})
	if err != nil {
		t.Fatalf("safe commit: %v", err)
	}
	if len(hash) < 7 {
		t.Errorf("expected git hash, got %q", hash)
	}
}

func TestGitGuard_SafeCommit_NotOnFeatureBranch(t *testing.T) {
	t.Parallel()
	dir := initTestGitRepo(t)
	g := NewGitGuard(dir, nil)

	// We're on main/master — commit should be denied.
	writeFile(t, filepath.Join(dir, "new.go"), "package main")
	_, err := g.SafeCommit(context.Background(), "agent-1", "add new.go", []string{"new.go"})
	if err == nil {
		t.Fatal("expected error on non-feature branch")
	}
	if !errors.Is(err, types.ErrPermissionDenied) {
		t.Errorf("expected ErrPermissionDenied, got %v", err)
	}
}

func TestGitGuard_SafeCommit_EmptyFiles(t *testing.T) {
	t.Parallel()
	dir := initTestGitRepo(t)
	g := NewGitGuard(dir, nil)

	_, err := g.SafeCommit(context.Background(), "agent-1", "empty", nil)
	if err == nil {
		t.Fatal("expected error for empty files")
	}
	if !errors.Is(err, types.ErrPermissionDenied) {
		t.Errorf("expected ErrPermissionDenied, got %v", err)
	}
}
