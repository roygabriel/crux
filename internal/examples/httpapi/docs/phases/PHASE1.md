# Phase 1: Project Setup

## Objective

Initialize the Go project with a working build, a health check endpoint, and
standard project scaffolding.

## Scope

- Go module initialization
- Directory structure (`cmd/`, `internal/`, `Makefile`)
- Basic `main.go` with an HTTP server and `/healthz` endpoint
- Makefile with `build`, `test`, `lint`, and `run` targets

## Tasks

1. Initialize Go module and create directory layout
2. Implement health check server with graceful shutdown

## Dependencies

None — this is the first phase.

## Exit Criteria

- `go build ./...` exits 0
- `go vet ./...` exits 0
- `go test -race ./...` exits 0
- `curl localhost:8080/healthz` returns 200
