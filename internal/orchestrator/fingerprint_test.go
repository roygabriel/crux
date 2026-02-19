package orchestrator

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/roygabriel/crux/internal/phase"
)

func TestProgressFingerprintScore(t *testing.T) {
	fp := &ProgressFingerprint{
		FilesExistCount: 4,
		FilesExpected:   4,
		TestPassCount:   3,
		TestTotalCount:  3,
		GitDiffHash:     "x",
	}
	if got := fp.Score(); got != 1.0 {
		t.Fatalf("Score() = %v, want 1.0", got)
	}
}

func TestProgressFingerprintScore_NoFilesNoTestsNoDiff(t *testing.T) {
	fp := &ProgressFingerprint{
		FilesExistCount: 0,
		FilesExpected:   0,
		TestPassCount:   0,
		TestTotalCount:  0,
		GitDiffHash:     hashString(""),
	}
	if got := fp.Score(); got != 0.8 {
		t.Fatalf("Score() = %v, want 0.8", got)
	}
}

func TestProgressFingerprintScore_Partial(t *testing.T) {
	fp := &ProgressFingerprint{
		FilesExistCount: 2,
		FilesExpected:   4,
		TestPassCount:   3,
		TestTotalCount:  5,
		GitDiffHash:     "x",
	}
	// files 0.2 + tests 0.24 + git 0.2 = 0.64
	if got := fp.Score(); got != 0.64 {
		t.Fatalf("Score() = %v, want 0.64", got)
	}
}

func TestProgressFingerprintScore_ZeroExpectedFiles(t *testing.T) {
	fp := &ProgressFingerprint{
		FilesExpected:  0,
		TestPassCount:  2,
		TestTotalCount: 4,
		GitDiffHash:    "x",
	}
	// files 0.4 + tests 0.2 + git 0.2
	if got := fp.Score(); got != 0.8 {
		t.Fatalf("Score() = %v, want 0.8", got)
	}
}

func TestProgressFingerprintSameAs(t *testing.T) {
	a := &ProgressFingerprint{FilesExistCount: 1, GitDiffHash: "x", TestPassCount: 1, LastGateHash: "g"}
	b := &ProgressFingerprint{FilesExistCount: 1, GitDiffHash: "x", TestPassCount: 1, LastGateHash: "g"}
	if !a.SameAs(b) {
		t.Fatal("SameAs should be true for identical semantic fields")
	}
	b.GitDiffHash = "y"
	if a.SameAs(b) {
		t.Fatal("SameAs should be false for different git hash")
	}
}

func TestComputeFingerprint(t *testing.T) {
	root := t.TempDir()
	runCmd(t, root, "git", "init")
	runCmd(t, root, "git", "config", "user.email", "test@example.com")
	runCmd(t, root, "git", "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	spec := phase.PhaseSpec{ID: "1A", FilesNew: []string{"a.go", "b.go"}}
	fp, err := ComputeFingerprint(context.Background(), root, spec, 1, 1, nil)
	if err != nil {
		t.Fatalf("ComputeFingerprint: %v", err)
	}
	if fp.FilesExpected != 2 || fp.FilesExistCount != 1 {
		t.Fatalf("unexpected file counts: %#v", fp)
	}
}

func runCmd(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, string(out))
	}
}
