---
sidebar_position: 1
title: Writing Phases
---

# Writing Phases

This guide covers how to author effective phase specs and prompt docs.

## Start with Scaffolding

```bash
crux phase create --id 3A --name "API Layer" --depends-on 2A --prompts 3
```

This generates two files in your `docs/phases/` directory:
- `PHASE3A.md` (the spec: what)
- `PHASE3A-PROMPT.md` (the prompt contract: how)

## Writing a Good Spec

### Status

Start with `Planned`. The engine moves it to `Active` when dependencies complete.

### Dependencies

List phases that must complete before this one can start. Use phase IDs:

```markdown
## Depends On
- Phase 1A
- Phase 2A
```

### Design Rationale

Explain why this phase exists and what it isolates. This helps the orchestrator (and humans) understand the purpose:

```markdown
## Design Rationale
The API layer depends on core types (2A) being stable. Isolating HTTP
handlers lets us test routing independently of business logic.
```

### Tasks

Group work items by prompt number. Each prompt should be a coherent unit:

```markdown
## Tasks

### Prompt 1
- Define route handlers
- Wire middleware stack

### Prompt 2
- Write handler tests
- Add integration test helpers
```

### Files

Be explicit about which files are created, modified, or referenced:

```markdown
## Files

### New
- internal/api/router.go
- internal/api/router_test.go

### Modified
- cmd/crux/root.go

### Referenced (read-only)
- docs/OVERVIEW.md
- internal/config/config.go
```

This enables the engine to detect file conflicts between parallel phases.

### Exit Criteria

Every spec must have executable exit criteria:

```markdown
## Exit Criteria
- [ ] `go build ./...` exits 0
- [ ] `go vet ./...` exits 0
- [ ] `go test -race ./...` exits 0
- [ ] Coverage >= 80% on internal/api
```

## Writing Prompt Docs

### Required Reading

List the exact files an agent must read before writing code:

```markdown
### Required Reading
- docs/OVERVIEW.md (Section 5: Agent Plugin System)
- internal/config/config.go
- pkg/types/agent.go
```

### Task Steps

Number the steps precisely. Each step should produce a testable artifact:

```markdown
### Task
1. Create `internal/api/router.go` with `NewRouter(deps...) *Router`
2. Add health check endpoint at `GET /health`
3. Wire chi middleware: RequestID, Logger, Recoverer
4. Register routes from agent config
```

### Verification

Every prompt must have verification commands:

````markdown
### Verification
```bash
go build ./...
go vet ./...
go test -race ./internal/api/...
```
````

### Acceptance Criteria

Concrete, checkable conditions:

```markdown
### Acceptance Criteria
- `NewRouter` accepts config and logger as dependencies
- Health endpoint returns 200 with `{"status": "ok"}`
- All middleware is applied in correct order
```

## Validation

Run validation to catch common issues:

```bash
crux phase validate
```

This checks:
- Every spec has exit criteria
- Every prompt has verification commands
- Parallel phases have no file conflicts
