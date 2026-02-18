---
sidebar_position: 2
title: Phase System
description: Two-document phase system in Crux. Phase specs define scope, dependencies, and exit criteria. Prompt docs define the exact execution contract for each agent task.
---

# The Two-Document Phase System

Every unit of work in Crux has two files: a **phase spec** and a **prompt doc**. Together they define both what needs to be done and exactly how agents should execute it.

## Phase Spec (`PHASE{ID}.md`)

The spec defines **what**: scope, dependencies, files, and exit criteria:

```markdown
# Phase 2A: Core Types

## Status
Planned

## Depends On
- Phase 1A

## Design Rationale
Why this phase exists, what it isolates, why now.

## Tasks
### Prompt 1
- Define core type structs
- Add JSON tags

### Prompt 2
- Write table-driven tests

## Files
### New
- pkg/types/agent.go
- pkg/types/agent_test.go

### Modified
- go.mod

### Referenced (read-only)
- docs/OVERVIEW.md

## Exit Criteria
- [ ] `go build ./...` exits 0
- [ ] `go vet ./...` exits 0
- [ ] `go test -race ./...` exits 0
- [ ] Coverage >= 80%

## Progress Notes
```

### Key Sections

| Section | Purpose |
|---------|---------|
| **Status** | `Planned`, `Active`, `Complete` |
| **Depends On** | Which phases must complete first |
| **Tasks** | Work items grouped by prompt number |
| **Files** | New, Modified, and Referenced file paths |
| **Exit Criteria** | Executable gates that must pass |

## Prompt Doc (`PHASE{ID}-PROMPT.md`)

The prompt doc defines **how**: the exact execution contract for each prompt:

```markdown
# Phase 2A Implementation Prompts

## Prompt 1 of 2: Core Type Definitions

### Required Reading
- docs/OVERVIEW.md
- pkg/types/phase.go

### Task
1. Create agent.go with AgentState struct
2. Add JSON tags on all fields
3. Add Go doc comments

### Verification
```bash
go build ./...
go vet ./...
```

### Acceptance Criteria
- AgentState struct exported with 5 fields
- All fields have json tags
```

### Execution Rules

1. **Sequential execution**: Prompt N+1 does not start until Prompt N's verification gates pass.
2. **Required reading**: Agents read listed files before writing code.
3. **Verification is mandatory**: If `go build` or `go test` fails, the agent is halted.
4. **Stop rule**: An agent must not proceed past a failed gate.

## Phase Lifecycle

```
Planned → Active → Complete
            │
            ├── Prompt 1 → Gate ✓
            ├── Prompt 2 → Gate ✓
            ├── Prompt 3 → Gate ✓
            └── Exit Criteria → Phase Complete
```

1. A phase moves to `Active` when all dependencies are `Complete`.
2. The engine assigns prompts sequentially.
3. After each prompt, verification gates run.
4. When all prompts complete and exit criteria pass, the phase is `Complete`.

## Creating Phases

Use the CLI to generate scaffolding:

```bash
crux phase create --id 3A --name "Memory System" --depends-on 2A --prompts 4
```

This creates both `PHASE3A.md` and `PHASE3A-PROMPT.md` with the standard template.

## Validating Phases

```bash
crux phase validate
```

Checks that every spec has exit criteria, every prompt has verification commands, and parallel phases have no file conflicts.
