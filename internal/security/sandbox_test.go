package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheck_PathWithinProject(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "src", "main.go"), "package main")

	sb, err := NewSandbox(root, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := sb.Check(filepath.Join(root, "src", "main.go"), OpRead); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestCheck_PathOutsideProject(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	sb, err := NewSandbox(root, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = sb.Check(filepath.Join(root, "..", "..", "..", "etc", "passwd"), OpRead)
	if err != ErrPathOutsideProject {
		t.Errorf("expected ErrPathOutsideProject, got %v", err)
	}
}

func TestCheck_PathTraversalEscape(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "src", "app.go"), "package app")

	sb, err := NewSandbox(root, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Path that looks like it's under root but traverses out.
	err = sb.Check(filepath.Join(root, "src", "..", "..", "etc", "passwd"), OpRead)
	if err != ErrPathOutsideProject {
		t.Errorf("expected ErrPathOutsideProject, got %v", err)
	}
}

func TestCheck_DeniedFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".env"), "SECRET=x")

	sb, err := NewSandbox(root, nil, []string{".env"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = sb.Check(filepath.Join(root, ".env"), OpRead)
	if err != ErrPathDenied {
		t.Errorf("expected ErrPathDenied, got %v", err)
	}
}

func TestCheck_DeniedDirectoryPrefix(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".git", "config"), "[core]")

	sb, err := NewSandbox(root, nil, []string{".git/"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = sb.Check(filepath.Join(root, ".git", "config"), OpWrite)
	if err != ErrPathDenied {
		t.Errorf("expected ErrPathDenied, got %v", err)
	}
}

func TestCheck_SymlinkEscape(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "secret.txt"), "secret data")

	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	sb, err := NewSandbox(root, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = sb.Check(filepath.Join(root, "escape", "secret.txt"), OpRead)
	if err != ErrPathOutsideProject {
		t.Errorf("expected ErrPathOutsideProject, got %v", err)
	}
}

func TestCheck_EmptyAllowedPermitsAll(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.go"), "package a")
	writeFile(t, filepath.Join(root, "b", "c.go"), "package b")

	sb, err := NewSandbox(root, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, p := range []string{
		filepath.Join(root, "a.go"),
		filepath.Join(root, "b", "c.go"),
	} {
		if err := sb.Check(p, OpRead); err != nil {
			t.Errorf("path %s: expected nil, got %v", p, err)
		}
	}
}

func TestCheck_AllowedPathRestricts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "src", "main.go"), "package main")
	writeFile(t, filepath.Join(root, "vendor", "bar.go"), "package vendor")

	sb, err := NewSandbox(root, []string{"src/"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := sb.Check(filepath.Join(root, "src", "main.go"), OpRead); err != nil {
		t.Errorf("allowed path: expected nil, got %v", err)
	}

	err = sb.Check(filepath.Join(root, "vendor", "bar.go"), OpRead)
	if err != ErrPathNotAllowed {
		t.Errorf("expected ErrPathNotAllowed, got %v", err)
	}
}

func TestCheckPaths_FirstFailure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "ok.go"), "package ok")

	sb, err := NewSandbox(root, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	paths := []string{
		filepath.Join(root, "ok.go"),
		filepath.Join(root, "..", "..", "etc", "passwd"),
		filepath.Join(root, "also-ok.go"),
	}

	err = sb.CheckPaths(paths, OpRead)
	if err != ErrPathOutsideProject {
		t.Errorf("expected ErrPathOutsideProject, got %v", err)
	}
}

func TestCheckPaths_AllValid(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.go"), "package a")
	writeFile(t, filepath.Join(root, "b.go"), "package b")

	sb, err := NewSandbox(root, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	paths := []string{
		filepath.Join(root, "a.go"),
		filepath.Join(root, "b.go"),
	}
	if err := sb.CheckPaths(paths, OpRead); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestNewSandbox_InvalidRoot(t *testing.T) {
	t.Parallel()

	_, err := NewSandbox("/nonexistent/path/does/not/exist", nil, nil, nil)
	if err == nil {
		t.Error("expected error for non-existent root")
	}
}

// writeFile creates parent directories and writes content to path.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
