package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Help overlay styles.
var (
	helpOverlayStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("235")).
				Foreground(lipgloss.Color("252")).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("141")).
				Padding(1, 2)
	helpTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("141"))
	helpKeyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
)

// HelpOverlay displays a centered keybinding reference card.
type HelpOverlay struct {
	visible bool
	width   int
	height  int
}

// Toggle flips the visibility of the help overlay.
func (h *HelpOverlay) Toggle() {
	h.visible = !h.visible
}

// Show makes the help overlay visible.
func (h *HelpOverlay) Show() {
	h.visible = true
}

// Dismiss hides the help overlay.
func (h *HelpOverlay) Dismiss() {
	h.visible = false
}

// IsVisible returns whether the help overlay is currently shown.
func (h *HelpOverlay) IsVisible() bool {
	return h.visible
}

// SetSize sets the available dimensions for centering.
func (h *HelpOverlay) SetSize(w, height int) {
	h.width = w
	h.height = height
}

// View renders the help overlay as a centered box with keybinding reference.
func (h *HelpOverlay) View() string {
	var b strings.Builder

	b.WriteString(helpTitleStyle.Render("Keyboard Shortcuts"))
	b.WriteString("\n\n")

	sections := []struct {
		title string
		keys  [][2]string
	}{
		{
			title: "Global",
			keys: [][2]string{
				{"q", "Quit"},
				{"?", "Toggle help"},
				{"tab", "Switch panel"},
			},
		},
		{
			title: "Agents Panel",
			keys: [][2]string{
				{"j/k", "Move cursor"},
				{"enter", "Open detail"},
				{"p", "Pause agent"},
				{"r", "Resume agent"},
				{"x", "Kill agent"},
			},
		},
		{
			title: "Logs Panel",
			keys: [][2]string{
				{"j/k", "Scroll up/down"},
				{"pgup/pgdn", "Scroll page"},
				{"f", "Filter logs"},
			},
		},
		{
			title: "Detail Panel",
			keys: [][2]string{
				{"esc", "Close detail"},
				{"m", "Send message"},
			},
		},
	}

	for _, sec := range sections {
		b.WriteString(helpTitleStyle.Render(sec.title))
		b.WriteByte('\n')
		for _, kv := range sec.keys {
			b.WriteString(fmt.Sprintf("  %s  %s\n", helpKeyStyle.Render(padOrTruncate(kv[0], 12)), kv[1]))
		}
		b.WriteByte('\n')
	}

	b.WriteString("Press any key to dismiss")

	content := helpOverlayStyle.Render(b.String())
	return lipgloss.Place(h.width, h.height, lipgloss.Center, lipgloss.Center, content)
}
