---
sidebar_position: 3
title: Custom Plugins
---

# Custom Plugins

Crux ships with adapters for Claude Code, Codex CLI, and Gemini CLI. You can add support for any CLI tool using the **Generic plugin** or by writing a Go adapter.

## Using the Generic Plugin

The generic plugin uses regex patterns to detect agent state. Configure it in `.crux/config.yaml`:

```yaml
generic_plugins:
  my-tool:
    name: "My Custom Tool"
    binary: "my-tool"
    args: ["--interactive"]
    ready_pattern: "^> $"
    busy_pattern: "\\.\\.\\.$"
    error_pattern: "Error: (.+)"
    rate_limit_pattern: "rate limit"
    capabilities: ["code_generation", "file_editing"]
```

Then reference it in your agent config:

```yaml
agents:
  custom-agent:
    plugin: my-tool
    role: engineer
    permission: standard
```

### Pattern Reference

| Pattern | Purpose | Match Target |
|---------|---------|-------------|
| `ready_pattern` | Agent is ready for input | Last non-empty line |
| `busy_pattern` | Agent is processing | Tail of pane content |
| `error_pattern` | Agent encountered an error | Capture group 1 = error message |
| `rate_limit_pattern` | Rate limit detected | Pane content |

## Writing a Go Adapter

For full control, implement the `AgentPlugin` interface in a new package under `plugins/`:

```go
package mytool

import (
    "github.com/roygabriel/crux/internal/plugin"
    "github.com/roygabriel/crux/pkg/types"
)

type Plugin struct{}

func (p *Plugin) Name() string { return "my-tool" }

func (p *Plugin) LaunchCmd(cfg types.AgentConfig) (string, []string, error) {
    return "my-tool", []string{"--interactive"}, nil
}

func (p *Plugin) DetectReady(paneContent string) bool {
    // Return true when the tool is ready for input
}

func (p *Plugin) DetectBusy(paneContent string) bool {
    // Return true when the tool is processing
}

func (p *Plugin) DetectError(paneContent string) (string, bool) {
    // Return error message and true if an error is detected
}

func (p *Plugin) DetectRateLimit(paneContent string) (time.Duration, bool) {
    // Return backoff duration and true if rate limited
}

func (p *Plugin) FormatMessage(msg types.Message) string {
    // Format an orchestrator message for this tool's input
}

func (p *Plugin) ParseOutput(paneContent string) (types.AgentOutput, error) {
    // Parse the tool's output into structured form
}

func (p *Plugin) Capabilities() []types.Capability {
    return []types.Capability{types.CapCodeGeneration}
}
```

Register the plugin in `internal/pluginloader/loader.go` to make it available by name.

## Testing Plugins

Follow the pattern used by existing adapters. Write table-driven tests for each detection method:

```go
func TestDetectReady(t *testing.T) {
    tests := []struct {
        name    string
        content string
        want    bool
    }{
        {"prompt visible", "output\n> ", true},
        {"still processing", "thinking...", false},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            p := &Plugin{}
            if got := p.DetectReady(tt.content); got != tt.want {
                t.Errorf("DetectReady() = %v, want %v", got, tt.want)
            }
        })
    }
}
```
