// Package journal provides a high-level decision recording and retrieval layer
// over the SQLite operational store. It handles validation, ID generation,
// timestamp defaults, and export. When a vector index is provided, decisions
// are also indexed for semantic similarity search.
package journal

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/roygabriel/crux/internal/memory/store"
	"github.com/roygabriel/crux/internal/memory/vector"
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
	store       *store.Store
	vectorIndex *vector.VectorIndex
}

// NewJournal creates a Journal backed by the given store. The vector index is
// optional — pass nil to disable semantic search (FTS5 fallback is used).
func NewJournal(s *store.Store, vi *vector.VectorIndex) *Journal {
	return &Journal{store: s, vectorIndex: vi}
}

// Record validates a decision, generates an ID if empty, sets the timestamp
// if zero, and persists it to the store. If a vector index is configured,
// the decision is also indexed for semantic search.
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

	if err := j.store.RecordDecision(ctx, d); err != nil {
		return err
	}

	if j.vectorIndex != nil {
		embedText := fmt.Sprintf("Phase %s Prompt %d: %s — Decision: %s because %s",
			d.PhaseID, d.PromptNum, d.Context, d.Action, d.Rationale)
		metadata := map[string]string{
			"phase_id": string(d.PhaseID),
			"agent_id": string(d.AgentID),
			"id":       d.ID,
		}
		if err := j.vectorIndex.Add(ctx, d.ID, embedText, metadata); err != nil {
			slog.Warn("failed to index decision in vector store", "id", d.ID, "error", err)
		}
	}

	return nil
}

// Search performs a full-text search over decisions.
func (j *Journal) Search(ctx context.Context, query string, n int) ([]types.Decision, error) {
	return j.store.SearchDecisions(ctx, query, n)
}

// SemanticSearch performs a vector similarity search when a vector index is
// available, falling back to FTS5 keyword search otherwise.
func (j *Journal) SemanticSearch(ctx context.Context, query string, n int) ([]types.Decision, error) {
	if j.vectorIndex == nil {
		return j.store.SearchDecisions(ctx, query, n)
	}

	results, err := j.vectorIndex.Search(ctx, query, n, nil)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}

	decisions := make([]types.Decision, 0, len(results))
	for _, r := range results {
		d, err := j.store.GetDecision(ctx, r.ID)
		if err != nil {
			return nil, fmt.Errorf("fetching decision %s: %w", r.ID, err)
		}
		if d != nil {
			decisions = append(decisions, *d)
		}
	}
	return decisions, nil
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
