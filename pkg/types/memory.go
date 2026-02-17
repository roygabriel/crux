package types

import "time"

// MemoryScope determines the visibility and lifetime of a memory entry.
type MemoryScope string

const (
	// ScopeProject is visible across all sessions and phases.
	ScopeProject MemoryScope = "project"
	// ScopeSession is visible only within the current session.
	ScopeSession MemoryScope = "session"
	// ScopePhase is visible only within a specific phase.
	ScopePhase MemoryScope = "phase"
	// ScopeAgent is private to a single agent instance.
	ScopeAgent MemoryScope = "agent"
)

// String returns the string representation of a MemoryScope.
func (s MemoryScope) String() string { return string(s) }

// MemoryEntry is a key-value record in the memory subsystem.
type MemoryEntry struct {
	// ID uniquely identifies this memory entry.
	ID string `json:"id"`
	// Scope determines the entry's visibility and lifetime.
	Scope MemoryScope `json:"scope"`
	// Key is the lookup key for this entry.
	Key string `json:"key"`
	// Value is the stored content.
	Value string `json:"value"`
	// Tags are optional labels for categorization and search.
	Tags []string `json:"tags,omitempty"`
	// CreatedAt is when the entry was first written.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is when the entry was last modified.
	UpdatedAt time.Time `json:"updated_at"`
}
