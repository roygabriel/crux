package plugin

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

// Sentinel errors for plugin registry operations.
var (
	// ErrPluginNotFound is returned when a requested plugin is not registered.
	ErrPluginNotFound = errors.New("plugin not found")
	// ErrPluginAlreadyRegistered is returned when registering a duplicate plugin name.
	ErrPluginAlreadyRegistered = errors.New("plugin already registered")
)

// PluginFactory is a constructor that returns a fresh AgentPlugin instance.
// Runtime configuration flows through AgentConfig at call sites, not at
// construction time.
type PluginFactory func() AgentPlugin

// Registry stores plugin factories keyed by name and provides thread-safe
// access for registration and retrieval.
type Registry struct {
	mu       sync.RWMutex
	factories map[string]PluginFactory
}

// NewRegistry creates an empty plugin registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]PluginFactory),
	}
}

// Register adds a plugin factory under the given name. It returns an error
// if name is empty, factory is nil, or a plugin with that name is already
// registered.
func (r *Registry) Register(name string, factory PluginFactory) error {
	if name == "" {
		return fmt.Errorf("register plugin: name must not be empty")
	}
	if factory == nil {
		return fmt.Errorf("register plugin: factory must not be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[name]; exists {
		return fmt.Errorf("register plugin %q: %w", name, ErrPluginAlreadyRegistered)
	}

	r.factories[name] = factory
	return nil
}

// Get retrieves a plugin by name, invoking its factory to return a fresh
// instance. It returns ErrPluginNotFound if no plugin is registered under
// that name.
func (r *Registry) Get(name string) (AgentPlugin, error) {
	r.mu.RLock()
	factory, exists := r.factories[name]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("get plugin %q: %w", name, ErrPluginNotFound)
	}

	return factory(), nil
}

// List returns the names of all registered plugins in sorted order.
// It always returns a non-nil slice.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
