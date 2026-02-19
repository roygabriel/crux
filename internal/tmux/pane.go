package tmux

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
)

// PaneInfo describes a tmux pane.
type PaneInfo struct {
	// ID is the tmux pane identifier (e.g. "%0").
	ID string `json:"id"`
	// PID is the process ID of the pane's shell.
	PID int `json:"pid"`
	// Command is the current command running in the pane.
	Command string `json:"command"`
}

// PaneManager manages tmux panes within sessions via a Commander.
type PaneManager struct {
	cmd    Commander
	logger *slog.Logger
}

// NewPaneManager creates a PaneManager backed by the given Commander.
func NewPaneManager(cmd Commander, logger *slog.Logger) *PaneManager {
	return &PaneManager{
		cmd:    cmd,
		logger: logger,
	}
}

// Create splits a new pane in the given session, optionally starting in dir.
// It returns the new pane's ID.
func (m *PaneManager) Create(ctx context.Context, session string, dir string) (string, error) {
	if err := validateSessionName(session); err != nil {
		return "", err
	}

	args := []string{"split-window", "-t", session, "-P", "-F", "#{pane_id}"}
	if dir != "" {
		args = append(args, "-c", dir)
	}

	out, err := m.cmd.Run(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("create pane in session %q: %w", session, err)
	}

	m.logger.Info("created tmux pane", "session", session, "pane_id", out)
	return out, nil
}

// List returns information about all panes in the given session.
func (m *PaneManager) List(ctx context.Context, session string) ([]PaneInfo, error) {
	if err := validateSessionName(session); err != nil {
		return nil, err
	}

	out, err := m.cmd.Run(ctx, "list-panes", "-t", session, "-F", "#{pane_id}:#{pane_pid}:#{pane_current_command}")
	if err != nil {
		return nil, fmt.Errorf("list panes in session %q: %w", session, err)
	}

	return parsePaneList(out)
}

// Capture returns the visible content of a tmux pane. The lines parameter
// controls how many lines of scrollback to capture (0 captures the visible area).
func (m *PaneManager) Capture(ctx context.Context, paneID string, lines int) (string, error) {
	if paneID == "" {
		return "", fmt.Errorf("pane ID must not be empty")
	}

	// Use -J and -N for better fidelity in dashboard rendering:
	// -J joins wrapped screen lines
	// -N preserves trailing spaces (important for ASCII alignment)
	args := []string{"capture-pane", "-t", paneID, "-p", "-J", "-N"}
	if lines > 0 {
		args = append(args, "-S", fmt.Sprintf("-%d", lines))
	}

	out, err := m.cmd.Run(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("capture pane %q: %w", paneID, err)
	}
	m.logger.Debug("captured pane",
		"pane_id", paneID,
		"bytes", len(out),
		"lines", lines,
	)

	return out, nil
}

// SendKeys sends sanitized text to a tmux pane followed by Enter.
// The text is validated by SanitizeInput before sending. Callers must
// not include newlines in text; use multiple SendKeys calls instead.
func (m *PaneManager) SendKeys(ctx context.Context, paneID string, text string) error {
	if paneID == "" {
		return fmt.Errorf("pane ID must not be empty")
	}

	sanitized, err := SanitizeInput(text)
	if err != nil {
		return fmt.Errorf("send keys to pane %q: %w", paneID, err)
	}

	_, err = m.cmd.Run(ctx, "send-keys", "-t", paneID, "--", sanitized, "Enter")
	if err != nil {
		return fmt.Errorf("send keys to pane %q: %w", paneID, err)
	}

	return nil
}

// SendKeysLiteral sends text literally to a tmux pane followed by Enter.
// Unlike SendKeys, it uses the tmux -l flag to prevent interpretation of
// key names and does not reject shell metacharacters. Use this for
// delivering task content that may contain code fences and semicolons.
// Only the byte-length limit is enforced.
func (m *PaneManager) SendKeysLiteral(ctx context.Context, paneID string, text string) error {
	if paneID == "" {
		return fmt.Errorf("pane ID must not be empty")
	}
	if err := ValidateLength(text); err != nil {
		return fmt.Errorf("send literal keys to pane %q: %w", paneID, err)
	}
	// Send text literally — tmux will not interpret key names.
	_, err := m.cmd.Run(ctx, "send-keys", "-l", "-t", paneID, "--", text)
	if err != nil {
		return fmt.Errorf("send literal keys to pane %q: %w", paneID, err)
	}
	// Send Enter separately (as a key name, not literal text).
	_, err = m.cmd.Run(ctx, "send-keys", "-t", paneID, "Enter")
	if err != nil {
		return fmt.Errorf("send literal keys to pane %q (enter): %w", paneID, err)
	}
	m.logger.Debug("sent literal keys",
		"pane_id", paneID,
		"bytes", len(text),
	)
	return nil
}

// SendKeysRaw sends literal key names to a tmux pane without appending Enter
// and without sanitization. Use this for control sequences (e.g. "C-c", "Escape").
func (m *PaneManager) SendKeysRaw(ctx context.Context, paneID string, keys ...string) error {
	if paneID == "" {
		return fmt.Errorf("pane ID must not be empty")
	}

	if len(keys) == 0 {
		return fmt.Errorf("send raw keys to pane %q: no keys provided", paneID)
	}

	args := append([]string{"send-keys", "-t", paneID}, keys...)
	_, err := m.cmd.Run(ctx, args...)
	if err != nil {
		return fmt.Errorf("send raw keys to pane %q: %w", paneID, err)
	}
	m.logger.Debug("sent raw keys",
		"pane_id", paneID,
		"keys", keys,
	)

	return nil
}

// Kill destroys the specified pane.
func (m *PaneManager) Kill(ctx context.Context, paneID string) error {
	if paneID == "" {
		return fmt.Errorf("pane ID must not be empty")
	}

	_, err := m.cmd.Run(ctx, "kill-pane", "-t", paneID)
	if err != nil {
		return fmt.Errorf("kill pane %q: %w", paneID, err)
	}

	m.logger.Info("killed tmux pane", "pane_id", paneID)
	return nil
}

// Respawn kills the current process in the pane and starts command in-place.
// The pane ID remains stable, so existing watchers can continue polling.
func (m *PaneManager) Respawn(ctx context.Context, paneID, command string) error {
	if paneID == "" {
		return fmt.Errorf("pane ID must not be empty")
	}
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("command must not be empty")
	}

	if _, err := m.cmd.Run(ctx, "respawn-pane", "-k", "-t", paneID, command); err != nil {
		return fmt.Errorf("respawn pane %q: %w", paneID, err)
	}

	m.logger.Info("respawned tmux pane",
		"pane_id", paneID,
		"command", command,
	)
	return nil
}

// parsePaneList parses the output of list-panes with format
// "#{pane_id}:#{pane_pid}:#{pane_current_command}" into PaneInfo slices.
// It uses SplitN with limit 3 to preserve colons in command names.
func parsePaneList(output string) ([]PaneInfo, error) {
	if output == "" {
		return nil, nil
	}

	var panes []PaneInfo
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("parse pane line: expected 3 fields, got %d in %q", len(parts), line)
		}

		pid, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("parse pane PID %q: %w", parts[1], err)
		}

		panes = append(panes, PaneInfo{
			ID:      parts[0],
			PID:     pid,
			Command: parts[2],
		})
	}

	return panes, nil
}
