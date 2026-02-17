# Phase 2A: Agent Plugin Interface, Claude Code Adapter, Agent Registry

## Status
Planned

## Depends On
Phase 1B

## Design Rationale
Define the AgentPlugin interface that all CLI adapters implement, build the Claude Code adapter first (primary use case), and create the agent registry that manages plugin instances. This is the abstraction layer between the orchestrator and any CLI tool.

## Tasks

### Prompt 1
- AgentPlugin interface, AgentConfig, AgentOutput types

### Prompt 2
- Claude Code plugin adapter

### Prompt 3
- Agent registry (register, get, list, lifecycle management)

### Prompt 4
- Agent messenger (send message to agent via tmux, parse response)

## Files

### New
- internal/plugin/interface.go
- internal/plugin/registry.go
- internal/plugin/registry_test.go
- plugins/claude/plugin.go
- plugins/claude/plugin_test.go
- plugins/claude/detect.go
- internal/agent/registry.go
- internal/agent/registry_test.go
- internal/agent/lifecycle.go
- internal/agent/lifecycle_test.go
- internal/agent/messenger.go
- internal/agent/messenger_test.go

### Referenced (read-only)
- internal/tmux/session.go
- internal/tmux/pane.go
- pkg/types/agent.go
- pkg/types/message.go

## Exit Criteria
- [ ] `go build ./...` exits 0
- [ ] `go vet ./...` exits 0
- [ ] `go test -race ./...` exits 0
- [ ] Claude plugin correctly generates launch command
- [ ] Agent registry manages full lifecycle (spawn → monitor → kill)

## Progress Notes
