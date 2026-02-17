# Phase 1A Implementation Prompts

## Prompt 1 of 4: Go Module, Directory Layout, Makefile

### Required Reading (read these files before writing code)
- README.md (repo layout)
- LLM.md (conventions)

### Task

Initialize the Go module and project skeleton.

1. Create `go.mod` with module path `github.com/roygabriel/torch` and Go 1.23+
2. Create the full directory structure from README.md
3. Create `Makefile` with targets:
   - `build`: `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/torch ./cmd/torch`
   - `test`: `go test -race -coverprofile=coverage.out ./...`
   - `lint`: `golangci-lint run ./...`
   - `vet`: `go vet ./...`
   - `coverage`: `go tool cover -func=coverage.out`
   - `clean`: remove build artifacts
4. Create `.gitignore` (bin/, coverage.out, *.exe, .env, vendor/, .torch/memory.db, .torch/vectors/, .torch/audit.log, .torch/secrets.env)
5. Create `.github/workflows/ci.yml` skeleton with test, lint, build jobs
6. Create placeholder `cmd/torch/main.go` with minimal main function

### Verification
```bash
go mod tidy
go vet ./...
```

### Acceptance Criteria
- `go mod tidy` exits 0
- Directory structure matches README.md layout
- `.gitignore` excludes sensitive `.torch/` files but not `.torch/config.yaml`

---

## Prompt 2 of 4: Core Shared Types

### Required Reading (read these files before writing code)
- docs/phases/PHASE1A.md (types list)
- docs/OVERVIEW.md (architecture, sections 5-8)
- LLM.md (naming conventions)

### Interface Contract

```go
// pkg/types/agent.go
type AgentID string
type AgentRole string   // orchestrator, project-manager, engineer
type AgentStatus string // idle, busy, error, rate-limited, stopped

type Agent struct {
    ID          AgentID     `json:"id"`
    Name        string      `json:"name"`
    Plugin      string      `json:"plugin"`       // claude, codex, gemini, generic
    Role        AgentRole   `json:"role"`
    Status      AgentStatus `json:"status"`
    Permission  Permission  `json:"permission"`
    CurrentTask string      `json:"current_task,omitempty"`
    PaneID      string      `json:"pane_id,omitempty"`
    SessionID   string      `json:"session_id,omitempty"`
}

// pkg/types/message.go
type MessageType string // task, status, decision, error, ack
type Priority string    // low, normal, high, critical

type Message struct {
    ID        string      `json:"id"`
    From      AgentID     `json:"from"`
    To        AgentID     `json:"to"`
    Type      MessageType `json:"type"`
    Priority  Priority    `json:"priority"`
    Payload   any         `json:"payload"`
    Timestamp time.Time   `json:"timestamp"`
}

// pkg/types/phase.go
type PhaseID string
type PhaseStatus string // planned, active, blocked, complete

type Phase struct {
    ID          PhaseID     `json:"id"`
    Name        string      `json:"name"`
    Status      PhaseStatus `json:"status"`
    DependsOn   []PhaseID   `json:"depends_on,omitempty"`
    TotalPrompts int        `json:"total_prompts"`
    CurrentPrompt int       `json:"current_prompt"`
}

// pkg/types/decision.go
type Decision struct {
    ID        string    `json:"id"`
    Timestamp time.Time `json:"timestamp"`
    PhaseID   PhaseID   `json:"phase_id"`
    PromptNum int       `json:"prompt_num"`
    AgentID   AgentID   `json:"agent_id"`
    Context   string    `json:"context"`
    Rationale string    `json:"rationale"`
    Action    string    `json:"action"`
    Outcome   string    `json:"outcome,omitempty"`
}

// pkg/types/memory.go
type MemoryScope string // project, session, phase, agent

type MemoryEntry struct {
    ID        string      `json:"id"`
    Scope     MemoryScope `json:"scope"`
    Key       string      `json:"key"`
    Value     string      `json:"value"`
    Tags      []string    `json:"tags,omitempty"`
    CreatedAt time.Time   `json:"created_at"`
    UpdatedAt time.Time   `json:"updated_at"`
}

// pkg/types/permission.go
type Permission string // readonly, standard, elevated, autonomous
```

### Task

Create all shared type definitions.

1. `pkg/types/agent.go` — Agent, AgentID, AgentRole, AgentStatus with constants and String() methods
2. `pkg/types/message.go` — Message, MessageType, Priority with constants
3. `pkg/types/phase.go` — Phase, PhaseID, PhaseStatus with constants
4. `pkg/types/decision.go` — Decision type
5. `pkg/types/memory.go` — MemoryEntry, MemoryScope with constants
6. `pkg/types/permission.go` — Permission type with constants and validation method `IsValid() bool`
7. `pkg/types/errors.go` — Package-level sentinel errors: ErrNotFound, ErrAlreadyExists, ErrInvalidConfig, ErrGateFailed, ErrPermissionDenied, ErrRateLimited

### Verification
```bash
go build ./...
go vet ./...
go test -race ./...
```

### Acceptance Criteria
- All types have JSON tags
- All exported types have Go doc comments
- Enum types have String() methods
- Permission has IsValid() method

---

## Prompt 3 of 4: Configuration

### Required Reading (read these files before writing code)
- docs/OVERVIEW.md (section 10 — configuration)
- configs/default.yaml (create this first)
- LLM.md (config patterns)

### Task

Create YAML configuration with environment variable overrides.

1. Create `configs/default.yaml` with all configuration fields and sensible defaults
2. Create `internal/config/config.go`:
   - `Config` struct with nested structs: ProjectConfig, AgentConfig (map), MemoryConfig, PhaseConfig, SecurityConfig
   - `Load(path string) (*Config, error)` — read YAML, apply env overrides, validate
   - `Validate() error` — check required fields, path existence, valid enums
   - Environment override pattern: `TORCH_MEMORY_SQLITE_PATH` overrides `memory.sqlite_path`
3. Create `internal/config/defaults.go` — DefaultConfig() returning zero-config defaults
4. Create `internal/config/config_test.go`:
   - Test Load with valid YAML
   - Test Load with env overrides
   - Test Validate catches missing required fields
   - Test DefaultConfig produces valid config

### Verification
```bash
go build ./...
go vet ./...
go test -race ./internal/config/...
```

### Acceptance Criteria
- Config loads from YAML file
- Environment variables override YAML values
- Validation catches invalid configuration
- Defaults produce a runnable config

---

## Prompt 4 of 4: CLI Framework

### Required Reading (read these files before writing code)
- docs/OVERVIEW.md (section 11 — CLI commands)
- internal/config/config.go
- LLM.md (conventions)

### Task

Set up cobra CLI with stub subcommands.

1. `cmd/torch/root.go` — Root command with `--config` flag (default `.torch/config.yaml`), version flag, slog setup
2. `cmd/torch/start.go` — `torch start` stub that loads config, prints "starting orchestration"
3. `cmd/torch/status.go` — `torch status` stub
4. `cmd/torch/init_cmd.go` — `torch init` that creates `.torch/` directory, copies default config, creates `docs/phases/` and `work-notes/` directories
5. `cmd/torch/main.go` — main() calls root.Execute()

### Verification
```bash
go build ./...
go vet ./...
./bin/torch --help
./bin/torch init --help
```

### Acceptance Criteria
- `torch --help` shows all subcommands
- `torch init` creates the expected directory structure
- `torch start` loads config without error
- Config flag works: `torch --config custom.yaml start`
