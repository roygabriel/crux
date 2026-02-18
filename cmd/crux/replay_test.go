package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/roygabriel/crux/internal/memory/session"
)

func TestSessionList(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sessions")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	m := session.NewManager(dir, nil, logger)

	// List on nonexistent dir.
	sessions, err := m.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}

	// Create sessions.
	sc1, err := m.Start()
	if err != nil {
		t.Fatal(err)
	}
	sc2, err := m.Start()
	if err != nil {
		t.Fatal(err)
	}

	sessions, err = m.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	// Both IDs should be present.
	ids := map[string]bool{}
	for _, s := range sessions {
		ids[s.ID] = true
	}
	if !ids[sc1.ID] || !ids[sc2.ID] {
		t.Errorf("missing expected session IDs")
	}
}

func TestSessionList_CorruptFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sessions")
	os.MkdirAll(dir, 0o755)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	m := session.NewManager(dir, nil, logger)

	// Valid session.
	sc, err := m.Start()
	if err != nil {
		t.Fatal(err)
	}

	// Corrupt file.
	os.WriteFile(filepath.Join(dir, "20250101T000000Z_bad.json"), []byte("not json"), 0o644)

	sessions, err := m.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (corrupt skipped), got %d", len(sessions))
	}
	if sessions[0].ID != sc.ID {
		t.Errorf("expected ID %q, got %q", sc.ID, sessions[0].ID)
	}
}
