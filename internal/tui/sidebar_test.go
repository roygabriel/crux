package tui

import (
	"strings"
	"testing"

	"github.com/roygabriel/crux/pkg/types"
)

func TestSidebarPanel_RendersAgentList(t *testing.T) {
	p := NewSidebarPanel()
	p.SetSize(30, 20)
	p.SetAgents([]AgentSnapshot{
		{ID: "eng-1", Status: types.StatusBusy},
		{ID: "eng-2", Status: types.StatusIdle},
	})

	view := p.View()
	if !strings.Contains(view, "eng-1") {
		t.Error("expected eng-1 in sidebar view")
	}
	if !strings.Contains(view, "eng-2") {
		t.Error("expected eng-2 in sidebar view")
	}
}

func TestSidebarPanel_EmptyAgents(t *testing.T) {
	p := NewSidebarPanel()
	p.SetSize(30, 20)
	p.SetAgents(nil)

	view := p.View()
	if !strings.Contains(view, "(no agents)") {
		t.Error("expected '(no agents)' in empty sidebar")
	}
}

func TestSidebarPanel_CursorMovement(t *testing.T) {
	tests := []struct {
		name       string
		keys       []string
		wantCursor int
	}{
		{"initial", nil, 0},
		{"down once", []string{"j"}, 1},
		{"down twice", []string{"j", "j"}, 2},
		{"down clamp", []string{"j", "j", "j", "j"}, 2},
		{"down then up", []string{"j", "j", "k"}, 1},
		{"up clamp", []string{"k", "k"}, 0},
		{"arrow down", []string{"down"}, 1},
		{"arrow up after down", []string{"down", "up"}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewSidebarPanel()
			p.SetAgents([]AgentSnapshot{
				{ID: "a1", Status: types.StatusIdle},
				{ID: "a2", Status: types.StatusIdle},
				{ID: "a3", Status: types.StatusIdle},
			})

			for _, key := range tt.keys {
				p.HandleKey(key)
			}
			if p.cursor != tt.wantCursor {
				t.Errorf("cursor = %d, want %d", p.cursor, tt.wantCursor)
			}
		})
	}
}

func TestSidebarPanel_SelectedAgent(t *testing.T) {
	tests := []struct {
		name    string
		agents  []AgentSnapshot
		keys    []string
		wantID  types.AgentID
		wantNil bool
	}{
		{"empty list", nil, nil, "", true},
		{"first agent", []AgentSnapshot{{ID: "a1"}}, nil, "a1", false},
		{"after j", []AgentSnapshot{{ID: "a1"}, {ID: "a2"}}, []string{"j"}, "a2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewSidebarPanel()
			p.SetAgents(tt.agents)
			for _, key := range tt.keys {
				p.HandleKey(key)
			}
			agent := p.SelectedAgent()
			if tt.wantNil {
				if agent != nil {
					t.Error("expected nil")
				}
				return
			}
			if agent == nil {
				t.Fatal("expected non-nil")
			}
			if agent.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", agent.ID, tt.wantID)
			}
		})
	}
}

func TestSidebarPanel_SetAgents_ClampsCursor(t *testing.T) {
	p := NewSidebarPanel()
	p.SetAgents([]AgentSnapshot{{ID: "a1"}, {ID: "a2"}, {ID: "a3"}})
	p.HandleKey("j")
	p.HandleKey("j")

	p.SetAgents([]AgentSnapshot{{ID: "a1"}})
	if p.cursor != 0 {
		t.Errorf("cursor = %d after shrink, want 0", p.cursor)
	}
}

func TestSidebarPanel_PauseConfirmation(t *testing.T) {
	p := NewSidebarPanel()
	p.SetAgents([]AgentSnapshot{{ID: "a1", Status: types.StatusBusy}})

	handled, cmd := p.HandleKey("p")
	if !handled {
		t.Fatal("p should be handled")
	}
	if cmd != nil {
		t.Error("p should not return a command immediately")
	}
	if !p.confirming {
		t.Fatal("expected confirming state")
	}
	if !strings.Contains(p.confirmPrompt, "Pause") {
		t.Errorf("prompt = %q, want to contain 'Pause'", p.confirmPrompt)
	}

	handled, cmd = p.HandleKey("y")
	if !handled || cmd == nil {
		t.Fatal("y should confirm and return command")
	}
	if cmd.Type != CmdPauseAgent || cmd.AgentID != "a1" {
		t.Errorf("cmd = %+v, want PauseAgent for a1", cmd)
	}
	if p.confirming {
		t.Error("should exit confirming after y")
	}
}

func TestSidebarPanel_PauseCancelN(t *testing.T) {
	p := NewSidebarPanel()
	p.SetAgents([]AgentSnapshot{{ID: "a1", Status: types.StatusBusy}})
	p.HandleKey("p")

	handled, cmd := p.HandleKey("n")
	if !handled {
		t.Fatal("n should be handled")
	}
	if cmd != nil {
		t.Error("n should not return a command")
	}
	if p.confirming {
		t.Error("should exit confirming after n")
	}
}

