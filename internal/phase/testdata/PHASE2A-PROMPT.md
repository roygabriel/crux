# Phase 2A Implementation Prompts

## Prompt 1 of 4: Agent Plugin Interface

### Required Reading (read these files before writing code)
- docs/phases/PHASE2A.md
- docs/OVERVIEW.md (section 5 — agent plugin system)
- pkg/types/agent.go
- LLM.md (conventions)

### Interface Contract

```go
// internal/plugin/interface.go
type AgentPlugin interface {
    Name() string
    LaunchCmd(cfg AgentConfig) (bin string, args []string, err error)
    DetectReady(paneContent string) bool
    DetectBusy(paneContent string) bool
    DetectError(paneContent string) (errMsg string, isError bool)
    DetectRateLimit(paneContent string) (retryAfter time.Duration, isLimited bool)
    FormatMessage(msg types.Message) string
    ParseOutput(paneContent string) (AgentOutput, error)
    Capabilities() []Capability
}

type AgentConfig struct {
    ID            types.AgentID
    WorkDir       string
    Permission    types.Permission
    ExtraArgs     []string
    Environment   map[string]string
}

type AgentOutput struct {
    Raw           string
    FilesChanged  []string
    Decisions     []OutputDecision
    Errors        []string
    IsComplete    bool
}

type OutputDecision struct {
    Decision  string
    Rationale string
}

type Capability string // const: CapCodeGen, CapFileEdit, CapShellExec, CapWebSearch
```

### Task

1. `internal/plugin/interface.go` — Define AgentPlugin, AgentConfig, AgentOutput, Capability
2. `internal/plugin/registry.go` — PluginRegistry: Register(name, factory), Get(name), List()
3. `internal/plugin/registry_test.go` — Test register, get, list, get unknown returns error

### Verification
```bash
go build ./...
go vet ./...
go test -race ./internal/plugin/...
```

### Acceptance Criteria
- Interface is comprehensive enough for Claude, Codex, and Gemini
- Registry is thread-safe
- Unknown plugin name returns typed error

---

## Prompt 2 of 4: Claude Code Plugin Adapter

### Required Reading (read these files before writing code)
- internal/plugin/interface.go
- docs/OVERVIEW.md (section 5 — Claude Code details)

### Task

1. `plugins/claude/plugin.go`:
   - Implement AgentPlugin for Claude Code CLI
   - `LaunchCmd`: returns `claude` binary with optional flags. If Permission is autonomous, add `--dangerously-skip-permissions`. Add `--output-format json` when available.
   - `DetectReady`: look for `>` prompt or `claude>` prompt at end of pane content
   - `DetectBusy`: look for spinner characters or "Thinking..." patterns
   - `DetectError`: look for "Error:", "error:", stack traces
   - `DetectRateLimit`: look for "rate limit", "429", extract retry duration
   - `FormatMessage`: format Message payload as natural language task prompt
   - `ParseOutput`: extract file changes, decisions, errors from agent output
   - `Capabilities`: CapCodeGen, CapFileEdit, CapShellExec
2. `plugins/claude/detect.go` — regex patterns for detection methods
3. `plugins/claude/plugin_test.go` — test each detection method with sample pane content

### Verification
```bash
go build ./...
go vet ./...
go test -race ./plugins/claude/...
```

### Acceptance Criteria
- Launch command respects permission tier
- Detection methods handle real Claude Code output patterns
- No false positives on ready/busy detection

---

## Prompt 3 of 4: Agent Registry

### Required Reading (read these files before writing code)
- internal/plugin/interface.go
- internal/tmux/session.go
- internal/tmux/pane.go
- pkg/types/agent.go

### Interface Contract

```go
// internal/agent/registry.go
type Registry struct { /* ... */ }

func NewRegistry(sm *tmux.SessionManager, pm *tmux.PaneManager, plugins *plugin.Registry, logger *slog.Logger) *Registry
func (r *Registry) Spawn(ctx context.Context, cfg types.Agent) error
func (r *Registry) Get(id types.AgentID) (*AgentInstance, error)
func (r *Registry) List() []*AgentInstance
func (r *Registry) Kill(ctx context.Context, id types.AgentID) error
func (r *Registry) UpdateStatus(id types.AgentID, status types.AgentStatus) error
```

### Task

1. `internal/agent/registry.go` — AgentInstance struct (Agent + plugin + paneID + launchedAt), Registry with CRUD
2. `internal/agent/lifecycle.go` — Spawn creates tmux pane, runs launch command; Kill sends exit + kills pane
3. Tests with mock tmux Commander

### Verification
```bash
go build ./...
go vet ./...
go test -race ./internal/agent/...
```

### Acceptance Criteria
- Spawn creates tmux pane and launches agent CLI
- Kill gracefully stops agent before killing pane
- Registry is thread-safe for concurrent access
- Duplicate agent ID returns error

---

## Prompt 4 of 4: Agent Messenger

### Required Reading (read these files before writing code)
- internal/agent/registry.go
- internal/tmux/pane.go
- pkg/types/message.go
- pkg/protocol/ (create this)

### Task

1. `pkg/protocol/envelope.go` — JSON envelope: Marshal/Unmarshal Message to/from JSON
2. `internal/agent/messenger.go`:
   - `Messenger` struct wrapping PaneManager and agent Registry
   - `Send(ctx, agentID, msg Message) error` — format via plugin, send via tmux send-keys
   - `WaitForResponse(ctx, agentID, timeout) (string, error)` — poll capture-pane until agent is no longer busy or timeout
3. Tests

### Verification
```bash
go build ./...
go vet ./...
go test -race ./...
```

### Acceptance Criteria
- Messages are formatted by the target agent's plugin before sending
- WaitForResponse respects context cancellation and timeout
- Large messages are chunked to stay under tmux send-keys limits
