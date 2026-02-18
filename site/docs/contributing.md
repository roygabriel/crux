---
sidebar_position: 101
title: Contributing
description: How to contribute to Crux. Development setup, building, testing, code quality checks, Go conventions, project structure, commit message format, and PR checklist.
---

# Contributing

Contributions to Crux are welcome. This guide covers the development workflow.

## Prerequisites

- **Go 1.25+**: [install](https://go.dev/dl/)
- **golangci-lint**: `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`
- **make**
- **tmux** (optional): required only for manual end-to-end testing with real agents

## Quick Start

```bash
git clone https://github.com/roygabriel/crux.git
cd crux
make build
make test
```

The binary is written to `bin/crux`. Run `./bin/crux --help` to see available commands.

## Building

```bash
make build
```

Produces `bin/crux`. The build uses `CGO_ENABLED=0` for a static binary.

## Testing

```bash
# Run all tests with race detector
make test

# Run specific package
go test -race ./internal/phase/...

# Check coverage
make coverage

# Enforce minimum coverage (70%)
make coverage-check

# Integration tests
make integration
```

## Code Quality

Before submitting changes:

```bash
# Lint
make lint

# Vet
make vet

# All three must pass
go build ./...
go vet ./...
go test -race ./...
```

## Go Conventions

- Follow the [Google Go Style Guide](https://google.github.io/styleguide/go/)
- Use `gofmt` formatting
- Return errors instead of panicking
- `context.Context` as first parameter for I/O functions
- Wrap errors with context: `fmt.Errorf("operation: %w", err)`
- JSON tags on all exported struct fields
- Interfaces in the consuming package, not the implementing package
- Tests live next to the code: `foo.go` → `foo_test.go`
- Table-driven tests with descriptive names

## Project Structure

```
cmd/crux/       CLI entry point (cobra commands)
internal/       Private packages
  config/       YAML config loading
  phase/        Phase engine
  memory/       Memory system (bank, store, vector, journal, worknotes)
  agent/        Agent lifecycle management
  orchestrator/ Main control loop
  security/     Sandbox, permissions, audit, rate limiting
  plugin/       Plugin interface
  pluginloader/ Plugin discovery and registration
  tmux/         Tmux session management
  tui/          Terminal UI
pkg/            Public packages
  types/        Shared types
  protocol/     JSON message envelope
plugins/        Agent CLI adapters
  claude/       Claude Code
  codex/        Codex CLI
  gemini/       Gemini CLI
  generic/      Configurable regex adapter
```

## Commit Messages

Use conventional commit format:

```
feat(package): description
fix(package): description
test(package): description
refactor(package): description
```

- Imperative mood, lowercase first word
- First line under 72 characters
- Body paragraph for non-trivial changes

## Adding a New Plugin

1. Create a package under `plugins/<name>/`
2. Implement the `AgentPlugin` interface
3. Register in `internal/pluginloader/loader.go`
4. Add to the valid plugins list in `internal/config/config.go`
5. Write table-driven tests for all detection methods

See the [Custom Plugins Guide](./guides/custom-plugins.md) for details.

## PR Checklist

- [ ] `make test` passes
- [ ] `make lint` passes (no new warnings)
- [ ] `make vet` passes
- [ ] Integration tests pass: `make integration`
- [ ] Coverage meets 70% minimum: `make coverage-check`
- [ ] Work notes updated in `docs/notes/` (if applicable)
- [ ] New exported types have Go doc comments
- [ ] No `CGO` dependencies introduced
