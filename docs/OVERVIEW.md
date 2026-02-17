# Crux — Architecture Reference

## 1. Introduction

Crux is a single-binary Go CLI that orchestrates multiple AI coding agents (Claude Code, Codex CLI, Gemini CLI) running in tmux sessions. It enforces a phase-based execution workflow with persistent memory, verification gates, and vector-searchable decision tracking.

The tool automates the workflow described in the LLM Development Guide: Plan → Prompt docs → Work notes → Execute → Verify → Commit. Instead of a human manually pasting work notes and running verification commands, Crux does this for every agent, every prompt, every session.

## 2. Core Architecture

```
┌──────────────────────────────────────────────────────────┐
│                    Orchestrator Loop                      │
│  World State │ Agent Assignment │ Decision RAG │ Gates    │
├──────────┬───────────┬───────────┬───────────────────────┤
│  Phase   │  Memory   │  Agent    │  Security             │
│  Engine  │  System   │  Manager  │  Layer                │
├──────────┼───────────┼───────────┼───────────────────────┤
│  Spec    │  Bank     │  Registry │  Sandbox              │
│  Parser  │  (md)     │  Lifecycle│  Permissions           │
│  Prompt  │  SQLite   │  Messenger│  Audit                │
│  Contract│  Vector   │  Health   │  Rate Limit           │
│  Gate    │  Journal  │  Scheduler│  Git Safety           │
│  Runner  │  WorkNotes│           │                       │
├──────────┴───────────┴───────────┴───────────────────────┤
│                    Plugin Layer                           │
│  Claude Code │ Codex CLI │ Gemini CLI │ Generic           │
├──────────────┴───────────┴────────────┴──────────────────┤
│                    tmux Layer                             │
│  Sessions │ Panes │ Windows │ capture-pane │ send-keys    │
└──────────────────────────────────────────────────────────┘
```

## 3. The Two-Document Phase System

Every unit of work has two files:

**Phase Spec** (`docs/phases/PHASE<ID>.md`) defines WHAT:
- Status, dependencies, design rationale
- Tasks grouped by prompt number
- Files: new, modified, referenced-only
- Exit criteria with executable commands

**Prompt Doc** (`docs/phases/PHASE<ID>-PROMPT.md`) defines HOW:
- One section per prompt, executed sequentially
- Required reading (exact file paths)
- Interface contracts (Go signatures)
- Task steps (numbered)
- Verification commands
- Stop rule: do not proceed until gates pass

The phase engine parses both files and enforces the execution contract programmatically.

## 4. Memory System

Three-layer architecture solving the orchestrator context loss problem:

### Layer 1: File-Based Memory Bank (`.crux/memory-bank/`)
Six markdown files (git-tracked) providing human-readable project context:
- `projectbrief.md` — what we're building
- `productContext.md` — why, user problems, UX goals
- `systemPatterns.md` — architecture decisions, patterns used
- `techContext.md` — stack, dependencies, constraints
- `activeContext.md` — current focus, recent changes, next steps
- `progress.md` — what works, what doesn't, blockers

Read on agent launch. Injected into agent prompts. Updated after significant actions.

### Layer 2: SQLite Operational Store (`.crux/memory.db`)
Structured storage for machine-searchable data:
- **decisions** — every decision with context, rationale, outcome
- **sessions** — session logs with agent, phase, prompt, timestamps
- **events** — append-only event log (event sourcing for replay)
- FTS5 full-text search across all tables
- Auto-prune entries older than retention window (default 90 days)

### Layer 3: Vector Index (`.crux/vectors/`)
chromem-go embedded vector database:
- Every decision record embedded for semantic search
- Sub-millisecond retrieval for <50K records
- Persisted to disk via gob serialization
- Used by the orchestrator for Decision RAG before coordination decisions

### Work Notes (`work-notes/PHASE<ID>.md`)
Per-phase state tracking (auto-generated and updated):
- Status (not started / in progress / blocked / complete)
- Decisions with rationale
- Assumptions
- Open questions
- Session log (what changed, why, blockers, next)
- Commit references
- Prompt progress (`[x] Prompt 1`, `[ ] Prompt 2`, etc.)

## 5. Agent Plugin System

Each CLI tool implements the `AgentPlugin` interface:

