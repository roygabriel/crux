package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenMan_CreatesFiles(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "man1")

	_, err := executeCommand("__gen-man", "--dir", outDir)
	if err != nil {
		t.Fatalf("__gen-man returned error: %v", err)
	}

	manFile := filepath.Join(outDir, "crux.1")
	data, err := os.ReadFile(manFile)
	if err != nil {
		t.Fatalf("man page not created at %s: %v", manFile, err)
	}
	if !strings.Contains(string(data), ".TH") {
		t.Error("man page does not contain .TH header")
	}
}

func TestGenDocs_CreatesMarkdown(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "docs")

	_, err := executeCommand("__gen-docs", "--dir", outDir)
	if err != nil {
		t.Fatalf("__gen-docs returned error: %v", err)
	}

	mdFile := filepath.Join(outDir, "crux.md")
	if _, err := os.Stat(mdFile); err != nil {
		t.Fatalf("markdown doc not created at %s: %v", mdFile, err)
	}
}

func TestGenMan_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "nested", "path", "man1")

	_, err := executeCommand("__gen-man", "--dir", outDir)
	if err != nil {
		t.Fatalf("__gen-man returned error: %v", err)
	}

	if _, err := os.Stat(outDir); err != nil {
		t.Fatalf("directory not created: %v", err)
	}
}

func TestGenDocs_HiddenExcluded(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "docs")

	_, err := executeCommand("__gen-docs", "--dir", outDir)
	if err != nil {
		t.Fatalf("__gen-docs returned error: %v", err)
	}

	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.Contains(name, "__gen-man") || strings.Contains(name, "__gen-docs") {
			t.Errorf("hidden command %q should not appear in generated docs", name)
		}
	}
}
