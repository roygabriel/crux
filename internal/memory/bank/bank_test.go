package bank_test

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/roygabriel/crux/internal/memory/bank"
)

func newTestBank(t *testing.T) *bank.Bank {
	t.Helper()
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return bank.NewBank(dir, logger)
}

func TestInitCreatesAllFiles(t *testing.T) {
	t.Parallel()
	b := newTestBank(t)

	if err := b.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	for _, name := range bank.TemplateFilenames() {
		content, err := b.Read(name)
		if err != nil {
			t.Errorf("Read(%q) error after Init: %v", name, err)
		}
		if content == "" {
			t.Errorf("Read(%q) returned empty content after Init", name)
		}
	}
}

func TestInitIdempotent(t *testing.T) {
	t.Parallel()
	b := newTestBank(t)

	if err := b.Init(); err != nil {
		t.Fatalf("first Init() error: %v", err)
	}

	// Modify a file.
	if err := b.Update("progress.md", "custom content"); err != nil {
		t.Fatalf("Update() error: %v", err)
	}

	// Second init should not overwrite existing files.
	if err := b.Init(); err != nil {
		t.Fatalf("second Init() error: %v", err)
	}

	content, err := b.Read("progress.md")
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if content != "custom content" {
		t.Errorf("Init() overwrote existing file; got %q", content)
	}
}

func TestReadValidFile(t *testing.T) {
	t.Parallel()
	b := newTestBank(t)
	if err := b.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	content, err := b.Read("projectBrief.md")
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if !strings.Contains(content, "# Project Brief") {
		t.Errorf("expected Project Brief header, got %q", content[:50])
	}
}

func TestReadUnknownFile(t *testing.T) {
	t.Parallel()
	b := newTestBank(t)

	_, err := b.Read("nonexistent.md")
	if err == nil {
		t.Fatal("expected error for unknown file, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestReadAll(t *testing.T) {
	t.Parallel()
	b := newTestBank(t)
	if err := b.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	all, err := b.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}

	expected := len(bank.TemplateFilenames())
	if len(all) != expected {
		t.Errorf("ReadAll() returned %d files, want %d", len(all), expected)
	}

	for _, name := range bank.TemplateFilenames() {
		if _, ok := all[name]; !ok {
			t.Errorf("ReadAll() missing file %q", name)
		}
	}
}

func TestUpdateOverwrites(t *testing.T) {
	t.Parallel()
	b := newTestBank(t)
	if err := b.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	newContent := "# Updated Content\n\nThis is new."
	if err := b.Update("activeContext.md", newContent); err != nil {
		t.Fatalf("Update() error: %v", err)
	}

	got, err := b.Read("activeContext.md")
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if got != newContent {
		t.Errorf("Update() did not overwrite; got %q, want %q", got, newContent)
	}
}

func TestUpdateUnknownFile(t *testing.T) {
	t.Parallel()
	b := newTestBank(t)

	err := b.Update("unknown.md", "content")
	if err == nil {
		t.Fatal("expected error for unknown file, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestAppendSection(t *testing.T) {
	t.Parallel()
	b := newTestBank(t)
	if err := b.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	if err := b.AppendSection("progress.md", "Completed", "- Phase 1A done"); err != nil {
		t.Fatalf("AppendSection() error: %v", err)
	}

	content, err := b.Read("progress.md")
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if !strings.Contains(content, "- Phase 1A done") {
		t.Error("AppendSection() content not found in file")
	}
}

func TestAppendSectionNotFound(t *testing.T) {
	t.Parallel()
	b := newTestBank(t)
	if err := b.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	err := b.AppendSection("progress.md", "Nonexistent", "content")
	if err == nil {
		t.Fatal("expected error for missing section, got nil")
	}
	if !strings.Contains(err.Error(), "section not found") {
		t.Errorf("expected 'section not found' error, got: %v", err)
	}
}

func TestAppendSectionPreservesOtherSections(t *testing.T) {
	t.Parallel()
	b := newTestBank(t)
	if err := b.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	if err := b.AppendSection("progress.md", "Completed", "- Phase 1A done"); err != nil {
		t.Fatalf("AppendSection() error: %v", err)
	}

	content, err := b.Read("progress.md")
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	// All original sections should still be present.
	for _, section := range []string{"Completed", "In Progress", "Upcoming", "Known Issues"} {
		if !strings.Contains(content, "## "+section) {
			t.Errorf("AppendSection() removed section %q", section)
		}
	}
}

func TestSummaryNonEmpty(t *testing.T) {
	t.Parallel()
	b := newTestBank(t)
	if err := b.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	summary, err := b.Summary()
	if err != nil {
		t.Fatalf("Summary() error: %v", err)
	}
	if summary == "" {
		t.Error("Summary() returned empty string")
	}
}

func TestSummaryIncludesAllFiles(t *testing.T) {
	t.Parallel()
	b := newTestBank(t)
	if err := b.Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	summary, err := b.Summary()
	if err != nil {
		t.Fatalf("Summary() error: %v", err)
	}

	for _, name := range bank.TemplateFilenames() {
		if !strings.Contains(summary, name) {
			t.Errorf("Summary() does not include file %q", name)
		}
	}
}

func TestTemplateFilenamesReturnsCorrectCount(t *testing.T) {
	t.Parallel()
	names := bank.TemplateFilenames()
	if len(names) != 6 {
		t.Errorf("TemplateFilenames() returned %d names, want 6", len(names))
	}
}

func TestInitCreatesDirectory(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "nested", "memory", "bank")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	b := bank.NewBank(dir, logger)

	if err := b.Init(); err != nil {
		t.Fatalf("Init() error creating nested dir: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory, got file")
	}
}
