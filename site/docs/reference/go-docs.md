---
sidebar_position: 4
title: Go Docs
---

# Go Documentation

Crux's Go API documentation is available on pkg.go.dev:

**[pkg.go.dev/github.com/roygabriel/crux →](https://pkg.go.dev/github.com/roygabriel/crux)**

## Key Packages

| Package | Import Path | Description |
|---------|-------------|-------------|
| **types** | `github.com/roygabriel/crux/pkg/types` | Shared domain types (Agent, Phase, Decision, etc.) |
| **protocol** | `github.com/roygabriel/crux/pkg/protocol` | JSON message envelope for agent communication |

## Internal Packages

These packages are internal and cannot be imported by external projects, but their documentation is useful for understanding Crux's architecture:

| Package | Description |
|---------|-------------|
| `internal/orchestrator` | Main control loop, world state, agent assignment |
| `internal/phase` | Phase engine — spec parsing, prompt contracts, gate runner |
| `internal/agent` | Agent registry, lifecycle management, messenger |
| `internal/memory` | Memory system — bank, SQLite store, vector index, journal, work notes |
| `internal/tmux` | Tmux session/pane management, watcher |
| `internal/plugin` | Agent plugin interface and registry |
| `internal/security` | Filesystem sandbox, permissions, audit logging, rate limiting |
| `internal/config` | YAML configuration loading and validation |
| `internal/tui` | Terminal UI dashboard (bubbletea) |

## Generating Docs Locally

Generate Go documentation locally using `godoc`:

```bash
go install golang.org/x/tools/cmd/godoc@latest
godoc -http=:6060
```

Then visit [http://localhost:6060/pkg/github.com/roygabriel/crux/](http://localhost:6060/pkg/github.com/roygabriel/crux/).
