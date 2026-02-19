package runner

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Event represents a structured event emitted during non-interactive execution.
type Event struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Raw       string    `json:"raw,omitempty"`
}

// Request describes a deterministic task execution request.
type Request struct {
	RunID        string `json:"run_id"`
	AgentID      string `json:"agent_id"`
	Prompt       string `json:"prompt"`
	WorkDir      string `json:"work_dir"`
	EnvelopePath string `json:"envelope_path,omitempty"`
	Timeout      time.Duration
	IdleTimeout  time.Duration
	RunnerMode   string `json:"runner_mode,omitempty"`
}

// Result captures the outcome of a deterministic task execution.
type Result struct {
	Output            string    `json:"output"`
	RawOutput         string    `json:"raw_output,omitempty"`
	Events            []Event   `json:"events,omitempty"`
	StartedAt         time.Time `json:"started_at"`
	FinishedAt        time.Time `json:"finished_at"`
	LastEventAt       time.Time `json:"last_event_at,omitempty"`
	TerminationReason string    `json:"termination_reason,omitempty"`
	ExitCode          int       `json:"exit_code"`
	Duration          time.Duration
}

// TaskRunner executes a prompt deterministically outside of interactive panes.
type TaskRunner interface {
	Name() string
	Run(ctx context.Context, req Request) (Result, error)
}

// Registry maps plugin names to deterministic task runners.
type Registry struct {
	mu      sync.RWMutex
	runners map[string]TaskRunner
	logger  *slog.Logger
}

// NewRegistry creates an empty deterministic runner registry.
func NewRegistry(logger *slog.Logger) *Registry {
	if logger == nil {
		logger = slog.Default()
	}
	return &Registry{
		runners: make(map[string]TaskRunner),
		logger:  logger,
	}
}

// Register associates a plugin name with a deterministic runner.
func (r *Registry) Register(plugin string, taskRunner TaskRunner) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runners[plugin] = taskRunner
}

// Get returns the runner for a plugin, if registered.
func (r *Registry) Get(plugin string) (TaskRunner, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	taskRunner, ok := r.runners[plugin]
	return taskRunner, ok
}
