package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/roygabriel/crux/pkg/types"
)

// Status indicator styles.
var (
	statusDotIdle        = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))  // green
	statusDotBusy        = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))  // yellow
	statusDotError       = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))  // red
	statusDotRateLimited = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))  // cyan
	statusDotStopped     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))  // gray
	statusDotDefault     = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))  // white
	headerStyle          = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	selectedRowStyle     = lipgloss.NewStyle().Background(lipgloss.Color("236")) // dark gray bg
	confirmStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3")) // yellow bold
)

// styledStatusDot returns a colored bullet character for the given status.
func styledStatusDot(status types.AgentStatus) string {
	switch status {
	case types.StatusIdle:
		return statusDotIdle.Render("\u25cf")
	case types.StatusBusy:
		return statusDotBusy.Render("\u25cf")
	case types.StatusError:
		return statusDotError.Render("\u25cf")
	case types.StatusRateLimited:
		return statusDotRateLimited.Render("\u25cf")
	case types.StatusStopped:
		return statusDotStopped.Render("\u25cf")
	default:
		return statusDotDefault.Render("\u25cf")
	}
}

// styledStatusLabel returns the status dot followed by the status text.
func styledStatusLabel(status types.AgentStatus) string {
	return fmt.Sprintf("%s %s", styledStatusDot(status), status)
}

// truncateRole shortens long role names for display.
func truncateRole(role types.AgentRole, maxLen int) string {
	s := string(role)
	if len(s) > maxLen {
		if maxLen <= 1 {
			return s[:maxLen]
		}
		return s[:maxLen-1] + "."
	}
	return s
}

// padOrTruncate pads s with spaces to width, or truncates with "..." if too long.
func padOrTruncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) > width {
		if width <= 3 {
			return string(runes[:width])
		}
		return string(runes[:width-3]) + "..."
	}
	if len(runes) < width {
		return s + strings.Repeat(" ", width-len(runes))
	}
	return s
}
