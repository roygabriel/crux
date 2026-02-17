package store

// schemaDDL is the idempotent DDL for the operational store.
// It creates tables, FTS5 virtual tables, sync triggers, and indexes.
const schemaDDL = `
-- Decisions table: records orchestration decisions made during execution.
CREATE TABLE IF NOT EXISTS decisions (
    id         TEXT PRIMARY KEY,
    timestamp  TEXT NOT NULL,
    phase_id   TEXT NOT NULL DEFAULT '',
    prompt_num INTEGER NOT NULL DEFAULT 0,
    agent_id   TEXT NOT NULL DEFAULT '',
    context    TEXT NOT NULL DEFAULT '',
    rationale  TEXT NOT NULL DEFAULT '',
    action     TEXT NOT NULL DEFAULT '',
    outcome    TEXT NOT NULL DEFAULT ''
);

-- Events table: records operational events for auditing and replay.
CREATE TABLE IF NOT EXISTS events (
    id        TEXT PRIMARY KEY,
    type      TEXT NOT NULL DEFAULT '',
    agent_id  TEXT NOT NULL DEFAULT '',
    phase_id  TEXT NOT NULL DEFAULT '',
    data      TEXT NOT NULL DEFAULT '',
    timestamp TEXT NOT NULL
);

-- FTS5 virtual table for full-text search over decisions.
-- Uses external content mode referencing the decisions table.
CREATE VIRTUAL TABLE IF NOT EXISTS decisions_fts USING fts5(
    context,
    rationale,
    action,
    outcome,
    content=decisions,
    content_rowid=rowid
);

-- Triggers to keep the FTS index in sync with the decisions table.
CREATE TRIGGER IF NOT EXISTS decisions_ai AFTER INSERT ON decisions BEGIN
    INSERT INTO decisions_fts(rowid, context, rationale, action, outcome)
    VALUES (new.rowid, new.context, new.rationale, new.action, new.outcome);
END;

CREATE TRIGGER IF NOT EXISTS decisions_ad AFTER DELETE ON decisions BEGIN
    INSERT INTO decisions_fts(decisions_fts, rowid, context, rationale, action, outcome)
    VALUES ('delete', old.rowid, old.context, old.rationale, old.action, old.outcome);
END;

CREATE TRIGGER IF NOT EXISTS decisions_au AFTER UPDATE ON decisions BEGIN
    INSERT INTO decisions_fts(decisions_fts, rowid, context, rationale, action, outcome)
    VALUES ('delete', old.rowid, old.context, old.rationale, old.action, old.outcome);
    INSERT INTO decisions_fts(rowid, context, rationale, action, outcome)
    VALUES (new.rowid, new.context, new.rationale, new.action, new.outcome);
END;

-- Indexes for common query patterns.
CREATE INDEX IF NOT EXISTS idx_decisions_phase_id  ON decisions(phase_id);
CREATE INDEX IF NOT EXISTS idx_decisions_agent_id  ON decisions(agent_id);
CREATE INDEX IF NOT EXISTS idx_decisions_timestamp ON decisions(timestamp);

CREATE INDEX IF NOT EXISTS idx_events_type      ON events(type);
CREATE INDEX IF NOT EXISTS idx_events_agent_id  ON events(agent_id);
CREATE INDEX IF NOT EXISTS idx_events_phase_id  ON events(phase_id);
CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);
`
