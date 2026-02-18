# Phase 2: API Endpoints

## Objective

Add CRUD-style HTTP endpoints for a simple "items" resource with routing
and request logging middleware.

## Scope

- Item model and in-memory store
- HTTP handlers for list, get, create
- Request logging middleware
- Route registration

## Tasks

1. Create item model, store, and handlers
2. Add middleware and wire routes

## Dependencies

- Phase 1 (project setup must be complete)

## Exit Criteria

- `go build ./...` exits 0
- `go vet ./...` exits 0
- `go test -race ./...` exits 0
- All handlers have test coverage
