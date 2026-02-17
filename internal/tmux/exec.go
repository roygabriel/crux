// Package tmux provides session, pane, and window management for tmux,
// along with capture-pane polling and send-keys transport.
package tmux

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// Commander abstracts tmux command execution for testability.
type Commander interface {
	// Run executes a tmux subcommand with the given arguments and returns
	// the trimmed stdout. If the command fails, the error wraps stderr.
	Run(ctx context.Context, args ...string) (string, error)
}

// RealCommander executes tmux commands via os/exec.
type RealCommander struct {
	binPath string
	logger  *slog.Logger
}

// NewRealCommander resolves the tmux binary path and returns a Commander.
// It returns an error if tmux is not found on the system PATH.
func NewRealCommander(logger *slog.Logger) (*RealCommander, error) {
	path, err := exec.LookPath("tmux")
	if err != nil {
		return nil, fmt.Errorf("find tmux binary: %w", err)
	}
	return &RealCommander{
		binPath: path,
		logger:  logger,
	}, nil
}

// Run executes a tmux subcommand, captures stdout, and returns it with
// trailing whitespace trimmed. If the command exits non-zero, the error
// contains the stderr output.
func (c *RealCommander) Run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, c.binPath, args...)

	c.logger.Debug("executing tmux command", "args", args)

	out, err := cmd.Output()
	if err != nil {
		var stderr string
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(exitErr.Stderr))
		}
		return "", fmt.Errorf("tmux %s: %s: %w", strings.Join(args, " "), stderr, err)
	}

	return strings.TrimRight(string(out), " \t\n\r"), nil
}
