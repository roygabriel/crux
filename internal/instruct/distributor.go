package instruct

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// GenerationState tracks the result of the most recent instruction
// generation for a single agent.
type GenerationState struct {
	// AgentID is the agent this state belongs to.
	AgentID string `json:"agent_id"`
	// Role is the agent's functional role.
	Role RoleName `json:"role"`
	// CLI is the agent's CLI tool.
	CLI AgentCLI `json:"cli"`
	// Hash is the SHA-256 hex digest of the rendered content.
	Hash string `json:"hash"`
	// GeneratedAt is when the instructions were last rendered.
	GeneratedAt time.Time `json:"generated_at"`
	// TokenCount is the total token count of the rendered output.
	TokenCount int `json:"token_count"`
	// FilesWritten lists the absolute paths of files written.
	FilesWritten []string `json:"files_written"`
}

// TmuxKeySender sends literal key sequences to a tmux pane.
type TmuxKeySender interface {
	Run(ctx context.Context, args ...string) (string, error)
}

// AuditLogger records instruction generation events.
type AuditLogger interface {
	LogGeneration(agentID string, action string, files []string) error
}

// AgentConfigProvider provides the agent configurations needed for generation.
type AgentConfigProvider interface {
	// AgentIDs returns all configured agent IDs.
	AgentIDs() []string
	// AgentRole returns the role for the given agent ID.
	AgentRole(id string) RoleName
	// AgentCLI returns the CLI for the given agent ID.
	AgentCLI(id string) AgentCLI
	// AgentPaneID returns the tmux pane ID for the given agent.
	AgentPaneID(id string) string
}

// DistributorDeps groups the dependencies for NewDistributor.
type DistributorDeps struct {
	// Aggregator builds InstructionData for each agent.
	Aggregator *Aggregator
	// Renderer renders InstructionData with token budgets.
	Renderer *Renderer
	// TmuxCmd sends reload commands to agent panes.
	TmuxCmd TmuxKeySender
	// Audit logs generation events. May be nil.
	Audit AuditLogger
	// Agents provides agent configuration. Required.
	Agents AgentConfigProvider
	// ProjectRoot is the project root directory for file paths.
	ProjectRoot string
	// Logger is the structured logger. Falls back to slog.Default() if nil.
	Logger *slog.Logger
}

// Distributor coordinates instruction generation, file writing, and
// agent reload for all configured agents. It is thread-safe.
type Distributor struct {
	aggregator  *Aggregator
	renderer    *Renderer
	tmuxCmd     TmuxKeySender
	audit       AuditLogger
	agents      AgentConfigProvider
	projectRoot string
	state       map[string]*GenerationState
	mu          sync.RWMutex
	logger      *slog.Logger
}

// NewDistributor creates a Distributor from the provided dependencies.
func NewDistributor(deps DistributorDeps) *Distributor {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Distributor{
		aggregator:  deps.Aggregator,
		renderer:    deps.Renderer,
		tmuxCmd:     deps.TmuxCmd,
		audit:       deps.Audit,
		agents:      deps.Agents,
		projectRoot: deps.ProjectRoot,
		state:       make(map[string]*GenerationState),
		logger:      logger,
	}
}

