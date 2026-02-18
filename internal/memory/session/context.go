// Package session provides session context persistence using JSON files.
// Each session is stored as a timestamped JSON file with lexicographic
// ordering matching chronological ordering.
package session

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/roygabriel/crux/internal/memory/store"
)

// ErrSessionNotFound indicates no session file matches the requested ID.
var ErrSessionNotFound = errors.New("session not found")

// fileTimeFmt is the time format prefix for session filenames.
// Lexicographic sort = chronological sort.
const fileTimeFmt = "20060102T150405Z"

// AgentState tracks the current state of an agent within a session.
type AgentState struct {
	// Status is the agent's current operational status.
	Status string `json:"status"`
	// CurrentTask describes what the agent is working on.
	CurrentTask string `json:"current_task"`
	// UpdatedAt is when this agent state was last modified.
	UpdatedAt time.Time `json:"updated_at"`
}

// SessionContext holds the state for a single orchestration session.
type SessionContext struct {
	// ID is a short hex identifier for this session.
	ID string `json:"id"`
	// CurrentPhase is the active phase ID.
	CurrentPhase string `json:"current_phase"`
	// Summary describes the session's purpose or current state.
	Summary string `json:"summary"`
	// StartedAt is when the session was created.
	StartedAt time.Time `json:"started_at"`
	// Agents maps agent IDs to their current state.
	Agents map[string]AgentState `json:"agents"`
	// PromptProgress is the current prompt number within the active phase.
	PromptProgress int `json:"prompt_progress"`
}

// Manager handles session creation, persistence, and retrieval.
type Manager struct {
	sessionsDir string
	store       *store.Store
	logger      *slog.Logger
}

// NewManager creates a Manager that persists sessions to the given directory.
// The store parameter is optional (nil-safe) — reserved for future event recording.
func NewManager(dir string, store *store.Store, logger *slog.Logger) *Manager {
	return &Manager{
		sessionsDir: dir,
		store:       store,
		logger:      logger,
	}
}

// Start creates a new session with a generated ID and saves it to disk.
func (m *Manager) Start() (*SessionContext, error) {
	if err := os.MkdirAll(m.sessionsDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating sessions directory: %w", err)
	}

	id, err := generateHexID()
	if err != nil {
		return nil, fmt.Errorf("generating session ID: %w", err)
	}

	sc := &SessionContext{
		ID:        id,
		StartedAt: time.Now().UTC(),
		Agents:    make(map[string]AgentState),
	}

	if err := m.Save(sc); err != nil {
		return nil, err
	}

	m.logger.Info("session started", "id", id)
	return sc, nil
}

// Resume loads a session by its ID from disk.
func (m *Manager) Resume(id string) (*SessionContext, error) {
	entries, err := os.ReadDir(m.sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrSessionNotFound, id)
		}
		return nil, fmt.Errorf("reading sessions directory: %w", err)
	}

	suffix := "_" + id + ".json"
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), suffix) {
			return m.loadFile(filepath.Join(m.sessionsDir, entry.Name()))
		}
	}

	return nil, fmt.Errorf("%w: %s", ErrSessionNotFound, id)
}

// ResumeLatest loads the most recent session by lexicographic filename ordering.
func (m *Manager) ResumeLatest() (*SessionContext, error) {
	entries, err := os.ReadDir(m.sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: no sessions directory", ErrSessionNotFound)
		}
		return nil, fmt.Errorf("reading sessions directory: %w", err)
	}

	// Filter to JSON files only.
	var jsonFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			jsonFiles = append(jsonFiles, entry.Name())
		}
	}

	if len(jsonFiles) == 0 {
		return nil, fmt.Errorf("%w: no session files", ErrSessionNotFound)
	}

	sort.Strings(jsonFiles)
	latest := jsonFiles[len(jsonFiles)-1]
	return m.loadFile(filepath.Join(m.sessionsDir, latest))
}

// Save persists a session context to disk as indented JSON.
func (m *Manager) Save(sc *SessionContext) error {
	if err := os.MkdirAll(m.sessionsDir, 0o755); err != nil {
		return fmt.Errorf("creating sessions directory: %w", err)
	}

	filename := sc.StartedAt.UTC().Format(fileTimeFmt) + "_" + sc.ID + ".json"
	path := filepath.Join(m.sessionsDir, filename)

	data, err := json.MarshalIndent(sc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling session: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing session file: %w", err)
	}

	return nil
}

// UpdateAgent updates an agent's status within a session.
func (m *Manager) UpdateAgent(sessionID, agentID, status string) error {
	sc, err := m.Resume(sessionID)
	if err != nil {
		return err
	}

	if sc.Agents == nil {
		sc.Agents = make(map[string]AgentState)
	}

	state := sc.Agents[agentID]
	state.Status = status
	state.UpdatedAt = time.Now().UTC()
	sc.Agents[agentID] = state

	return m.Save(sc)
}

// UpdatePhase updates the current phase and prompt progress for a session.
func (m *Manager) UpdatePhase(sessionID, phaseID string, promptNum int) error {
	sc, err := m.Resume(sessionID)
	if err != nil {
		return err
	}

	sc.CurrentPhase = phaseID
	sc.PromptProgress = promptNum

	return m.Save(sc)
}

// List returns all sessions in chronological order (oldest first).
// Corrupt files are skipped with a warning.
func (m *Manager) List() ([]SessionContext, error) {
	entries, err := os.ReadDir(m.sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading sessions directory: %w", err)
	}

	var jsonFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			jsonFiles = append(jsonFiles, entry.Name())
		}
	}
	sort.Strings(jsonFiles)

	sessions := make([]SessionContext, 0, len(jsonFiles))
	for _, name := range jsonFiles {
		sc, err := m.loadFile(filepath.Join(m.sessionsDir, name))
		if err != nil {
			m.logger.Warn("skipping corrupt session file", "file", name, "error", err)
			continue
		}
		sessions = append(sessions, *sc)
	}
	return sessions, nil
}

// loadFile reads and unmarshals a session file.
func (m *Manager) loadFile(path string) (*SessionContext, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading session file: %w", err)
	}

	var sc SessionContext
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil, fmt.Errorf("unmarshaling session: %w", err)
	}

	return &sc, nil
}

// generateHexID produces an 8-character hex string using crypto/rand.
func generateHexID() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%08x", b), nil
}
