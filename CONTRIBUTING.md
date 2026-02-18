# Contributing

## Prerequisites

- **Go 1.25+** — [install](https://go.dev/dl/)
- **golangci-lint** — `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`
- **make**
- **tmux** (optional) — required only for manual end-to-end testing with real agents

## Quick Start

```bash
git clone https://github.com/roygabriel/crux.git
cd crux
make build
make test
```

The binary is written to `bin/crux`. Run `./bin/crux --help` to see available commands.

## Running Tests

### Unit Tests

```bash
make test
```

Runs all unit tests with the race detector and generates `coverage.out`.

### Integration Tests

```bash
make integration
```

Runs tests tagged `//go:build integration`. These exercise the full orchestrator loop with mock agents and take ~60s.

### Coverage

```bash
make coverage          # print per-function coverage
make coverage-check    # fail if total coverage < 70%
```

### Lint

```bash
make lint   # golangci-lint
make vet    # go vet
```

## Project Structure

See `docs/OVERVIEW.md` for the full architecture reference. Key packages:

| Package | Purpose |
|---------|---------|
| `cmd/crux` | CLI entry point (cobra) |
| `internal/orchestrator` | Main control loop, world state, assignment |
| `internal/phase` | Phase specs, prompt contracts, gate runner |
| `internal/agent` | Agent registry, lifecycle, messenger |
| `internal/tmux` | Session/pane management, watcher |
| `internal/plugin` | Agent plugin interface and registry |
| `internal/memory` | Session, journal, work notes, vector store |
| `internal/testutil` | Shared test doubles and integration harness |
| `pkg/types` | Shared domain types |

## Adding a Plugin

1. Create `internal/plugin/<name>/plugin.go` implementing `plugin.AgentPlugin`.
2. Register the factory in `plugin.Registry` (see existing plugins for patterns).
3. Add detection regex patterns for ready, busy, error, and rate-limit states.
4. Write tests covering all 9 interface methods.

## Adding a Phase

1. Run `crux phase create` to scaffold the spec and prompt files.
2. Edit the generated `PHASE*.md` spec: set dependencies, exit criteria, file lists.
3. Edit the generated `PHASE*-PROMPT.md`: define prompts with task descriptions and verification gates.
4. Reference `templates/phase-spec.md` for the full spec format.

## Commit Format

Use conventional commit prefixes scoped to the package:

```
feat(orchestrator): add conflict detection
fix(phase): handle empty exit criteria
test(testutil): add harness builder tests
refactor(agent): extract spawn logic
```

- Imperative mood, lowercase first word
- First line under 72 characters
- Body paragraph for non-trivial changes

## PR Checklist

- [ ] `make test` passes
- [ ] `make lint` passes (no new warnings)
- [ ] `make vet` passes
- [ ] Integration tests pass: `make integration`
- [ ] Coverage meets 70% minimum: `make coverage-check`
- [ ] Work notes updated in `docs/notes/` (if applicable)
- [ ] New exported types have Go doc comments
- [ ] No `CGO` dependencies introduced
