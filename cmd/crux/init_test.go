package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/roygabriel/crux/internal/scaffold"
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

func TestWriteEmbeddedFSSkip_SkipsSpecifiedPaths(t *testing.T) {
	dstDir := t.TempDir()
	fsys := fstest.MapFS{
		"README.md":   {Data: []byte("readme")},
		"config.yaml": {Data: []byte("config")},
		"main.go":     {Data: []byte("package main")},
	}

	created, skipped, err := writeEmbeddedFSSkip(fsys, dstDir, false, "config.yaml")
	if err != nil {
		t.Fatalf("writeEmbeddedFSSkip: %v", err)
	}
	if len(created) != 2 {
		t.Errorf("created = %d, want 2", len(created))
	}
	if len(skipped) != 0 {
		t.Errorf("skipped = %d, want 0", len(skipped))
	}

	// config.yaml should not exist in destination.
	if _, err := os.Stat(filepath.Join(dstDir, "config.yaml")); err == nil {
		t.Error("config.yaml should have been skipped")
	}
	// Other files should exist.
	if _, err := os.Stat(filepath.Join(dstDir, "README.md")); err != nil {
		t.Errorf("README.md should exist: %v", err)
	}
}

func TestLoadExample_UnknownName(t *testing.T) {
	_, err := loadExample("nonexistent")
	if err == nil {
		t.Fatal("loadExample should return error for unknown name")
	}
	if !strings.Contains(err.Error(), "unknown example") {
		t.Errorf("error = %q, want it to contain 'unknown example'", err.Error())
	}
	if !strings.Contains(err.Error(), "httpapi") {
		t.Errorf("error = %q, want it to list available examples", err.Error())
	}
}

func TestLoadExample_ValidName(t *testing.T) {
	fsys, err := loadExample("httpapi")
	if err != nil {
		t.Fatalf("loadExample(httpapi): %v", err)
	}
	if fsys == nil {
		t.Fatal("loadExample(httpapi) returned nil FS")
	}
}

func TestWriteEmbeddedFS_CreatesFiles(t *testing.T) {
	dstDir := t.TempDir()
	fsys := fstest.MapFS{
		"README.md":        {Data: []byte("# Example")},
		"config.yaml":      {Data: []byte("project: test")},
		"docs/phases/P.md": {Data: []byte("# Phase")},
	}

	created, skipped, err := writeEmbeddedFS(fsys, dstDir, false)
	if err != nil {
		t.Fatalf("writeEmbeddedFS: %v", err)
	}
	if len(created) != 3 {
		t.Errorf("created = %d, want 3", len(created))
	}
	if len(skipped) != 0 {
		t.Errorf("skipped = %d, want 0", len(skipped))
	}

	for _, name := range []string{"README.md", "config.yaml", "docs/phases/P.md"} {
		if _, err := os.Stat(filepath.Join(dstDir, name)); err != nil {
			t.Errorf("file %s not created: %v", name, err)
		}
	}
}

func TestWriteEmbeddedFS_SkipsExisting(t *testing.T) {
	dstDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dstDir, "README.md"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	fsys := fstest.MapFS{
		"README.md":   {Data: []byte("new")},
		"config.yaml": {Data: []byte("project: test")},
	}

	created, skipped, err := writeEmbeddedFS(fsys, dstDir, false)
	if err != nil {
		t.Fatalf("writeEmbeddedFS: %v", err)
	}
	if len(created) != 1 {
		t.Errorf("created = %d, want 1", len(created))
	}
	if len(skipped) != 1 {
		t.Errorf("skipped = %d, want 1", len(skipped))
	}

	data, _ := os.ReadFile(filepath.Join(dstDir, "README.md"))
	if string(data) != "old" {
		t.Errorf("README.md content = %q, want %q (should not be overwritten)", string(data), "old")
	}
}

func TestWriteEmbeddedFS_ForceOverwrites(t *testing.T) {
	dstDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dstDir, "README.md"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	fsys := fstest.MapFS{
		"README.md": {Data: []byte("new")},
	}

	created, skipped, err := writeEmbeddedFS(fsys, dstDir, true)
	if err != nil {
		t.Fatalf("writeEmbeddedFS: %v", err)
	}
	if len(created) != 1 {
		t.Errorf("created = %d, want 1", len(created))
	}
	if len(skipped) != 0 {
		t.Errorf("skipped = %d, want 0", len(skipped))
	}

	data, _ := os.ReadFile(filepath.Join(dstDir, "README.md"))
	if string(data) != "new" {
		t.Errorf("README.md content = %q, want %q (should be overwritten)", string(data), "new")
	}
}

