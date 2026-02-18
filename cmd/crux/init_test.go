package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureGitignoreEntries_AddsMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")

	if err := os.WriteFile(path, []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := []string{".crux/memory.db", ".crux/vectors/"}
	added, err := ensureGitignoreEntries(path, entries)
	if err != nil {
		t.Fatalf("ensureGitignoreEntries: %v", err)
	}
	if added != 2 {
		t.Errorf("added = %d, want 2", added)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	for _, entry := range entries {
		if !strings.Contains(content, entry) {
			t.Errorf("gitignore missing entry %q", entry)
		}
	}

	if !strings.Contains(content, "*.log") {
		t.Error("original entry *.log was removed")
	}
}

func TestEnsureGitignoreEntries_SkipsExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")

	if err := os.WriteFile(path, []byte(".crux/memory.db\n.crux/vectors/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := []string{".crux/memory.db", ".crux/vectors/"}
	added, err := ensureGitignoreEntries(path, entries)
	if err != nil {
		t.Fatalf("ensureGitignoreEntries: %v", err)
	}
	if added != 0 {
		t.Errorf("added = %d, want 0", added)
	}
}

func TestEnsureGitignoreEntries_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")

	entries := []string{".crux/memory.db", ".crux/audit.log"}
	added, err := ensureGitignoreEntries(path, entries)
	if err != nil {
		t.Fatalf("ensureGitignoreEntries: %v", err)
	}
	if added != 2 {
		t.Errorf("added = %d, want 2", added)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	for _, entry := range entries {
		if !strings.Contains(content, entry) {
			t.Errorf("gitignore missing entry %q", entry)
		}
	}
}

func TestEnsureGitignoreEntries_PartialMatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")

	if err := os.WriteFile(path, []byte(".crux/memory.db\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := []string{".crux/memory.db", ".crux/vectors/", ".crux/audit.log"}
	added, err := ensureGitignoreEntries(path, entries)
	if err != nil {
		t.Fatalf("ensureGitignoreEntries: %v", err)
	}
	if added != 2 {
		t.Errorf("added = %d, want 2", added)
	}
}

func TestEnsureGitignoreEntries_NoTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")

	// Write without trailing newline.
	if err := os.WriteFile(path, []byte("*.log"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := []string{".crux/memory.db"}
	added, err := ensureGitignoreEntries(path, entries)
	if err != nil {
		t.Fatalf("ensureGitignoreEntries: %v", err)
	}
	if added != 1 {
		t.Errorf("added = %d, want 1", added)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Verify the new entry is on its own line.
	lines := strings.Split(content, "\n")
	found := false
	for _, line := range lines {
		if strings.TrimSpace(line) == ".crux/memory.db" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("entry not on its own line; content:\n%s", content)
	}
}

func TestGitignoreContains(t *testing.T) {
	content := ".crux/memory.db\n.crux/vectors/\n# comment\n"

	if !gitignoreContains(content, ".crux/memory.db") {
		t.Error("should contain .crux/memory.db")
	}
	if !gitignoreContains(content, ".crux/vectors/") {
		t.Error("should contain .crux/vectors/")
	}
	if gitignoreContains(content, ".crux/audit.log") {
		t.Error("should not contain .crux/audit.log")
	}
	if gitignoreContains(content, "# comment") {
		// Comments are treated as entries — this is intentional since we
		// match exact trimmed lines.
	}
}

func TestCopyTemplates_CopiesWhenMissing(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(srcDir, "a.md"), []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "b.md"), []byte("bravo"), 0o644); err != nil {
		t.Fatal(err)
	}

	created, skipped, err := copyTemplates(srcDir, dstDir, false)
	if err != nil {
		t.Fatalf("copyTemplates: %v", err)
	}
	if len(created) != 2 {
		t.Errorf("created = %v, want 2 items", created)
	}
	if len(skipped) != 0 {
		t.Errorf("skipped = %v, want empty", skipped)
	}

	data, err := os.ReadFile(filepath.Join(dstDir, "a.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "alpha" {
		t.Errorf("a.md content = %q, want %q", string(data), "alpha")
	}
}

func TestCopyTemplates_SkipsExisting(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(srcDir, "a.md"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Pre-existing file in destination.
	if err := os.WriteFile(filepath.Join(dstDir, "a.md"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	created, skipped, err := copyTemplates(srcDir, dstDir, false)
	if err != nil {
		t.Fatalf("copyTemplates: %v", err)
	}
	if len(created) != 0 {
		t.Errorf("created = %v, want empty", created)
	}
	if len(skipped) != 1 {
		t.Errorf("skipped = %v, want 1 item", skipped)
	}

	// Verify original content is preserved.
	data, err := os.ReadFile(filepath.Join(dstDir, "a.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "old" {
		t.Errorf("a.md content = %q, want %q (should not be overwritten)", string(data), "old")
	}
}

func TestCopyTemplates_ForceOverwrites(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(srcDir, "a.md"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Pre-existing file in destination.
	if err := os.WriteFile(filepath.Join(dstDir, "a.md"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	created, skipped, err := copyTemplates(srcDir, dstDir, true)
	if err != nil {
		t.Fatalf("copyTemplates: %v", err)
	}
	if len(created) != 1 {
		t.Errorf("created = %v, want 1 item", created)
	}
	if len(skipped) != 0 {
		t.Errorf("skipped = %v, want empty", skipped)
	}

	// Verify content was overwritten.
	data, err := os.ReadFile(filepath.Join(dstDir, "a.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Errorf("a.md content = %q, want %q (should be overwritten)", string(data), "new")
	}
}

func TestCopyTemplates_MissingSrcDir(t *testing.T) {
	dstDir := t.TempDir()
	nonExistent := filepath.Join(t.TempDir(), "does-not-exist")

	created, skipped, err := copyTemplates(nonExistent, dstDir, false)
	if err != nil {
		t.Fatalf("copyTemplates with missing src should not error: %v", err)
	}
	if len(created) != 0 || len(skipped) != 0 {
		t.Errorf("expected empty results for missing src dir")
	}
}

func TestCopyTemplates_SkipsSubdirectories(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(srcDir, "a.md"), []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	created, _, err := copyTemplates(srcDir, dstDir, false)
	if err != nil {
		t.Fatalf("copyTemplates: %v", err)
	}
	if len(created) != 1 {
		t.Errorf("created = %v, want 1 item (subdirs should be skipped)", created)
	}
}
