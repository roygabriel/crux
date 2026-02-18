# HTTP API Example

This is an example project for the Crux orchestrator. It demonstrates a two-phase
workflow that builds a simple HTTP API in Go.

## Phases

1. **Project Setup** — Initialize the Go module, directory structure, Makefile, and
   basic `main.go` with a health endpoint.
2. **API Endpoints** — Add HTTP handlers, routing, and middleware for a simple
   resource API.

## Agents

- `engineer-1` — Implements code changes with scoped write access.
- `reviewer-1` — Reviews changes with read-only access.

## Usage

```bash
crux init --example
crux start
```

The orchestrator will assign prompts to agents and advance through both phases
automatically.
