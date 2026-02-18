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

func TestAgentsPanel_CursorDownUp(t *testing.T) {
	p := NewAgentsPanel()
	p.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusIdle},
		{ID: "a2", Status: types.StatusIdle},
		{ID: "a3", Status: types.StatusIdle},
	})

	if p.cursor != 0 {
		t.Fatalf("initial cursor = %d, want 0", p.cursor)
	}

	p.HandleKey("j")
	if p.cursor != 1 {
		t.Errorf("after j: cursor = %d, want 1", p.cursor)
	}

	p.HandleKey("j")
	if p.cursor != 2 {
		t.Errorf("after second j: cursor = %d, want 2", p.cursor)
	}

	// Clamp at end.
	p.HandleKey("j")
	if p.cursor != 2 {
		t.Errorf("after third j: cursor = %d, want 2 (clamped)", p.cursor)
	}

	p.HandleKey("k")
	if p.cursor != 1 {
		t.Errorf("after k: cursor = %d, want 1", p.cursor)
	}

	p.HandleKey("k")
	p.HandleKey("k")
	if p.cursor != 0 {
		t.Errorf("after multiple k: cursor = %d, want 0 (clamped)", p.cursor)
	}
}

func TestAgentsPanel_CursorDownUp_ArrowKeys(t *testing.T) {
	p := NewAgentsPanel()
	p.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusIdle},
		{ID: "a2", Status: types.StatusIdle},
	})

	p.HandleKey("down")
	if p.cursor != 1 {
		t.Errorf("after down: cursor = %d, want 1", p.cursor)
	}

	p.HandleKey("up")
	if p.cursor != 0 {
		t.Errorf("after up: cursor = %d, want 0", p.cursor)
	}
}

func TestAgentsPanel_PauseConfirmation(t *testing.T) {
	p := NewAgentsPanel()
	p.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusBusy},
	})

	// Press 'p' to enter confirmation.
	handled, cmd := p.HandleKey("p")
	if !handled {
		t.Fatal("'p' should be handled")
	}
	if cmd != nil {
		t.Error("'p' should not return a command immediately")
	}
	if !p.confirming {
		t.Fatal("expected confirming state after 'p'")
	}
	if !strings.Contains(p.confirmPrompt, "Pause") {
		t.Errorf("confirmPrompt = %q, want to contain 'Pause'", p.confirmPrompt)
	}

	// Confirm with 'y'.
	handled, cmd = p.HandleKey("y")
	if !handled {
		t.Fatal("'y' should be handled during confirmation")
	}
	if cmd == nil {
		t.Fatal("'y' should return the pause command")
	}
	if cmd.Type != CmdPauseAgent {
		t.Errorf("Type = %v, want CmdPauseAgent", cmd.Type)
	}
	if cmd.AgentID != "a1" {
		t.Errorf("AgentID = %q, want %q", cmd.AgentID, "a1")
	}
	if p.confirming {
		t.Error("should exit confirming after 'y'")
	}
}

func TestAgentsPanel_PauseCancel_N(t *testing.T) {
	p := NewAgentsPanel()
	p.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusBusy},
	})

	p.HandleKey("p")
	if !p.confirming {
		t.Fatal("expected confirming state")
	}

	handled, cmd := p.HandleKey("n")
	if !handled {
		t.Fatal("'n' should be handled")
	}
	if cmd != nil {
		t.Error("'n' should not return a command")
	}
	if p.confirming {
		t.Error("should exit confirming after 'n'")
	}
}

func TestAgentsPanel_PauseCancel_Esc(t *testing.T) {
	p := NewAgentsPanel()
	p.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusBusy},
	})

	p.HandleKey("p")
	handled, cmd := p.HandleKey("esc")
	if !handled {
		t.Fatal("'esc' should be handled")
	}
	if cmd != nil {
		t.Error("'esc' should not return a command")
	}
	if p.confirming {
		t.Error("should exit confirming after 'esc'")
	}
}

func TestAgentsPanel_KillConfirmation(t *testing.T) {
	p := NewAgentsPanel()
	p.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusBusy},
	})

	p.HandleKey("x")
	if !p.confirming {
		t.Fatal("expected confirming state after 'x'")
	}
	if !strings.Contains(p.confirmPrompt, "Kill") {
		t.Errorf("confirmPrompt = %q, want to contain 'Kill'", p.confirmPrompt)
	}
	if !strings.Contains(p.confirmPrompt, "cannot be undone") {
		t.Errorf("confirmPrompt = %q, want to contain 'cannot be undone'", p.confirmPrompt)
	}

	handled, cmd := p.HandleKey("y")
	if !handled {
		t.Fatal("'y' should be handled")
	}
	if cmd == nil {
		t.Fatal("'y' should return the kill command")
	}
	if cmd.Type != CmdKillAgent {
		t.Errorf("Type = %v, want CmdKillAgent", cmd.Type)
	}
}

