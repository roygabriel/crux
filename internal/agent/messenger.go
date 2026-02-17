package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/roygabriel/crux/internal/tmux"
	"github.com/roygabriel/crux/pkg/types"
)

const defaultPollInterval = time.Second

// MessageGate checks whether sending a message to an agent is permitted.
type MessageGate interface {
	GateMessage(agentID types.AgentID, perm types.Permission, action, target string) error
}

// Messenger sends messages to agents and waits for responses by
// coordinating between the plugin layer (formatting) and the tmux
// layer (transport).
type Messenger struct {
	pm           *tmux.PaneManager
	registry     *Registry
	logger       *slog.Logger
	pollInterval time.Duration
	gate         MessageGate
}

// NewMessenger creates a Messenger backed by the given PaneManager
// and agent Registry.
func NewMessenger(pm *tmux.PaneManager, registry *Registry, logger *slog.Logger) *Messenger {
	return &Messenger{
		pm:           pm,
		registry:     registry,
		logger:       logger,
		pollInterval: defaultPollInterval,
	}
}

// SetPollInterval configures the polling interval for WaitForResponse.
func (m *Messenger) SetPollInterval(d time.Duration) {
	m.pollInterval = d
}

// SetMessageGate configures an optional security gate for message sends.
func (m *Messenger) SetMessageGate(g MessageGate) {
	m.gate = g
}

// Send formats a message using the target agent's plugin and sends it
// to the agent's tmux pane. Large messages are split into chunks that
// stay under tmux send-keys limits. Each chunk is sent as a separate
// send-keys call.
func (m *Messenger) Send(ctx context.Context, agentID types.AgentID, msg types.Message) error {
	inst, err := m.registry.Get(agentID)
	if err != nil {
		return fmt.Errorf("send to agent %q: %w", agentID, err)
	}

	if m.gate != nil {
		if err := m.gate.GateMessage(agentID, inst.Agent.Permission, "message_send", string(msg.Type)); err != nil {
			return fmt.Errorf("send to agent %q: %w", agentID, err)
		}
	}

	text := inst.Plugin.FormatMessage(msg)
	if text == "" {
		return nil
	}

	chunks := chunkMessage(text, tmux.MaxInputLength)
	for _, chunk := range chunks {
		if err := m.pm.SendKeys(ctx, inst.Agent.PaneID, chunk); err != nil {
			return fmt.Errorf("send to agent %q: %w", agentID, err)
		}
	}

	m.logger.Info("sent message to agent",
		"agent_id", agentID,
		"message_id", msg.ID,
		"chunks", len(chunks),
	)
	return nil
}

// WaitForResponse polls the agent's tmux pane until the agent is no
// longer busy or the timeout expires. It returns the captured pane
// content when the agent finishes, or an error on timeout or context
// cancellation.
func (m *Messenger) WaitForResponse(ctx context.Context, agentID types.AgentID, timeout time.Duration) (string, error) {
	inst, err := m.registry.Get(agentID)
	if err != nil {
		return "", fmt.Errorf("wait for response from agent %q: %w", agentID, err)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	for {
		content, err := m.pm.Capture(ctx, inst.Agent.PaneID, 0)
		if err != nil {
			return "", fmt.Errorf("wait for response from agent %q: capture: %w", agentID, err)
		}

		if !inst.Plugin.DetectBusy(content) {
			return content, nil
		}

		select {
		case <-ctx.Done():
			return "", fmt.Errorf("wait for response from agent %q: %w", agentID, ctx.Err())
		case <-ticker.C:
		}
	}
}

// chunkMessage splits text into chunks suitable for tmux send-keys.
// It splits on newlines first, then breaks long lines at maxLen byte
// boundaries. Empty lines are skipped.
func chunkMessage(text string, maxLen int) []string {
	if text == "" {
		return nil
	}

	lines := strings.Split(text, "\n")
	var chunks []string

	for _, line := range lines {
		if line == "" {
			continue
		}

		for len(line) > maxLen {
			chunks = append(chunks, line[:maxLen])
			line = line[maxLen:]
		}
		if line != "" {
			chunks = append(chunks, line)
		}
	}

	return chunks
}
