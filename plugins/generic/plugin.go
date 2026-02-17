package generic

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/roygabriel/crux/internal/plugin"
	"github.com/roygabriel/crux/pkg/types"
)

// busyTailLines is the number of lines from the end of pane content
// checked for busy indicators.
const busyTailLines = 5

const defaultRetryAfter = 60 * time.Second

// ansiRe matches ANSI escape sequences for stripping from pane content.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// retryAfterRe extracts a numeric duration from retry/wait messages.
var retryAfterRe = regexp.MustCompile(`(?i)(?:retry|wait)\s*(?:after\s+)?(\d+)\s*(s|sec|seconds?|m|min|minutes?)`)

// Compile-time interface compliance check.
var _ plugin.AgentPlugin = (*Plugin)(nil)

// GenericPluginConfig holds user-supplied configuration for a generic
// plugin adapter. Regex patterns are compiled at construction time.
type GenericPluginConfig struct {
	// Name is the unique identifier for this plugin instance.
	Name string `yaml:"name" json:"name"`
	// Binary is the executable to launch.
	Binary string `yaml:"binary" json:"binary"`
	// Args are default CLI arguments prepended before extra args.
	Args []string `yaml:"args" json:"args,omitempty"`
	// ReadyPattern is a regex matched against the last non-empty line.
	ReadyPattern string `yaml:"ready_pattern" json:"ready_pattern"`
	// BusyPattern is a regex matched against the tail of pane content.
	BusyPattern string `yaml:"busy_pattern" json:"busy_pattern"`
	// ErrorPattern is a regex with a capture group for the error message.
	ErrorPattern string `yaml:"error_pattern" json:"error_pattern"`
	// RateLimitPattern is a regex for rate-limit detection.
	RateLimitPattern string `yaml:"rate_limit_pattern" json:"rate_limit_pattern,omitempty"`
	// Capabilities lists the capability strings this plugin supports.
	Capabilities []string `yaml:"capabilities" json:"capabilities,omitempty"`
}

// Plugin implements plugin.AgentPlugin using user-supplied regex patterns
// from a GenericPluginConfig.
type Plugin struct {
	name     string
	binary   string
	args     []string
	readyRe  *regexp.Regexp
	busyRe   *regexp.Regexp
	errorRe  *regexp.Regexp
	rateLimitRe *regexp.Regexp
	caps     []plugin.Capability
}

// New creates a new generic plugin from the given configuration. It compiles
// all regex patterns at construction time and returns an error for invalid patterns.
func New(cfg GenericPluginConfig) (*Plugin, error) {
	p := &Plugin{
		name:   cfg.Name,
		binary: cfg.Binary,
		args:   cfg.Args,
	}

	var err error

	if cfg.ReadyPattern != "" {
		p.readyRe, err = regexp.Compile(cfg.ReadyPattern)
		if err != nil {
			return nil, fmt.Errorf("compile ready_pattern: %w", err)
		}
	}

	if cfg.BusyPattern != "" {
		p.busyRe, err = regexp.Compile(cfg.BusyPattern)
		if err != nil {
			return nil, fmt.Errorf("compile busy_pattern: %w", err)
		}
	}

	if cfg.ErrorPattern != "" {
		p.errorRe, err = regexp.Compile(cfg.ErrorPattern)
		if err != nil {
			return nil, fmt.Errorf("compile error_pattern: %w", err)
		}
	}

	if cfg.RateLimitPattern != "" {
		p.rateLimitRe, err = regexp.Compile(cfg.RateLimitPattern)
		if err != nil {
			return nil, fmt.Errorf("compile rate_limit_pattern: %w", err)
		}
	}

	for _, c := range cfg.Capabilities {
		p.caps = append(p.caps, plugin.Capability(c))
	}

	return p, nil
}

// Name returns the plugin's unique identifier.
func (p *Plugin) Name() string { return p.name }

// LaunchCmd returns the binary and arguments for launching the generic agent.
func (p *Plugin) LaunchCmd(cfg plugin.AgentConfig) (string, []string, error) {
	if cfg.WorkDir == "" {
		return "", nil, fmt.Errorf("launch %s: work directory must not be empty", p.name)
	}

	args := make([]string, len(p.args))
	copy(args, p.args)
	args = append(args, cfg.ExtraArgs...)

	return p.binary, args, nil
}

