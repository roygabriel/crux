package agent

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/roygabriel/crux/internal/plugin"
	"github.com/roygabriel/crux/pkg/types"
)

// Spawn creates a tmux pane for the agent, launches its CLI process,
// and registers the instance. The cfg.SessionID must name an existing
// tmux session. The cfg.Plugin must match a registered plugin name.
func (r *Registry) Spawn(ctx context.Context, cfg types.Agent) error {
	if cfg.ID == "" {
		return fmt.Errorf("spawn agent: ID must not be empty")
	}
	if cfg.SessionID == "" {
		return fmt.Errorf("spawn agent %q: session ID must not be empty", cfg.ID)
	}
	if cfg.Plugin == "" {
		return fmt.Errorf("spawn agent %q: plugin name must not be empty", cfg.ID)
	}

	// Fast-path duplicate check before doing I/O.
	r.mu.RLock()
	_, exists := r.instances[cfg.ID]
	r.mu.RUnlock()
	if exists {
		return fmt.Errorf("spawn agent %q: %w", cfg.ID, ErrAgentAlreadyExists)
	}

	// Resolve the plugin adapter.
	agentPlugin, err := r.plugins.Get(cfg.Plugin)
	if err != nil {
		return fmt.Errorf("spawn agent %q: %w", cfg.ID, err)
	}

	// Build the launch command.
	pluginCfg := plugin.AgentConfig{
		ID:         cfg.ID,
		WorkDir:    ".",
		Permission: cfg.Permission,
	}
	bin, args, err := agentPlugin.LaunchCmd(pluginCfg)
	if err != nil {
		return fmt.Errorf("spawn agent %q: launch command: %w", cfg.ID, err)
	}

	// Create a tmux pane in the agent's session.
	paneID, err := r.pm.Create(ctx, cfg.SessionID, "")
	if err != nil {
		return fmt.Errorf("spawn agent %q: create pane: %w", cfg.ID, err)
	}

	// Send the launch command to the pane.
	cmd := formatCmd(bin, args)
	if err := r.pm.SendKeys(ctx, paneID, cmd); err != nil {
		// Clean up the pane we just created.
		_ = r.pm.Kill(ctx, paneID)
		return fmt.Errorf("spawn agent %q: send launch command: %w", cfg.ID, err)
	}

	// Store the agent instance.
	cfg.Status = types.StatusIdle
	cfg.PaneID = paneID

	inst := &AgentInstance{
		Agent:      cfg,
		Plugin:     agentPlugin,
		LaunchedAt: time.Now(),
	}
	if strings.TrimSpace(r.outputLogDir) != "" {
		tee, teeErr := NewOutputTee(string(cfg.ID), r.outputLogDir, io.Discard)
		if teeErr != nil {
			r.logger.Warn("failed to create output tee",
				"agent_id", cfg.ID,
				"error", teeErr,
			)
		} else {
			inst.OutputTee = tee
		}
	}

	r.mu.Lock()
	// Double-check: another goroutine may have registered the same ID
	// between our fast-path check and now.
	if _, exists := r.instances[cfg.ID]; exists {
		r.mu.Unlock()
		_ = r.pm.Kill(ctx, paneID)
		return fmt.Errorf("spawn agent %q: %w", cfg.ID, ErrAgentAlreadyExists)
	}
	r.instances[cfg.ID] = inst
	r.mu.Unlock()

	r.logger.Info("spawned agent",
		"agent_id", cfg.ID,
		"plugin", cfg.Plugin,
		"pane_id", paneID,
	)
	return nil
}

// Kill gracefully stops the agent by sending "exit" to its pane, then
// force-kills the pane. The agent is removed from the registry
// regardless of whether the pane cleanup succeeds.
func (r *Registry) Kill(ctx context.Context, id types.AgentID) error {
	r.mu.Lock()
	inst, exists := r.instances[id]
	if !exists {
		r.mu.Unlock()
		return fmt.Errorf("kill agent %q: %w", id, ErrAgentNotFound)
	}
	delete(r.instances, id)
	r.mu.Unlock()

	paneID := inst.Agent.PaneID

	// Best-effort graceful shutdown.
	_ = r.pm.SendKeys(ctx, paneID, "exit")

	// Force-kill the tmux pane.
	if err := r.pm.Kill(ctx, paneID); err != nil {
		r.logger.Warn("failed to kill pane during agent kill",
			"agent_id", id,
			"pane_id", paneID,
			"error", err,
		)
	}
	if inst.OutputTee != nil {
		_ = inst.OutputTee.Close()
	}

	r.logger.Info("killed agent", "agent_id", id, "pane_id", paneID)
	return nil
}

// Restart relaunches an existing agent in its current tmux pane.
// The pane ID remains stable so watchers and TUI bindings do not break.
func (r *Registry) Restart(ctx context.Context, id types.AgentID) error {
	r.mu.Lock()
	inst, exists := r.instances[id]
	if !exists {
		r.mu.Unlock()
		return fmt.Errorf("restart agent %q: %w", id, ErrAgentNotFound)
	}

	pluginCfg := plugin.AgentConfig{
		ID:         inst.Agent.ID,
		WorkDir:    ".",
		Permission: inst.Agent.Permission,
	}
	bin, args, err := inst.Plugin.LaunchCmd(pluginCfg)
	if err != nil {
		r.mu.Unlock()
		return fmt.Errorf("restart agent %q: launch command: %w", id, err)
	}
	cmd := formatCmd(bin, args)
	paneID := inst.Agent.PaneID
	r.mu.Unlock()

	if err := r.pm.Respawn(ctx, paneID, cmd); err != nil {
		return fmt.Errorf("restart agent %q: %w", id, err)
	}

	r.mu.Lock()
	inst, exists = r.instances[id]
	if !exists {
		r.mu.Unlock()
		return fmt.Errorf("restart agent %q: %w", id, ErrAgentNotFound)
	}
	inst.Agent.Status = types.StatusIdle
	inst.LaunchedAt = time.Now()
	r.mu.Unlock()

	r.logger.Info("restarted agent", "agent_id", id, "pane_id", paneID)
	return nil
}

// formatCmd joins a binary name and its arguments into a single
// command string suitable for tmux send-keys.
func formatCmd(bin string, args []string) string {
	if len(args) == 0 {
		return bin
	}
	return bin + " " + strings.Join(args, " ")
}
