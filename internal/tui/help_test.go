package tui

import (
	"strings"
	"testing"
)

func TestHelpOverlay_DefaultNotVisible(t *testing.T) {
	h := HelpOverlay{}
	if h.IsVisible() {
		t.Error("help overlay should not be visible by default")
	}
}

func TestHelpOverlay_Toggle(t *testing.T) {
	h := HelpOverlay{}
	h.Toggle()
	if !h.IsVisible() {
		t.Error("help should be visible after first toggle")
	}
	h.Toggle()
	if h.IsVisible() {
		t.Error("help should not be visible after second toggle")
	}
}

func TestHelpOverlay_ShowDismiss(t *testing.T) {
	h := HelpOverlay{}
	h.Show()
	if !h.IsVisible() {
		t.Error("help should be visible after Show")
	}
	h.Dismiss()
	if h.IsVisible() {
		t.Error("help should not be visible after Dismiss")
	}
}

func TestHelpOverlay_ViewContainsKeyBindings(t *testing.T) {
	h := HelpOverlay{visible: true, width: 80, height: 40}
	view := h.View()

	keys := []string{"q", "tab", "o/d/n", "?", "i", "/"}
	for _, k := range keys {
		if !strings.Contains(view, k) {
			t.Errorf("View should contain key %q", k)
		}
	}
}

func TestHelpOverlay_ViewContainsSectionHeaders(t *testing.T) {
	h := HelpOverlay{visible: true, width: 80, height: 40}
	view := h.View()

	headers := []string{"Global", "Sidebar", "Content Panel", "Logs Panel"}
	for _, hdr := range headers {
		if !strings.Contains(view, hdr) {
			t.Errorf("View should contain section %q", hdr)
		}
	}
}

func TestHelpOverlay_ViewContainsDismissHint(t *testing.T) {
	h := HelpOverlay{visible: true, width: 80, height: 40}
	view := h.View()

	if !strings.Contains(view, "Press any key to dismiss") {
		t.Error("View should contain dismiss hint")
	}
}
