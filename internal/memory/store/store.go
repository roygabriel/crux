// Package store provides a SQLite-backed operational store for decisions and events.
// It uses ncruces/go-sqlite3 (WASM-based, no CGO) with WAL mode and FTS5 full-text search.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/roygabriel/crux/pkg/types"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

// Event represents an operational event recorded in the store.
type Event struct {
	// ID uniquely identifies this event.
	ID string `json:"id"`
	// Type categorizes the event (e.g. "agent_started", "phase_completed").
	Type string `json:"type"`
	// AgentID is the agent associated with this event.
	AgentID string `json:"agent_id"`
	// PhaseID is the phase associated with this event.
	PhaseID string `json:"phase_id"`
	// Data is arbitrary JSON payload for the event.
	Data string `json:"data"`
	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`
}

// DecisionFilter specifies criteria for querying decisions.
type DecisionFilter struct {
	// PhaseID filters by phase, if non-empty.
	PhaseID types.PhaseID
	// AgentID filters by agent, if non-empty.
	AgentID types.AgentID
	// Since filters to decisions after this time, if non-zero.
	Since time.Time
	// Limit caps the number of results, if positive.
	Limit int
}

// EventFilter specifies criteria for querying events.
type EventFilter struct {
	// Type filters by event type, if non-empty.
	Type string
	// AgentID filters by agent, if non-empty.
	AgentID string
	// PhaseID filters by phase, if non-empty.
	PhaseID string
	// Since filters to events after this time, if non-zero.
	Since time.Time
	// Limit caps the number of results, if positive.
	Limit int
}

// Store provides SQLite-backed persistence for decisions and events.
type Store struct {
	db *sql.DB
}

// NewStore opens a SQLite database at the given path and returns a Store.
func NewStore(dbPath string) (*Store, error) {
	dsn := "file:" + dbPath + "?_txlock=immediate&_pragma=journal_mode(wal)&_pragma=foreign_keys(on)"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Verify the connection works.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// Migrate applies the schema DDL to the database. It is idempotent.
func (s *Store) Migrate() error {
	if _, err := s.db.Exec(schemaDDL); err != nil {
		return fmt.Errorf("applying migrations: %w", err)
	}
	return nil
}

// RecordDecision inserts a decision record into the store.
func (s *Store) RecordDecision(ctx context.Context, d types.Decision) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO decisions (id, timestamp, phase_id, prompt_num, agent_id, context, rationale, action, outcome)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID,
		d.Timestamp.Format(time.RFC3339),
		string(d.PhaseID),
		d.PromptNum,
		string(d.AgentID),
		d.Context,
		d.Rationale,
		d.Action,
		d.Outcome,
	)
	if err != nil {
		return fmt.Errorf("recording decision: %w", err)
	}
	return nil
}

// QueryDecisions returns decisions matching the given filter, ordered by timestamp descending.
func (s *Store) QueryDecisions(ctx context.Context, f DecisionFilter) ([]types.Decision, error) {
	var clauses []string
	var args []any

	if f.PhaseID != "" {
		clauses = append(clauses, "phase_id = ?")
		args = append(args, string(f.PhaseID))
	}
	if f.AgentID != "" {
		clauses = append(clauses, "agent_id = ?")
		args = append(args, string(f.AgentID))
	}
	if !f.Since.IsZero() {
		clauses = append(clauses, "timestamp > ?")
		args = append(args, f.Since.Format(time.RFC3339))
	}

	query := "SELECT id, timestamp, phase_id, prompt_num, agent_id, context, rationale, action, outcome FROM decisions"
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY timestamp DESC"
	if f.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", f.Limit)
	}

	return s.scanDecisions(ctx, query, args...)
}

// GetDecision retrieves a single decision by its ID. Returns nil if not found.
func (s *Store) GetDecision(ctx context.Context, id string) (*types.Decision, error) {
	decisions, err := s.scanDecisions(ctx,
		"SELECT id, timestamp, phase_id, prompt_num, agent_id, context, rationale, action, outcome FROM decisions WHERE id = ?",
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("getting decision %s: %w", id, err)
	}
	if len(decisions) == 0 {
		return nil, nil
	}
	return &decisions[0], nil
}

// SearchDecisions performs a full-text search over decisions using FTS5.
func (s *Store) SearchDecisions(ctx context.Context, queryText string, limit int) ([]types.Decision, error) {
	query := `SELECT d.id, d.timestamp, d.phase_id, d.prompt_num, d.agent_id, d.context, d.rationale, d.action, d.outcome
		FROM decisions_fts fts
		JOIN decisions d ON d.rowid = fts.rowid
		WHERE decisions_fts MATCH ?
		ORDER BY rank
		LIMIT ?`

	return s.scanDecisions(ctx, query, queryText, limit)
}

// RecordEvent inserts an event record into the store.
func (s *Store) RecordEvent(ctx context.Context, e Event) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO events (id, type, agent_id, phase_id, data, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		e.ID,
		e.Type,
		e.AgentID,
		e.PhaseID,
		e.Data,
		e.Timestamp.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("recording event: %w", err)
	}
	return nil
}

// QueryEvents returns events matching the given filter, ordered by timestamp descending.
func (s *Store) QueryEvents(ctx context.Context, f EventFilter) ([]Event, error) {
	var clauses []string
	var args []any

	if f.Type != "" {
		clauses = append(clauses, "type = ?")
		args = append(args, f.Type)
	}
	if f.AgentID != "" {
		clauses = append(clauses, "agent_id = ?")
		args = append(args, f.AgentID)
	}
	if f.PhaseID != "" {
		clauses = append(clauses, "phase_id = ?")
		args = append(args, f.PhaseID)
	}
	if !f.Since.IsZero() {
		clauses = append(clauses, "timestamp > ?")
		args = append(args, f.Since.Format(time.RFC3339))
	}

	query := "SELECT id, type, agent_id, phase_id, data, timestamp FROM events"
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY timestamp DESC"
	if f.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", f.Limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var ts string
		if err := rows.Scan(&e.ID, &e.Type, &e.AgentID, &e.PhaseID, &e.Data, &ts); err != nil {
			return nil, fmt.Errorf("scanning event: %w", err)
		}
		e.Timestamp, err = time.Parse(time.RFC3339, ts)
		if err != nil {
			return nil, fmt.Errorf("parsing event timestamp: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// Prune deletes decisions and events older than the given cutoff time.
// FTS triggers automatically clean up the search index. Returns the total
// number of rows deleted.
func (s *Store) Prune(ctx context.Context, olderThan time.Time) (int64, error) {
	cutoff := olderThan.Format(time.RFC3339)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("beginning prune transaction: %w", err)
	}
	defer tx.Rollback()

	res1, err := tx.ExecContext(ctx, "DELETE FROM decisions WHERE timestamp < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("pruning decisions: %w", err)
	}
	n1, _ := res1.RowsAffected()

	res2, err := tx.ExecContext(ctx, "DELETE FROM events WHERE timestamp < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("pruning events: %w", err)
	}
	n2, _ := res2.RowsAffected()

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("committing prune: %w", err)
	}

	return n1 + n2, nil
}

// scanDecisions is a helper that executes a query and scans rows into Decision slices.
func (s *Store) scanDecisions(ctx context.Context, query string, args ...any) ([]types.Decision, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying decisions: %w", err)
	}
	defer rows.Close()

	var decisions []types.Decision
	for rows.Next() {
		var d types.Decision
		var ts, phaseID, agentID string
		if err := rows.Scan(&d.ID, &ts, &phaseID, &d.PromptNum, &agentID, &d.Context, &d.Rationale, &d.Action, &d.Outcome); err != nil {
			return nil, fmt.Errorf("scanning decision: %w", err)
		}
		d.PhaseID = types.PhaseID(phaseID)
		d.AgentID = types.AgentID(agentID)
		d.Timestamp, err = time.Parse(time.RFC3339, ts)
		if err != nil {
			return nil, fmt.Errorf("parsing decision timestamp: %w", err)
		}
		decisions = append(decisions, d)
	}
	return decisions, rows.Err()
}
