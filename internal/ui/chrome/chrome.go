package chrome

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// LegendItem represents one key/action hint in the footer command list.
type LegendItem struct {
	Key    string
	Action string
}

// Theme contains shared lipgloss styles for the terminal dashboards.
type Theme struct {
	FocusedBorder   lipgloss.Style
	UnfocusedBorder lipgloss.Style
	TopBar          lipgloss.Style
	FooterBar       lipgloss.Style
	PanelTitle      lipgloss.Style
	PanelMeta       lipgloss.Style
	KeyStyle        lipgloss.Style
	MutedText       lipgloss.Style
}

// NewTheme returns the default board-like 256-color palette.
func NewTheme() Theme {
	return Theme{
		FocusedBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("81")),
		UnfocusedBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")),
		TopBar: lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252")),
		FooterBar: lipgloss.NewStyle().
			Background(lipgloss.Color("234")).
			Foreground(lipgloss.Color("250")),
		PanelTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("81")),
		PanelMeta: lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")),
		KeyStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("223")),
		MutedText: lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")),
	}
}

// PanelStyle returns a border style for focused/unfocused panel rendering.
func (t Theme) PanelStyle(focused bool) lipgloss.Style {
	if focused {
		return t.FocusedBorder
	}
	return t.UnfocusedBorder
}

// RenderHeaderBar renders a three-section header line.
func (t Theme) RenderHeaderBar(width int, left, center, right string) string {
	if width <= 0 {
		return ""
	}

	leftW := width / 3
	centerW := width / 3
	rightW := width - leftW - centerW

	row := padOrTruncate(left, leftW) +
		padOrTruncate(center, centerW) +
		padOrTruncate(right, rightW)
	return t.TopBar.Render(row)
}

// RenderLegend renders a context-sensitive command list in a single line.
func (t Theme) RenderLegend(width int, scope string, items []LegendItem) string {
	if width <= 0 {
		return ""
	}

	var parts []string
	if scope != "" {
		parts = append(parts, scope+":")
	}
	for _, item := range items {
		if item.Key == "" || item.Action == "" {
			continue
		}
		parts = append(parts, t.KeyStyle.Render(item.Key)+" "+item.Action)
	}

	line := strings.Join(parts, "  ")
	return t.FooterBar.Render(padOrTruncate(line, width))
}

func padOrTruncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidthWc(s) > width {
		tail := ""
		if width > 3 {
			tail = "..."
		}
		s = ansi.TruncateWc(s, width, tail)
	}
	if pad := width - ansi.StringWidthWc(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}
