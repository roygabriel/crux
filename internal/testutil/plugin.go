package testutil

import (
	"strings"
	"sync"
	"time"

	"github.com/roygabriel/crux/internal/plugin"
	"github.com/roygabriel/crux/pkg/types"
)

// Sentinel content strings used by ScenarioPlugin for status detection.
const (
	ContentReady     = ">"
	ContentBusy      = "\u280b thinking..."
	ContentError     = "Error: fatal"
	ContentRateLimit = "rate-limit 429"
)

// ParseOutputFn is a custom function for overriding ParseOutput behavior.
type ParseOutputFn func(content string) (plugin.AgentOutput, error)

// ScenarioPlugin is a test plugin.AgentPlugin that uses sentinel strings
// in pane content for deterministic status detection.
type ScenarioPlugin struct {
	mu            sync.Mutex
	name          string
	parseOutputFn ParseOutputFn
	caps          []plugin.Capability
}

// NewScenarioPlugin creates a ScenarioPlugin with the given name.
func NewScenarioPlugin(name string) *ScenarioPlugin {
	return &ScenarioPlugin{
		name: name,
		caps: []plugin.Capability{plugin.CapCodeGen, plugin.CapShellExec},
	}
}

// SetParseOutputFn configures a custom ParseOutput function.
func (p *ScenarioPlugin) SetParseOutputFn(fn ParseOutputFn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.parseOutputFn = fn
}

// Name returns the plugin name.
func (p *ScenarioPlugin) Name() string { return p.name }

// LaunchCmd returns a no-op launch command.
func (p *ScenarioPlugin) LaunchCmd(_ plugin.AgentConfig) (string, []string, error) {
	return "echo", []string{"mock"}, nil
}

// DetectReady returns true when content exactly equals the ready sentinel.
func (p *ScenarioPlugin) DetectReady(content string) bool {
	return content == ContentReady
}

// DetectBusy returns true when content contains "thinking".
func (p *ScenarioPlugin) DetectBusy(content string) bool {
	return strings.Contains(content, "thinking")
}

// DetectError returns the error message and true when content starts with "Error:".
func (p *ScenarioPlugin) DetectError(content string) (string, bool) {
	if strings.HasPrefix(content, "Error:") {
		return strings.TrimPrefix(content, "Error: "), true
	}
	return "", false
}

// DetectRateLimit returns a retry duration and true when content contains "rate-limit".
func (p *ScenarioPlugin) DetectRateLimit(content string) (time.Duration, bool) {
	if strings.Contains(content, "rate-limit") {
		return 30 * time.Second, true
	}
	return 0, false
}

// FormatMessage returns an empty string (no-op for tests).
func (p *ScenarioPlugin) FormatMessage(_ types.Message) string { return "" }

// ParseOutput returns AgentOutput with IsComplete: true by default,
// or delegates to the custom function if set.
func (p *ScenarioPlugin) ParseOutput(content string) (plugin.AgentOutput, error) {
	p.mu.Lock()
	fn := p.parseOutputFn
	p.mu.Unlock()

	if fn != nil {
		return fn(content)
	}
	return plugin.AgentOutput{
		Raw:        content,
		IsComplete: true,
	}, nil
}

// Capabilities returns the plugin's capability set.
func (p *ScenarioPlugin) Capabilities() []plugin.Capability { return p.caps }
