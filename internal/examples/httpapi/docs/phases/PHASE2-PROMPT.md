# Phase 2 Implementation Prompts

## Prompt 1 of 2: Item Model and Store

### Required Reading
- docs/phases/PHASE2.md
- internal/handler/health.go

---

### Task

1. Create `internal/model/item.go` with an `Item` struct (ID, Name, CreatedAt)
2. Create `internal/store/memory.go` with an in-memory `ItemStore`
3. Implement `List()`, `Get(id)`, and `Create(item)` methods
4. Write table-driven tests for all store methods

### Constraints
- Use a sync.RWMutex for concurrent access
- IDs should be generated with a simple counter or UUID

---

### Verification
```bash
go build ./...
go vet ./...
go test -race ./internal/store/...
```

### Acceptance Criteria
- Store is safe for concurrent use
- All three methods have test coverage
- Tests pass with `-race`

---

## Prompt 2 of 2: Handlers, Middleware, and Routing

### Required Reading
- internal/store/memory.go
- internal/handler/health.go
- docs/phases/PHASE2.md

---

### Task

1. Create `internal/handler/items.go` with handlers for `GET /items`, `GET /items/{id}`, `POST /items`
2. Create `internal/middleware/logging.go` with a request logging middleware
3. Update `cmd/server/main.go` to register all routes with middleware
4. Write tests for each handler

### Constraints
- Use `net/http` ServeMux pattern matching (Go 1.22+)
- Return JSON responses with appropriate status codes
- Log method, path, and duration for each request

---

### Verification
```bash
go build ./...
go vet ./...
go test -race ./...
```

### Acceptance Criteria
- All endpoints return correct JSON responses
- Middleware logs each request
- Handler tests cover success and error cases