func TestSidebarPanel_PauseCancelEsc(t *testing.T) {
	p := NewSidebarPanel()
	p.SetAgents([]AgentSnapshot{{ID: "a1", Status: types.StatusBusy}})
	p.HandleKey("p")

	handled, cmd := p.HandleKey("esc")
	if !handled {
		t.Fatal("esc should be handled")
	}
	if cmd != nil {
		t.Error("esc should not return a command")
	}
	if p.confirming {
		t.Error("should exit confirming after esc")
	}
}

func TestSidebarPanel_KillConfirmation(t *testing.T) {
	p := NewSidebarPanel()
	p.SetAgents([]AgentSnapshot{{ID: "a1", Status: types.StatusBusy}})
	p.HandleKey("x")

	if !p.confirming {
		t.Fatal("expected confirming state after x")
	}
	if !strings.Contains(p.confirmPrompt, "Kill") {
		t.Errorf("prompt = %q, want to contain 'Kill'", p.confirmPrompt)
	}
	if !strings.Contains(p.confirmPrompt, "cannot be undone") {
		t.Errorf("prompt = %q, want to contain 'cannot be undone'", p.confirmPrompt)
	}

	handled, cmd := p.HandleKey("y")
	if !handled || cmd == nil {
		t.Fatal("y should return kill command")
	}
	if cmd.Type != CmdKillAgent {
		t.Errorf("Type = %v, want CmdKillAgent", cmd.Type)
	}
}

func TestSidebarPanel_ResumeImmediate(t *testing.T) {
	p := NewSidebarPanel()
	p.SetAgents([]AgentSnapshot{{ID: "a1", Status: types.StatusStopped}})

	handled, cmd := p.HandleKey("r")
	if !handled {
		t.Fatal("r should be handled")
	}
	if cmd == nil {
		t.Fatal("r on stopped agent should return command immediately")
	}
	if cmd.Type != CmdResumeAgent || cmd.AgentID != "a1" {
		t.Errorf("cmd = %+v, want ResumeAgent for a1", cmd)
	}
	if p.confirming {
		t.Error("resume should not enter confirmation")
	}
}

func TestSidebarPanel_ResumeNoOpIfNotStopped(t *testing.T) {
	p := NewSidebarPanel()
	p.SetAgents([]AgentSnapshot{{ID: "a1", Status: types.StatusBusy}})

	handled, cmd := p.HandleKey("r")
	if !handled {
		t.Fatal("r should be handled")
	}
	if cmd != nil {
		t.Error("r on non-stopped agent should be no-op")
	}
}

func TestSidebarPanel_PauseNoOpIfStopped(t *testing.T) {
	p := NewSidebarPanel()
	p.SetAgents([]AgentSnapshot{{ID: "a1", Status: types.StatusStopped}})

	handled, cmd := p.HandleKey("p")
	if !handled {
		t.Fatal("p should be handled")
	}
	if cmd != nil {
		t.Error("p on stopped agent should be no-op")
	}
	if p.confirming {
		t.Error("should not enter confirmation for stopped agent")
	}
}

func TestSidebarPanel_ConfirmSwallowsOtherKeys(t *testing.T) {
	p := NewSidebarPanel()
	p.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusBusy},
		{ID: "a2", Status: types.StatusIdle},
	})
	p.HandleKey("x")

	handled, cmd := p.HandleKey("j")
	if !handled {
		t.Error("keys during confirmation should be swallowed")
	}
	if cmd != nil {
		t.Error("swallowed key should not return a command")
	}
	if p.cursor != 0 {
		t.Error("cursor should not move during confirmation")
	}
}

func TestSidebarPanel_ConfirmPromptInView(t *testing.T) {
	p := NewSidebarPanel()
	p.SetSize(30, 20)
	p.SetAgents([]AgentSnapshot{{ID: "a1", Status: types.StatusBusy}})
	p.HandleKey("x")

	view := p.View()
	if !strings.Contains(view, "Kill") {
		t.Error("expected 'Kill' in view during confirmation")
	}
}

func TestSidebarPanel_SelectedRowHighlight(t *testing.T) {
	p := NewSidebarPanel()
	p.SetSize(30, 20)
	p.SetFocused(true)
	p.SetAgents([]AgentSnapshot{
		{ID: "a1", Status: types.StatusIdle},
		{ID: "a2", Status: types.StatusBusy},
	})

	view := p.View()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	// Header + 2 rows = 3 lines minimum.
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}
	// Selected row (line 1) should differ from non-selected (line 2).
	if lines[1] == lines[2] {
		t.Error("selected row should be styled differently from non-selected row")
	}
}

func TestSidebarPanel_UnhandledKeys(t *testing.T) {
	p := NewSidebarPanel()
	p.SetAgents([]AgentSnapshot{{ID: "a1", Status: types.StatusIdle}})

	handled, cmd := p.HandleKey("z")
	if handled {
		t.Error("z should not be handled")
	}
	if cmd != nil {
		t.Error("unhandled key should not produce a command")
	}
}

func TestSidebarPanel_HeaderShown(t *testing.T) {
	p := NewSidebarPanel()
	p.SetSize(30, 20)
	p.SetAgents(nil)

	view := p.View()
	if !strings.Contains(view, "AGENTS") {
		t.Error("expected AGENTS header in sidebar view")
	}
}
