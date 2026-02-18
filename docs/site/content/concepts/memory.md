---
title: "Memory System"
weight: 3
---

# Memory System

Crux uses a three-layer memory architecture to solve the orchestrator context loss problem. Every decision, event, and piece of context is persisted and searchable -- even across sessions.

## Layer 1: File-Based Memory Bank

Six markdown files in `.crux/memory-bank/` provide human-readable project context:

| File | Purpose |
|------|---------|
| `projectbrief.md` | What we're building |
| `productContext.md` | Why -- user problems, UX goals |
| `systemPatterns.md` | Architecture decisions, patterns used |
| `techContext.md` | Stack, dependencies, constraints |
| `activeContext.md` | Current focus, recent changes, next steps |
| `progress.md` | What works, what doesn't, blockers |

These files are read on agent launch and injected into agent prompts. They are updated after significant actions.

## Layer 2: SQLite Operational Store

Structured storage at `.crux/memory.db` for machine-searchable data:

- **decisions** -- every decision with context, rationale, and outcome
- **sessions** -- session logs with agent, phase, prompt, and timestamps
- **events** -- append-only event log (event sourcing for replay)
- **FTS5** full-text search across all tables
- Auto-prune entries older than the retention window (default 90 days)

### Decision Records

Every coordination decision the orchestrator makes is recorded:

```json
{
  "id": "dec-uuid",
  "timestamp": "2026-02-17T12:00:00Z",
  "phase": "2A",
  "prompt": 2,
  "agent": "engineer-1",
  "context": "needed to choose between chi and gorilla for routing",
  "rationale": "chi has better middleware composition",
  "action": "selected chi router",
  "outcome": "implemented successfully"
}
```

## Layer 3: Vector Index

An embedded vector database (chromem-go) at `.crux/vectors/` enables semantic search:

- Every decision record is embedded for semantic similarity search
- Sub-millisecond retrieval for <50K records
- Persisted to disk via gob serialization
- Used by the orchestrator's Decision RAG before coordination decisions

### Decision RAG

Before the orchestrator makes a coordination decision, it:

1. Queries the vector index with the current context
2. Retrieves semantically similar past decisions
3. Injects them into the orchestrator prompt
4. Makes the decision with full historical context

This prevents repeated mistakes and contradictory instructions across sessions.

## Work Notes

Per-phase state tracking files (`work-notes/PHASE<ID>.md`) are auto-generated and updated:

- **Status**: not started / in progress / blocked / complete
- **Decisions**: with rationale
- **Assumptions**: stated explicitly
- **Open questions**: tracked for resolution
- **Session log**: what changed, why, blockers, next steps
- **Commit references**: linked to specific prompts
- **Prompt progress**: checklist (`[x] Prompt 1`, `[ ] Prompt 2`)

Work notes bridge sessions -- when a session resumes, the orchestrator reads the work notes to understand where it left off.

## Embedding Providers

Crux supports two embedding providers for vector search:

| Provider | Configuration | Notes |
|----------|--------------|-------|
| `chromem-default` | No setup required | Built-in, pure Go |
| `ollama` | Requires Ollama running locally | Uses `nomic-embed-text` by default |

Configure in `.crux/config.yaml`:

```yaml
memory:
  embedding_provider: ollama
  embedding_model: nomic-embed-text
```
