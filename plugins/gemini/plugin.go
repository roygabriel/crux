package gemini

import (
	"fmt"
	"strings"
	"time"

	"github.com/roygabriel/crux/internal/plugin"
	"github.com/roygabriel/crux/pkg/types"
)

const pluginName = "gemini"

// busyTailLines is the number of lines from the end of pane content
// checked for busy indicators.
const busyTailLines = 5

// Compile-time interface compliance check.
var _ plugin.AgentPlugin = (*Plugin)(nil)

// Plugin implements plugin.AgentPlugin for the Google Gemini CLI.
type Plugin struct{}

// New creates a new Gemini CLI plugin instance.
func New() *Plugin {
	return &Plugin{}
}

// Name returns "gemini".
func (p *Plugin) Name() string { return pluginName }

// LaunchCmd returns the command to start Gemini CLI with appropriate flags
// based on the agent configuration. Elevated and autonomous permissions
// disable sandboxing via --sandbox none.
func (p *Plugin) LaunchCmd(cfg plugin.AgentConfig) (string, []string, error) {
	if cfg.WorkDir == "" {
		return "", nil, fmt.Errorf("launch gemini: work directory must not be empty")
	}

	var args []string

	switch cfg.Permission {
	case types.PermElevated, types.PermAutonomous:
		args = append(args, "--sandbox", "none")
	}

	args = append(args, cfg.ExtraArgs...)

	return "gemini", args, nil
}

// DetectReady returns true if the last non-empty line of pane content
// shows a Gemini CLI ready prompt (">" or "gemini>").
func (p *Plugin) DetectReady(paneContent string) bool {
	line := lastNonEmptyLine(paneContent)
	return isReadyPrompt(line)
}

// DetectBusy returns true if the tail of pane content contains spinner
// characters or activity text indicating Gemini CLI is processing.
func (p *Plugin) DetectBusy(paneContent string) bool {
	tail := stripANSI(lastLines(paneContent, busyTailLines))
	return busyPatternRe.MatchString(tail)
}

// DetectError inspects pane content for error messages. It returns the
// first error message found and true, or empty string and false if no
// errors are detected. Panics are checked before standard errors.
func (p *Plugin) DetectError(paneContent string) (string, bool) {
	cleaned := stripANSI(paneContent)

	if match := panicRe.FindStringSubmatch(cleaned); match != nil {
		return strings.TrimSpace(match[1] + " " + match[2]), true
	}

	if match := errorMsgRe.FindStringSubmatch(cleaned); match != nil {
		return strings.TrimSpace(match[1]), true
	}

	return "", false
}

// DetectRateLimit inspects pane content for rate-limiting signals.
// If detected, it parses the retry duration from the message or falls
// back to a 60-second default.
func (p *Plugin) DetectRateLimit(paneContent string) (time.Duration, bool) {
	cleaned := stripANSI(paneContent)
	if !rateLimitRe.MatchString(cleaned) {
		return 0, false
	}
	return parseRetryDuration(cleaned), true
}

// FormatMessage converts a Message into a text prompt suitable for
// sending to Gemini CLI via tmux send-keys.
func (p *Plugin) FormatMessage(msg types.Message) string {
	switch msg.Type {
	case types.MessageTask:
		return formatTaskPayload(msg.Payload)
	default:
		return fmt.Sprintf("%v", msg.Payload)
	}
}

// ParseOutput extracts structured information from raw pane content
// captured after Gemini CLI finishes processing.
func (p *Plugin) ParseOutput(paneContent string) (plugin.AgentOutput, error) {
	cleaned := stripANSI(paneContent)

	return plugin.AgentOutput{
		Raw:          paneContent,
		FilesChanged: extractFilesChanged(cleaned),
		Errors:       extractErrors(cleaned),
		IsComplete:   isReadyPrompt(lastNonEmptyLine(paneContent)),
	}, nil
}

// Capabilities returns the capabilities supported by Gemini CLI:
// code generation, file editing, shell execution, and web search.
func (p *Plugin) Capabilities() []plugin.Capability {
	return []plugin.Capability{
		plugin.CapCodeGen,
		plugin.CapFileEdit,
		plugin.CapShellExec,
		plugin.CapWebSearch,
	}
}

// formatTaskPayload converts a message payload into a natural language
// task prompt for Gemini CLI.
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
