# Phase 1 Implementation Prompts

## Prompt 1 of 2: Initialize Project Structure

### Required Reading
- docs/phases/PHASE1.md

---

### Task

1. Run `go mod init example.com/http-api`
2. Create directory layout: `cmd/server/`, `internal/handler/`
3. Create `Makefile` with targets: `build`, `test`, `lint`, `run`
4. Create a minimal `cmd/server/main.go` that prints "server starting"

### Constraints
- Use Go 1.21 or later
- No external dependencies in this prompt

---

### Verification
```bash
go build ./...
```

### Acceptance Criteria
- Project compiles with `go build ./...`
- Makefile exists with all four targets

---

## Prompt 2 of 2: Health Check Server

### Required Reading
- cmd/server/main.go
- docs/phases/PHASE1.md

---

### Task

1. Update `cmd/server/main.go` to start an HTTP server on `:8080`
2. Create `internal/handler/health.go` with a `HealthHandler` returning 200 and `{"status":"ok"}`
3. Register the handler at `/healthz`
4. Add graceful shutdown using `signal.NotifyContext`
5. Write tests for the health handler

### Constraints
- Use only the standard library
- Handler must set `Content-Type: application/json`

---

### Verification
```bash
go build ./...
go vet ./...
go test -race ./...
```

### Acceptance Criteria
- Server starts and responds to `/healthz` with 200
- Health handler has test coverage
- Graceful shutdown on SIGINT/SIGTERM