func TestWriteEmbeddedFS_CreatesSubdirs(t *testing.T) {
	dstDir := t.TempDir()
	fsys := fstest.MapFS{
		"a/b/c/file.txt": {Data: []byte("deep")},
	}

	created, _, err := writeEmbeddedFS(fsys, dstDir, false)
	if err != nil {
		t.Fatalf("writeEmbeddedFS: %v", err)
	}
	if len(created) != 1 {
		t.Errorf("created = %d, want 1", len(created))
	}

	data, err := os.ReadFile(filepath.Join(dstDir, "a", "b", "c", "file.txt"))
	if err != nil {
		t.Fatalf("nested file not created: %v", err)
	}
	if string(data) != "deep" {
		t.Errorf("content = %q, want %q", string(data), "deep")
	}
}

func TestSeedExample_WritesConfigToCruxDir(t *testing.T) {
	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	cruxDir := filepath.Join(dir, ".crux")
	if err := os.MkdirAll(cruxDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(cruxDir, "config.yaml")
	if err := seedExample(cfgPath, "httpapi"); err != nil {
		t.Fatalf("seedExample: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("config.yaml not written: %v", err)
	}
	if len(data) == 0 {
		t.Error("config.yaml is empty")
	}

	// Verify example-specific phase files were also written.
	if _, err := os.Stat(filepath.Join(dir, "docs", "phases")); err != nil {
		t.Errorf("docs/phases/ not created: %v", err)
	}
}

func TestWriteEmbeddedFSSkip_MultipleSkipPaths(t *testing.T) {
	dstDir := t.TempDir()
	fsys := fstest.MapFS{
		"a.txt":    {Data: []byte("a")},
		"b.txt":    {Data: []byte("b")},
		"c.txt":    {Data: []byte("c")},
		"keep.txt": {Data: []byte("keep")},
	}

	created, skipped, err := writeEmbeddedFSSkip(fsys, dstDir, false, "a.txt", "b.txt")
	if err != nil {
		t.Fatalf("writeEmbeddedFSSkip: %v", err)
	}
	if len(created) != 2 {
		t.Errorf("created = %d, want 2; got %v", len(created), created)
	}
	if len(skipped) != 0 {
		t.Errorf("skipped = %d, want 0", len(skipped))
	}

	for _, name := range []string{"a.txt", "b.txt"} {
		if _, err := os.Stat(filepath.Join(dstDir, name)); err == nil {
			t.Errorf("%s should have been skipped but exists", name)
		}
	}
	for _, name := range []string{"c.txt", "keep.txt"} {
		if _, err := os.Stat(filepath.Join(dstDir, name)); err != nil {
			t.Errorf("%s should exist: %v", name, err)
		}
	}
}

func TestRunDefaultInit_CreatesExpectedFiles(t *testing.T) {
	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	// Reset flags to non-interactive defaults.
	forceFlag = false
	exampleFlag = ""
	noInteractive = true

	cfgPath := filepath.Join(".crux", "config.yaml")

	// Create required directories (normally done by runInit before calling runDefaultInit).
	for _, d := range []string{".crux", "docs/phases", "work-notes"} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if err := runDefaultInit(cfgPath); err != nil {
		t.Fatalf("runDefaultInit: %v", err)
	}

	// Config should exist and match embedded default.
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("config.yaml not created: %v", err)
	}
	if string(data) != string(scaffold.DefaultConfig()) {
		t.Error("config.yaml content does not match embedded default")
	}

	// Templates should have been written.
	tplFS, err := scaffold.TemplatesFS()
	if err != nil {
		t.Fatal(err)
	}
	err = fs.WalkDir(tplFS, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		tplPath := filepath.Join("templates", path)
		if _, statErr := os.Stat(tplPath); statErr != nil {
			t.Errorf("template file %s not created: %v", tplPath, statErr)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking templates: %v", err)
	}

	// .gitignore should exist.
	if _, err := os.Stat(".gitignore"); err != nil {
		t.Errorf(".gitignore not created: %v", err)
	}
}
