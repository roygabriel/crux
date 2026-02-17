# Phase 1A: Project Skeleton, Types, Config, CLI Framework

## Status
Planned

## Depends On
None

## Design Rationale
Bootstrap everything needed for subsequent phases to compile. Establish the module, directory layout, shared types, configuration loading, and CLI framework. No business logic — just the skeleton that every other phase builds on.

## Tasks

### Prompt 1
- Go module, directory layout, Makefile, CI skeleton

### Prompt 2
- Core shared types (Agent, Message, Phase, Decision, MemoryEntry, Config structs)

### Prompt 3
- YAML config loading with environment variable overrides and validation

### Prompt 4
- CLI framework with cobra (torch start, status, init subcommands as stubs)

## Files

### New
- go.mod
- go.sum
- Makefile
- .gitignore
- .github/workflows/ci.yml
- cmd/torch/main.go
- cmd/torch/root.go
- cmd/torch/start.go
- cmd/torch/status.go
- cmd/torch/init.go
- internal/config/config.go
- internal/config/config_test.go
- internal/config/defaults.go
- pkg/types/agent.go
- pkg/types/message.go
- pkg/types/phase.go
- pkg/types/decision.go
- pkg/types/memory.go
- configs/default.yaml

### Modified
None

### Referenced (read-only)
- README.md
- LLM.md
- docs/OVERVIEW.md

## Exit Criteria
- [ ] `go build ./...` exits 0
- [ ] `go vet ./...` exits 0
- [ ] `go test -race ./...` exits 0
- [ ] `./bin/torch --help` prints usage
- [ ] `./bin/torch init --help` prints usage
- [ ] Config loads from YAML and applies env overrides

## Progress Notes
