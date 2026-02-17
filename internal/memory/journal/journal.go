// Package journal provides a high-level decision recording and retrieval layer
// over the SQLite operational store. It handles validation, ID generation,
// timestamp defaults, and export.
package journal

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/roygabriel/crux/internal/memory/store"
	"github.com/roygabriel/crux/pkg/types"
)

// Validation sentinel errors.
var (
	// ErrMissingContext indicates the decision has no context.
	ErrMissingContext = errors.New("decision missing context")
	// ErrMissingRationale indicates the decision has no rationale.
	ErrMissingRationale = errors.New("decision missing rationale")
	// ErrMissingAction indicates the decision has no action.
	ErrMissingAction = errors.New("decision missing action")
)

// Journal provides validated decision recording and retrieval.
type Journal struct {
	store *store.Store
}

// NewJournal creates a Journal backed by the given store.
func NewJournal(s *store.Store) *Journal {
	return &Journal{store: s}
}

// Record validates a decision, generates an ID if empty, sets the timestamp
// if zero, and persists it to the store.
func (j *Journal) Record(ctx context.Context, d types.Decision) error {
	if d.Context == "" {
		return ErrMissingContext
	}
	if d.Rationale == "" {
		return ErrMissingRationale
	}
	if d.Action == "" {
		return ErrMissingAction
	}

	if d.ID == "" {
		id, err := generateUUID()
		if err != nil {
			return fmt.Errorf("generating decision ID: %w", err)
		}
		d.ID = id
	}
	if d.Timestamp.IsZero() {
		d.Timestamp = time.Now().UTC()
	}

	return j.store.RecordDecision(ctx, d)
}

// Search performs a full-text search over decisions.
func (j *Journal) Search(ctx context.Context, query string, n int) ([]types.Decision, error) {
	return j.store.SearchDecisions(ctx, query, n)
}

// ByPhase returns all decisions for the given phase.
func (j *Journal) ByPhase(ctx context.Context, phaseID types.PhaseID) ([]types.Decision, error) {
	return j.store.QueryDecisions(ctx, store.DecisionFilter{PhaseID: phaseID})
}

// ByAgent returns all decisions for the given agent.
func (j *Journal) ByAgent(ctx context.Context, agentID types.AgentID) ([]types.Decision, error) {
	return j.store.QueryDecisions(ctx, store.DecisionFilter{AgentID: agentID})
}

// Recent returns the n most recent decisions in reverse chronological order.
func (j *Journal) Recent(ctx context.Context, n int) ([]types.Decision, error) {
	return j.store.QueryDecisions(ctx, store.DecisionFilter{Limit: n})
}

// Export writes all decisions as newline-delimited JSON (JSONL) to the writer.
func (j *Journal) Export(ctx context.Context, w io.Writer) error {
	decisions, err := j.store.QueryDecisions(ctx, store.DecisionFilter{})
	if err != nil {
		return fmt.Errorf("exporting decisions: %w", err)
	}

	enc := json.NewEncoder(w)
	for _, d := range decisions {
		if err := enc.Encode(d); err != nil {
			return fmt.Errorf("encoding decision %s: %w", d.ID, err)
		}
	}
	return nil
}

// generateUUID produces a UUID v4 string using crypto/rand.
func generateUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	// Set version 4 and variant bits per RFC 4122.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