func TestAgentsPanel_ResumeImmediate(t *testing.T) {
	p := NewAgentsPanel()
	p.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusStopped},
	})

	handled, cmd := p.HandleKey("r")
	if !handled {
		t.Fatal("'r' should be handled")
	}
	if cmd == nil {
		t.Fatal("'r' on stopped agent should return command immediately")
	}
	if cmd.Type != CmdResumeAgent {
		t.Errorf("Type = %v, want CmdResumeAgent", cmd.Type)
	}
	if cmd.AgentID != "a1" {
		t.Errorf("AgentID = %q, want %q", cmd.AgentID, "a1")
	}
	if p.confirming {
		t.Error("resume should not enter confirmation")
	}
}

func TestAgentsPanel_ResumeNoOpIfNotStopped(t *testing.T) {
	p := NewAgentsPanel()
	p.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusBusy},
	})

	handled, cmd := p.HandleKey("r")
	if !handled {
		t.Fatal("'r' should be handled")
	}
	if cmd != nil {
		t.Error("'r' on non-stopped agent should be no-op")
	}
}

func TestAgentsPanel_PauseNoOpIfStopped(t *testing.T) {
	p := NewAgentsPanel()
	p.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusStopped},
	})

	handled, cmd := p.HandleKey("p")
	if !handled {
		t.Fatal("'p' should be handled")
	}
	if cmd != nil {
		t.Error("'p' on stopped agent should be no-op")
	}
	if p.confirming {
		t.Error("should not enter confirmation for stopped agent")
	}
}

func TestAgentsPanel_SelectedAgent_Nil(t *testing.T) {
	p := NewAgentsPanel()
	p.SetAgents(nil)

	if p.SelectedAgent() != nil {
		t.Error("SelectedAgent() should return nil on empty list")
	}
}

func TestAgentsPanel_SelectedAgent_Valid(t *testing.T) {
	p := NewAgentsPanel()
	p.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusIdle},
		{ID: "a2", Status: types.StatusBusy},
	})

	a := p.SelectedAgent()
	if a == nil {
		t.Fatal("SelectedAgent() should not be nil")
	}
	if a.ID != "a1" {
		t.Errorf("SelectedAgent().ID = %q, want %q", a.ID, "a1")
	}

	p.HandleKey("j")
	a = p.SelectedAgent()
	if a.ID != "a2" {
		t.Errorf("after j: SelectedAgent().ID = %q, want %q", a.ID, "a2")
	}
}

func TestAgentsPanel_SetAgents_ClampsCursor(t *testing.T) {
	p := NewAgentsPanel()
	p.SetAgents([]AgentSnapshot{
		{ID: "a1"},
		{ID: "a2"},
		{ID: "a3"},
	})

	p.HandleKey("j")
	p.HandleKey("j")
	if p.cursor != 2 {
		t.Fatalf("cursor = %d, want 2", p.cursor)
	}

	// Shrink the list.
	p.SetAgents([]AgentSnapshot{
		{ID: "a1"},
	})
	if p.cursor != 0 {
		t.Errorf("cursor = %d after shrink, want 0", p.cursor)
	}
}

func TestAgentsPanel_ConfirmPromptInView(t *testing.T) {
	p := NewAgentsPanel()
	p.SetSize(120, 20)
	p.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusBusy, Plugin: "claude", Role: types.RoleEngineer},
	})

	p.HandleKey("x")
	view := p.View()

	if !strings.Contains(view, "Kill") {
		t.Error("expected 'Kill' in view during confirmation")
	}
	if !strings.Contains(view, "a1") {
		t.Error("expected agent ID in confirmation prompt")
	}
}

func TestAgentsPanel_ConfirmSwallowsOtherKeys(t *testing.T) {
	p := NewAgentsPanel()
	p.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusBusy},
		{ID: "a2", Status: types.StatusIdle},
	})

	p.HandleKey("x")
	if !p.confirming {
		t.Fatal("expected confirming state")
	}

	// 'j' should be swallowed, cursor shouldn't move.
	handled, cmd := p.HandleKey("j")
	if !handled {
		t.Error("keys during confirmation should be handled (swallowed)")
	}
	if cmd != nil {
		t.Error("swallowed key should not return a command")
	}
	if p.cursor != 0 {
		t.Error("cursor should not move during confirmation")
	}
}

func TestAgentsPanel_SelectedRowHighlight(t *testing.T) {
	p := NewAgentsPanel()
	p.SetSize(120, 20)
	p.SetFocused(true)
	p.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusIdle, Plugin: "claude", Role: types.RoleEngineer},
		{ID: "a2", Status: types.StatusBusy, Plugin: "claude", Role: types.RoleEngineer},
	})

	view := p.View()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	// The first agent row (index 1, after header) should have the selection style.
	// We can't easily check the ANSI codes, but we can verify the output has different
	// styling for the two rows by checking they are not identical.
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}
	// The selected row (line 1) should differ from the non-selected row (line 2).
	if lines[1] == lines[2] {
		t.Error("selected row should be styled differently from non-selected row")
	}
}

func TestAgentsPanel_HandleKey_UnhandledKeys(t *testing.T) {
	p := NewAgentsPanel()
	p.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusIdle},
	})

	handled, cmd := p.HandleKey("z")
	if handled {
		t.Error("'z' should not be handled")
	}
	if cmd != nil {
		t.Error("unhandled key should not produce a command")
	}
}
