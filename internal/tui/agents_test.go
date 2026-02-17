package tui

import (
	"strings"
	"testing"

	"github.com/roygabriel/crux/pkg/types"
)

func TestAgentsPanel_RendersCorrectRowCount(t *testing.T) {
	p := NewAgentsPanel()
	p.SetSize(120, 20)
	p.SetAgents([]AgentSnapshot{
		{ID: "a1", Name: "agent-1", Plugin: "claude", Role: types.RoleEngineer, Status: types.StatusIdle},
		{ID: "a2", Name: "agent-2", Plugin: "claude", Role: types.RoleEngineer, Status: types.StatusBusy},
	})

	view := p.View()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")

	// 1 header + 2 agent rows = 3 lines.
	if len(lines) != 3 {
		t.Errorf("line count = %d, want 3", len(lines))
	}
}

func TestAgentsPanel_StatusColors(t *testing.T) {
	tests := []struct {
		status types.AgentStatus
		label  string
	}{
		{types.StatusIdle, "idle"},
		{types.StatusBusy, "busy"},
		{types.StatusError, "error"},
		{types.StatusRateLimited, "rate-limited"},
		{types.StatusStopped, "stopped"},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			dot := styledStatusDot(tt.status)
			if dot == "" {
				t.Error("styledStatusDot returned empty string")
			}
		})
	}
}

func TestAgentsPanel_LongTaskTruncates(t *testing.T) {
	p := NewAgentsPanel()
	p.SetSize(100, 20)
	p.SetAgents([]AgentSnapshot{
		{
			ID:     "a1",
			Plugin: "claude",
			Role:   types.RoleEngineer,
			Status: types.StatusBusy,
			Task:   "This is a very long task description that should be truncated in the table output",
		},
	})

	view := p.View()
	if !strings.Contains(view, "...") {
		t.Error("expected truncation with '...' for long task, but not found")
	}
}

func TestAgentsPanel_ZeroAgents(t *testing.T) {
	p := NewAgentsPanel()
	p.SetSize(120, 20)
	p.SetAgents(nil)

	view := p.View()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")

	// Header only.
	if len(lines) != 1 {
		t.Errorf("line count = %d, want 1 (header only)", len(lines))
	}
	if !strings.Contains(view, "AGENT") {
		t.Error("expected header with AGENT column")
	}
}

func TestPadOrTruncate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{"exact", "hello", 5, "hello"},
		{"pad", "hi", 5, "hi   "},
		{"truncate", "hello world", 8, "hello..."},
		{"zero_width", "hello", 0, ""},
		{"short_truncate", "abcd", 3, "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := padOrTruncate(tt.input, tt.width)
			if got != tt.want {
				t.Errorf("padOrTruncate(%q, %d) = %q, want %q", tt.input, tt.width, got, tt.want)
			}
		})
	}
}
