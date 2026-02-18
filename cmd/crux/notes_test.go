package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/roygabriel/crux/internal/memory/worknotes"
)

func TestNotesShow_Render(t *testing.T) {
	dir := t.TempDir()
	log := slog.Default()

	mgr := worknotes.NewManager(dir, log)
	if err := mgr.Init("1A", "Config Layer"); err != nil {
		t.Fatal(err)
	}

	notes, err := mgr.Read("1A")
	if err != nil {
		t.Fatal(err)
	}

	rendered := mgr.Render(notes)
	if !strings.Contains(rendered, "Phase 1A") {
		t.Error("rendered notes missing phase ID")
	}
	if !strings.Contains(rendered, "Config Layer") {
		t.Error("rendered notes missing phase name")
	}
	if !strings.Contains(rendered, "## Status") {
		t.Error("rendered notes missing Status section")
	}
}

func TestNotesList_MixedStatus(t *testing.T) {
	dir := t.TempDir()
	log := slog.Default()

	mgr := worknotes.NewManager(dir, log)

	// Create notes for phase 1A only.
	if err := mgr.Init("1A", "Config Layer"); err != nil {
		t.Fatal(err)
	}

	// 1A should be readable.
	notes, err := mgr.Read("1A")
	if err != nil {
		t.Fatal(err)
	}
	if notes.Status != "Not started" {
		t.Errorf("1A status = %q, want %q", notes.Status, "Not started")
	}

	// 2A should fail (no notes).
	_, err = mgr.Read("2A")
	if err == nil {
		t.Error("expected error reading 2A notes, got nil")
	}
}

func TestNotesEdit_InitCreatesFile(t *testing.T) {
	dir := t.TempDir()
	log := slog.Default()

	mgr := worknotes.NewManager(dir, log)
	if err := mgr.Init("5A", "New Phase"); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "PHASE5A.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Init did not create file: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "New Phase") {
		t.Error("file missing phase name")
	}
}