// DetectReady returns true if the last non-empty line matches the ready pattern.
// Returns false if no ready pattern is configured.
func (p *Plugin) DetectReady(paneContent string) bool {
	if p.readyRe == nil {
		return false
	}
	line := lastNonEmptyLine(paneContent)
	return p.readyRe.MatchString(line)
}

// DetectBusy returns true if the tail of pane content matches the busy pattern.
// Returns false if no busy pattern is configured.
func (p *Plugin) DetectBusy(paneContent string) bool {
	if p.busyRe == nil {
		return false
	}
	tail := stripANSI(lastLines(paneContent, busyTailLines))
	return p.busyRe.MatchString(tail)
}

// DetectError returns the first error match from pane content.
// Returns empty string and false if no error pattern is configured.
func (p *Plugin) DetectError(paneContent string) (string, bool) {
	if p.errorRe == nil {
		return "", false
	}
	cleaned := stripANSI(paneContent)
	match := p.errorRe.FindStringSubmatch(cleaned)
	if match == nil {
		return "", false
	}
	if len(match) > 1 {
		return strings.TrimSpace(match[1]), true
	}
	return strings.TrimSpace(match[0]), true
}

// DetectRateLimit returns the retry duration if the rate limit pattern matches.
// Returns 0 and false if no rate limit pattern is configured.
func (p *Plugin) DetectRateLimit(paneContent string) (time.Duration, bool) {
	if p.rateLimitRe == nil {
		return 0, false
	}
	cleaned := stripANSI(paneContent)
	if !p.rateLimitRe.MatchString(cleaned) {
		return 0, false
	}
	return parseRetryDuration(cleaned), true
}

// FormatMessage converts a Message into a text prompt suitable for sending
// to the generic agent via tmux send-keys.
func (p *Plugin) FormatMessage(msg types.Message) string {
	switch msg.Type {
	case types.MessageTask:
		return formatTaskPayload(msg.Payload)
	default:
		return fmt.Sprintf("%v", msg.Payload)
	}
}

// ParseOutput extracts structured information from raw pane content.
func (p *Plugin) ParseOutput(paneContent string) (plugin.AgentOutput, error) {
	return plugin.AgentOutput{
		Raw:        paneContent,
		IsComplete: p.DetectReady(paneContent),
	}, nil
}

// Capabilities returns the capabilities configured for this generic plugin.
func (p *Plugin) Capabilities() []plugin.Capability {
	return p.caps
}

// stripANSI removes ANSI escape sequences from s.
func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// lastNonEmptyLine returns the last non-empty line from s after stripping
// ANSI escape sequences and trimming whitespace.
func lastNonEmptyLine(s string) string {
	s = stripANSI(s)
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if trimmed := strings.TrimSpace(lines[i]); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// lastLines returns the last n lines of s as a single string.
func lastLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// parseRetryDuration extracts a retry duration from s, returning
// defaultRetryAfter if no parseable duration is found.
func parseRetryDuration(s string) time.Duration {
	matches := retryAfterRe.FindStringSubmatch(s)
	if matches == nil {
		return defaultRetryAfter
	}

	var val int
	if _, err := fmt.Sscanf(matches[1], "%d", &val); err != nil {
		return defaultRetryAfter
	}

	unit := strings.ToLower(matches[2])
	switch {
	case strings.HasPrefix(unit, "m"):
		return time.Duration(val) * time.Minute
	default:
		return time.Duration(val) * time.Second
	}
}

// formatTaskPayload converts a message payload into a text prompt.
func formatTaskPayload(payload any) string {
	switch v := payload.(type) {
	case string:
		return v
	case map[string]any:
		var sb strings.Builder

		if task, ok := v["task"].(string); ok {
			sb.WriteString(task)
		}

		if files, ok := v["context_files"].([]any); ok && len(files) > 0 {
			sb.WriteString("\n\nContext files:")
			for _, f := range files {
				fmt.Fprintf(&sb, "\n- %v", f)
			}
		}

		if sb.Len() == 0 {
			return fmt.Sprintf("%v", v)
		}
		return sb.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}