```go
type AgentPlugin interface {
    Name() string
    LaunchCmd(cfg AgentConfig) (bin string, args []string, err error)
    DetectReady(paneContent string) bool
    DetectBusy(paneContent string) bool
    DetectError(paneContent string) (string, bool)
    DetectRateLimit(paneContent string) (time.Duration, bool)
    FormatMessage(msg Message) string
    ParseOutput(paneContent string) (AgentOutput, error)
    Capabilities() []Capability
}
```

Plugins translate between the orchestrator's structured protocol and each CLI's native interface. The tmux layer provides the transport.

### Plugin Adapters

- **Claude Code**: Launches `claude` with optional `--dangerously-skip-permissions`, detects `>` prompt for ready, parses rate limit messages.
- **Codex CLI**: Launches `codex`, handles different prompt/output format, supports `--json` for structured output.
- **Gemini CLI**: Launches `gemini`, based on community fork patterns.
- **Generic**: Configurable via regex patterns in config for any CLI tool.

## 6. Orchestrator Control Loop

The orchestrator runs a continuous loop:

```
1. Poll all agent panes (capture-pane every 1-2s)
2. Update world state (agent status, current task, last output)
3. Check for completed prompts (parse output markers)
4. For completed prompts:
   a. Run verification gates
   b. Update work notes
   c. Record decisions in journal
   d. Advance to next prompt or next phase
5. Check for errors/rate limits/hangs
   a. Retry, wait, or reassign as appropriate
6. For coordination decisions:
   a. Query decision journal (vector search) for relevant context
   b. Inject into orchestrator prompt
   c. Make decision with full context
7. Sleep, repeat
```

### World State

A compact JSON summary always in the orchestrator's prompt (~200 tokens):

```json
{
  "phase": "2b",
  "agents": {
    "claude-1": { "status": "busy", "prompt": "2/4", "task": "core types" },
    "codex-1": { "status": "idle", "prompt": "3/4", "task": "API endpoint" }
  },
  "gates_passed": ["go build", "go vet"],
  "gates_pending": ["go test"],
  "decisions_today": 7,
  "open_questions": 2
}
```

## 7. Communication Protocol

JSON envelopes over tmux send-keys:

```json
{
  "id": "msg-uuid",
  "from": "orchestrator",
  "to": "engineer-1",
  "type": "task",
  "priority": "normal",
  "payload": {
    "phase": "2a",
    "prompt": 2,
    "context_files": ["internal/types/types.go"],
    "task": "Implement core orchestration types"
  },
  "timestamp": "2026-02-17T12:00:00Z"
}
```

Acknowledgment: orchestrator polls agent pane for response markers.

## 8. Security Model

### Permission Tiers
| Tier | File Write | Shell Commands | Network | Git Push |
|------|-----------|----------------|---------|----------|
| readonly | No | No | No | No |
| standard | Scoped paths | Allowlisted | No | No |
| elevated | Project root | Most commands | Localhost | Feature branches |
| autonomous | Project root | All non-destructive | Yes | Feature branches |

### Enforcement
- Filesystem sandbox: `filepath.Rel` checks against allowed paths
- Audit logging: every command, file change, message, gate result → `.crux/audit.log`
- Rate limiting: configurable per-agent caps on commands/min and files/session
- Git safety: all agent commits on feature branches (`crux/<agent-id>/<task>`), merge requires human

## 9. Parallel Execution

Parallel work requires non-overlapping files between phases. The engine validates:
- `FilesNew` and `FilesModified` sets must be disjoint across parallel phases
- If conflict detected, the later-assigned agent is halted
- Each parallel agent gets a separate git branch
- Periodic `git diff` monitoring for emergent conflicts

## 10. Configuration

YAML at `.crux/config.yaml`. All fields have defaults. Environment variable overrides with `CRUX_` prefix.

## 11. CLI Commands

```
crux init              Initialize project (creates .crux/, docs/phases/, work-notes/)
crux start             Start orchestration loop
crux status            Show world state (agents, phases, progress)
crux phase create      Generate phase spec + prompt doc from description
crux phase validate    Verify all specs have exit criteria, all prompts have verification
crux phase advance     Force-advance a phase (bypasses auto gates, still needs human approval)
crux decisions search  Semantic search across decision journal
crux notes show        Display work notes for a phase
crux replay            Replay a session transcript
crux audit             View audit log with filters
```
