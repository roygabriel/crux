package chrome

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestPadOrTruncate_ANSIWidthSafe(t *testing.T) {
	theme := NewTheme()
	styled := theme.KeyStyle.Render("tab/shift+tab")

	got := padOrTruncate(styled, 8)
	if w := ansi.StringWidthWc(got); w != 8 {
		t.Fatalf("visible width = %d, want 8", w)
	}
}

func TestRenderLegend_SingleLineFixedWidth(t *testing.T) {
	theme := NewTheme()
	line := theme.RenderLegend(60, "panel", []LegendItem{
		{Key: "q", Action: "quit"},
		{Key: "tab/shift+tab", Action: "focus"},
		{Key: "/", Action: "filter"},
	})

	if strings.Contains(line, "\n") {
		t.Fatalf("legend should render on one line, got %q", line)
	}
	if w := ansi.StringWidthWc(line); w != 60 {
		t.Fatalf("visible width = %d, want 60", w)
	}
}
