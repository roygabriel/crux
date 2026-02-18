package agent

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/roygabriel/crux/internal/plugin"
	"github.com/roygabriel/crux/internal/tmux"
	"github.com/roygabriel/crux/pkg/types"
)

// Sentinel errors for agent registry operations.
var (
	// ErrAgentNotFound is returned when a requested agent is not registered.
	ErrAgentNotFound = errors.New("agent not found")
	// ErrAgentAlreadyExists is returned when spawning an agent with a
	// duplicate ID.
	ErrAgentAlreadyExists = errors.New("agent already exists")
)

// AgentInstance holds a running agent's state, its plugin adapter, and
// the tmux pane where it operates.
type AgentInstance struct {
	// Agent is the agent metadata.
	Agent types.Agent `json:"agent"`
	// Plugin is the adapter used to interact with this agent.
	Plugin plugin.AgentPlugin `json:"-"`
	// LaunchedAt is when the agent was spawned.
	LaunchedAt time.Time `json:"launched_at"`
}

// Registry manages running agent instances with thread-safe CRUD
// operations and coordinates with tmux for pane lifecycle.
type Registry struct {
	mu        sync.RWMutex
	instances map[types.AgentID]*AgentInstance
	sm        *tmux.SessionManager
	pm        *tmux.PaneManager
	plugins   *plugin.Registry
	logger    *slog.Logger
}

// NewRegistry creates an agent registry backed by the given tmux
// managers and plugin registry.
func NewRegistry(sm *tmux.SessionManager, pm *tmux.PaneManager, plugins *plugin.Registry, logger *slog.Logger) *Registry {
	return &Registry{
		instances: make(map[types.AgentID]*AgentInstance),
		sm:        sm,
		pm:        pm,
		plugins:   plugins,
		logger:    logger,
	}
}

// Get returns the agent instance for the given ID. It returns
// ErrAgentNotFound if no agent with that ID exists.
func (r *Registry) Get(id types.AgentID) (*AgentInstance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	inst, exists := r.instances[id]
	if !exists {
		return nil, fmt.Errorf("get agent %q: %w", id, ErrAgentNotFound)
	}
	return inst, nil
}

// List returns all registered agent instances sorted by agent ID. The
// returned slice is always non-nil.
func (r *Registry) List() []*AgentInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*AgentInstance, 0, len(r.instances))
	for _, inst := range r.instances {
		list = append(list, inst)
	}
	slices.SortFunc(list, func(a, b *AgentInstance) int {
		return strings.Compare(string(a.Agent.ID), string(b.Agent.ID))
	})
	return list
}

// UpdateStatus changes the operational status of the given agent. It
// returns ErrAgentNotFound if the agent does not exist.
func (r *Registry) UpdateStatus(id types.AgentID, status types.AgentStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	inst, exists := r.instances[id]
	if !exists {
		return fmt.Errorf("update status for agent %q: %w", id, ErrAgentNotFound)
	}
	inst.Agent.Status = status
	return nil
}
