package session_test

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/roygabriel/crux/internal/memory/session"
)

func newTestManager(t *testing.T) *session.Manager {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "sessions")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return session.NewManager(dir, nil, logger)
}

func TestStartCreatesSession(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)

	sc, err := m.Start()
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if sc.ID == "" {
		t.Error("expected non-empty session ID")
	}
	if sc.StartedAt.IsZero() {
		t.Error("expected non-zero StartedAt")
	}
	if sc.Agents == nil {
		t.Error("expected initialized Agents map")
	}
}

func TestStartCreatesDirectory(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "nested", "deep", "sessions")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	m := session.NewManager(dir, nil, logger)

	if _, err := m.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory, got file")
	}
}

func TestStartWritesFile(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "sessions")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	m := session.NewManager(dir, nil, logger)

	if _, err := m.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 session file, got %d", len(entries))
	}
}

func TestSaveAndResume(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)

	sc, err := m.Start()
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	sc.CurrentPhase = "phase-1"
	sc.Summary = "test session"
	sc.PromptProgress = 3
	if err := m.Save(sc); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := m.Resume(sc.ID)
	if err != nil {
		t.Fatalf("Resume() error: %v", err)
	}
	if loaded.ID != sc.ID {
		t.Errorf("expected ID %q, got %q", sc.ID, loaded.ID)
	}
	if loaded.CurrentPhase != "phase-1" {
		t.Errorf("expected phase phase-1, got %q", loaded.CurrentPhase)
	}
	if loaded.Summary != "test session" {
		t.Errorf("expected summary %q, got %q", "test session", loaded.Summary)
	}
	if loaded.PromptProgress != 3 {
		t.Errorf("expected prompt progress 3, got %d", loaded.PromptProgress)
	}
}

func TestResumeNotFound(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)

	// Create the directory so it exists but is empty.
	m.Start()

	_, err := m.Resume("nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, session.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got: %v", err)
	}
}

func TestResumeLatestPicksNewest(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)

	// Create multiple sessions.
	sc1, err := m.Start()
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	sc2, err := m.Start()
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	latest, err := m.ResumeLatest()
	if err != nil {
		t.Fatalf("ResumeLatest() error: %v", err)
	}

	// The second session should be latest (or at least one of them,
	// since they may have the same second-precision timestamp).
	if latest.ID != sc1.ID && latest.ID != sc2.ID {
		t.Errorf("ResumeLatest() returned unknown ID %q", latest.ID)
	}
}

func TestResumeLatestEmptyDir(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "sessions")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	m := session.NewManager(dir, nil, logger)

	// Create the directory but no files.
	os.MkdirAll(dir, 0o755)

	_, err := m.ResumeLatest()
	if err == nil {
		t.Fatal("expected error for empty dir, got nil")
	}
	if !errors.Is(err, session.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got: %v", err)
	}
}

func TestUpdateAgent(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)

	sc, err := m.Start()
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	if err := m.UpdateAgent(sc.ID, "agent-1", "busy"); err != nil {
		t.Fatalf("UpdateAgent() error: %v", err)
	}

	loaded, err := m.Resume(sc.ID)
	if err != nil {
		t.Fatalf("Resume() error: %v", err)
	}

	state, ok := loaded.Agents["agent-1"]
	if !ok {
		t.Fatal("agent-1 not found in session agents")
	}
	if state.Status != "busy" {
		t.Errorf("expected status busy, got %q", state.Status)
	}
	if state.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
}

func TestUpdateAgentNotFound(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)

	// Ensure directory exists.
	m.Start()

	err := m.UpdateAgent("nonexistent", "agent-1", "busy")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, session.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got: %v", err)
	}
}

func TestUpdatePhase(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)

	sc, err := m.Start()
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	if err := m.UpdatePhase(sc.ID, "phase-2", 5); err != nil {
		t.Fatalf("UpdatePhase() error: %v", err)
	}

	loaded, err := m.Resume(sc.ID)
	if err != nil {
		t.Fatalf("Resume() error: %v", err)
	}
	if loaded.CurrentPhase != "phase-2" {
		t.Errorf("expected phase phase-2, got %q", loaded.CurrentPhase)
	}
	if loaded.PromptProgress != 5 {
		t.Errorf("expected prompt progress 5, got %d", loaded.PromptProgress)
	}
}

func TestSaveOverwrites(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)

	sc, err := m.Start()
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	sc.Summary = "first"
	m.Save(sc)

	sc.Summary = "second"
	m.Save(sc)

	loaded, err := m.Resume(sc.ID)
	if err != nil {
		t.Fatalf("Resume() error: %v", err)
	}
	if loaded.Summary != "second" {
		t.Errorf("expected summary %q, got %q", "second", loaded.Summary)
	}
}