// GenerateAll generates and writes instruction files for every configured
// agent. Errors from individual agents are logged but do not stop the loop.
func (d *Distributor) GenerateAll(ctx context.Context) error {
	ids := d.agents.AgentIDs()
	var firstErr error
	for _, id := range ids {
		if err := d.GenerateForAgent(ctx, id); err != nil {
			d.logger.Error("failed to generate instructions",
				"agent_id", id, "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// GenerateForAgent builds, renders, and writes instruction files for a
// single agent.
func (d *Distributor) GenerateForAgent(ctx context.Context, agentID string) error {
	role := d.agents.AgentRole(agentID)
	cli := d.agents.AgentCLI(agentID)

	adapter, err := AdapterForCLI(cli)
	if err != nil {
		return fmt.Errorf("get adapter for agent %s: %w", agentID, err)
	}

	// Build instruction data.
	data, err := d.aggregator.Build(ctx, agentID, role)
	if err != nil {
		return fmt.Errorf("aggregate context for agent %s: %w", agentID, err)
	}

	// Render with the adapter's token budget.
	result, err := d.renderer.Render(*data, adapter.TokenBudget())
	if err != nil {
		return fmt.Errorf("render instructions for agent %s: %w", agentID, err)
	}

	// Validate rendered output.
	if err := adapter.ValidateOutput(result.Content); err != nil {
		var warning *ValidationWarning
		if errors.As(err, &warning) {
			d.logger.Warn("instruction validation warning",
				"agent_id", agentID, "warning", warning.Message)
		} else {
			return fmt.Errorf("validate output for agent %s: %w", agentID, err)
		}
	}

	// Read existing files for marker preservation.
	existingFiles := d.readExistingFiles(adapter, result, d.projectRoot)

	// Prepare files via adapter.
	files, err := adapter.PrepareFiles(result, d.projectRoot, existingFiles)
	if err != nil {
		return fmt.Errorf("prepare files for agent %s: %w", agentID, err)
	}

	// Write files to disk.
	writtenPaths, err := d.writeFiles(files)
	if err != nil {
		return fmt.Errorf("write files for agent %s: %w", agentID, err)
	}

	// Compute content hash.
	hash := contentHash(result.Content)

	// Update generation state.
	d.mu.Lock()
	d.state[agentID] = &GenerationState{
		AgentID:      agentID,
		Role:         role,
		CLI:          cli,
		Hash:         hash,
		GeneratedAt:  time.Now().UTC(),
		TokenCount:   result.TotalTokens,
		FilesWritten: writtenPaths,
	}
	d.mu.Unlock()

	// Audit log.
	if d.audit != nil {
		if err := d.audit.LogGeneration(agentID, "generate", writtenPaths); err != nil {
			d.logger.Warn("audit log failed", "agent_id", agentID, "error", err)
		}
	}

	d.logger.Info("generated instructions",
		"agent_id", agentID,
		"cli", cli,
		"tokens", result.TotalTokens,
		"files", len(writtenPaths),
	)

	return nil
}

// RegenerateIfStale re-renders instructions for an agent and writes files
// only if the content hash has changed. Returns whether files were updated.
func (d *Distributor) RegenerateIfStale(ctx context.Context, agentID string) (bool, error) {
	role := d.agents.AgentRole(agentID)
	cli := d.agents.AgentCLI(agentID)

	adapter, err := AdapterForCLI(cli)
	if err != nil {
		return false, fmt.Errorf("get adapter for agent %s: %w", agentID, err)
	}

	data, err := d.aggregator.Build(ctx, agentID, role)
	if err != nil {
		return false, fmt.Errorf("aggregate context for agent %s: %w", agentID, err)
	}

	result, err := d.renderer.Render(*data, adapter.TokenBudget())
	if err != nil {
		return false, fmt.Errorf("render instructions for agent %s: %w", agentID, err)
	}

	newHash := contentHash(result.Content)

	// Compare with stored hash.
	d.mu.RLock()
	prev := d.state[agentID]
	d.mu.RUnlock()

	if prev != nil && prev.Hash == newHash {
		d.logger.Debug("instructions unchanged, skipping write",
			"agent_id", agentID)
		return false, nil
	}

	// Content changed — validate and write.
	if err := adapter.ValidateOutput(result.Content); err != nil {
		var warning *ValidationWarning
		if errors.As(err, &warning) {
			d.logger.Warn("instruction validation warning",
				"agent_id", agentID, "warning", warning.Message)
		} else {
			return false, fmt.Errorf("validate output for agent %s: %w", agentID, err)
		}
	}

	existingFiles := d.readExistingFiles(adapter, result, d.projectRoot)
	files, err := adapter.PrepareFiles(result, d.projectRoot, existingFiles)
	if err != nil {
		return false, fmt.Errorf("prepare files for agent %s: %w", agentID, err)
	}

	writtenPaths, err := d.writeFiles(files)
	if err != nil {
		return false, fmt.Errorf("write files for agent %s: %w", agentID, err)
	}

	d.mu.Lock()
	d.state[agentID] = &GenerationState{
		AgentID:      agentID,
		Role:         role,
		CLI:          cli,
		Hash:         newHash,
		GeneratedAt:  time.Now().UTC(),
		TokenCount:   result.TotalTokens,
		FilesWritten: writtenPaths,
	}
	d.mu.Unlock()

	if d.audit != nil {
		if err := d.audit.LogGeneration(agentID, "regenerate", writtenPaths); err != nil {
			d.logger.Warn("audit log failed", "agent_id", agentID, "error", err)
		}
	}

	d.logger.Info("regenerated instructions (content changed)",
		"agent_id", agentID,
		"cli", cli,
		"tokens", result.TotalTokens,
	)

	return true, nil
}

// ReloadAgent sends the adapter's reload command to the agent's tmux pane.
// Only agents with ReloadMemoryRefresh or ReloadNewSession are reloaded
// mid-session. For ReloadRestart agents, the orchestrator handles restart
// at session boundaries.
func (d *Distributor) ReloadAgent(ctx context.Context, agentID string) error {
	if d.tmuxCmd == nil {
		return fmt.Errorf("reload agent %s: no tmux commander configured", agentID)
	}

	cli := d.agents.AgentCLI(agentID)
	adapter, err := AdapterForCLI(cli)
	if err != nil {
		return fmt.Errorf("reload agent %s: %w", agentID, err)
	}

	method := adapter.ReloadMethod()
	if method == ReloadRestart || method == ReloadNone {
		d.logger.Debug("skipping mid-session reload",
			"agent_id", agentID, "method", method)
		return nil
	}

	paneID := d.agents.AgentPaneID(agentID)
	if paneID == "" {
		return fmt.Errorf("reload agent %s: no pane ID available", agentID)
	}

	reloadCmd := adapter.ReloadCommand()
	_, err = d.tmuxCmd.Run(ctx, "send-keys", "-t", paneID, reloadCmd, "Enter")
	if err != nil {
		return fmt.Errorf("reload agent %s: %w", agentID, err)
	}

	d.logger.Info("sent reload command to agent",
		"agent_id", agentID,
		"method", method,
		"pane_id", paneID,
	)

	return nil
}

// State returns a snapshot of the current generation state for all agents.
func (d *Distributor) State() map[string]*GenerationState {
	d.mu.RLock()
	defer d.mu.RUnlock()

	snapshot := make(map[string]*GenerationState, len(d.state))
	for k, v := range d.state {
		cp := *v
		snapshot[k] = &cp
	}
	return snapshot
}

// NeedsReload returns true if the agent's adapter supports mid-session reload.
func (d *Distributor) NeedsReload(agentID string) bool {
	cli := d.agents.AgentCLI(agentID)
	adapter, err := AdapterForCLI(cli)
	if err != nil {
		return false
	}
	method := adapter.ReloadMethod()
	return method == ReloadMemoryRefresh || method == ReloadNewSession
}

// readExistingFiles reads current on-disk content for files that the adapter
// might produce. This enables marker preservation in adapters.
func (d *Distributor) readExistingFiles(adapter AgentAdapter, _ *RenderResult, projectRoot string) map[string]string {
	existing := make(map[string]string)

	// Generate a dummy file list to discover paths the adapter will write.
	// We do a lightweight PrepareFiles with nil existingFiles to discover paths,
	// then read those paths from disk.
	dummyResult := &RenderResult{Content: "probe"}
	probe, err := adapter.PrepareFiles(dummyResult, projectRoot, nil)
	if err != nil {
		return existing
	}

	for _, f := range probe {
		data, err := os.ReadFile(f.Path)
		if err != nil {
			continue
		}
		existing[f.Path] = string(data)
	}

	return existing
}

// writeFiles writes instruction files to disk, creating directories as
// needed. Files are written with 0644 permissions.
func (d *Distributor) writeFiles(files []InstructionFile) ([]string, error) {
	paths := make([]string, 0, len(files))
	for _, f := range files {
		dir := filepath.Dir(f.Path)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return paths, fmt.Errorf("create directory %s: %w", dir, err)
		}
		if err := os.WriteFile(f.Path, []byte(f.Content), 0o644); err != nil {
			return paths, fmt.Errorf("write file %s: %w", f.Path, err)
		}
		paths = append(paths, f.Path)
	}
	return paths, nil
}

// contentHash returns the hex-encoded SHA-256 digest of content.
func contentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h)
}
