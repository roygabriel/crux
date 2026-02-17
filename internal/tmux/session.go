package tmux

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

// ErrInvalidSessionName is returned when a session name fails validation.
var ErrInvalidSessionName = errors.New("invalid session name")

// sessionNameRe matches valid tmux session names: alphanumeric and hyphens,
// must start and end with an alphanumeric character.
var sessionNameRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)

// validateSessionName checks that name is a valid tmux session name.
func validateSessionName(name string) error {
	if !sessionNameRe.MatchString(name) {
		return fmt.Errorf("%w: %q", ErrInvalidSessionName, name)
	}
	return nil
}

// SessionManager manages tmux sessions via a Commander.
type SessionManager struct {
	cmd    Commander
	logger *slog.Logger
}

// NewSessionManager creates a SessionManager backed by the given Commander.
func NewSessionManager(cmd Commander, logger *slog.Logger) *SessionManager {
	return &SessionManager{
		cmd:    cmd,
		logger: logger,
	}
}

// Create creates a new detached tmux session with the given name.
func (m *SessionManager) Create(ctx context.Context, name string) error {
	if err := validateSessionName(name); err != nil {
		return err
	}

	_, err := m.cmd.Run(ctx, "new-session", "-d", "-s", name)
	if err != nil {
		return fmt.Errorf("create session %q: %w", name, err)
	}

	m.logger.Info("created tmux session", "session", name)
	return nil
}

// Exists checks whether a tmux session with the given name exists.
// It returns false, nil if the Commander returns an error (tmux has-session
// exits non-zero for both "not found" and "server not running"). Use List
// to verify server connectivity.
func (m *SessionManager) Exists(ctx context.Context, name string) (bool, error) {
	if err := validateSessionName(name); err != nil {
		return false, err
	}

	_, err := m.cmd.Run(ctx, "has-session", "-t", name)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// List returns the names of all active tmux sessions.
func (m *SessionManager) List(ctx context.Context) ([]string, error) {
	out, err := m.cmd.Run(ctx, "list-sessions", "-F", "#{session_name}")
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	if out == "" {
		return nil, nil
	}

	var sessions []string
	for _, line := range strings.Split(out, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			sessions = append(sessions, s)
		}
	}
	return sessions, nil
}

// Kill destroys the named tmux session.
func (m *SessionManager) Kill(ctx context.Context, name string) error {
	if err := validateSessionName(name); err != nil {
		return err
	}

	_, err := m.cmd.Run(ctx, "kill-session", "-t", name)
	if err != nil {
		return fmt.Errorf("kill session %q: %w", name, err)
	}

	m.logger.Info("killed tmux session", "session", name)
	return nil
}
