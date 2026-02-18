package tui

import (
	"log/slog"
	"testing"

	"github.com/roygabriel/crux/pkg/types"
)

func TestCommandBus_SendReceive(t *testing.T) {
	bus := NewCommandBus(4, slog.Default())
	cmd := Command{Type: CmdPauseAgent, AgentID: "a1"}
	bus.Send(cmd)

	select {
	case got := <-bus.Receive():
		if got.Type != CmdPauseAgent {
			t.Errorf("Type = %v, want CmdPauseAgent", got.Type)
		}
		if got.AgentID != "a1" {
			t.Errorf("AgentID = %q, want %q", got.AgentID, "a1")
		}
	default:
		t.Fatal("expected command on channel, got nothing")
	}
}

func TestCommandBus_DropOnFull(t *testing.T) {
	bus := NewCommandBus(1, slog.Default())

	// Fill the buffer.
	bus.Send(Command{Type: CmdPauseAgent, AgentID: "a1"})

	// This should be dropped without blocking.
	bus.Send(Command{Type: CmdKillAgent, AgentID: "a2"})

	// Only the first command should be present.
	got := <-bus.Receive()
	if got.AgentID != "a1" {
		t.Errorf("AgentID = %q, want %q (first command)", got.AgentID, "a1")
	}

	// Channel should be empty now.
	select {
	case extra := <-bus.Receive():
		t.Errorf("unexpected extra command: %+v", extra)
	default:
	}
}

func TestCommandBus_ClampToOne(t *testing.T) {
	bus := NewCommandBus(0, slog.Default())

	// Should still accept one command.
	bus.Send(Command{Type: CmdResumeAgent, AgentID: "a1"})

	select {
	case got := <-bus.Receive():
		if got.Type != CmdResumeAgent {
			t.Errorf("Type = %v, want CmdResumeAgent", got.Type)
		}
	default:
		t.Fatal("expected command after clamp-to-1")
	}
}

func TestCommandBus_NilLogger(t *testing.T) {
	bus := NewCommandBus(1, nil)
	bus.Send(Command{Type: CmdPauseAgent})

	select {
	case <-bus.Receive():
	default:
		t.Fatal("expected command with nil logger")
	}
}

func TestCommandType_String(t *testing.T) {
	tests := []struct {
		ct   CommandType
		want string
	}{
		{CmdPauseAgent, "pause"},
		{CmdResumeAgent, "resume"},
		{CmdKillAgent, "kill"},
		{CmdForceAdvance, "force-advance"},
		{CmdSendMessage, "send-message"},
		{CmdShutdown, "shutdown"},
		{CommandType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.ct.String()
			if got != tt.want {
				t.Errorf("CommandType(%d).String() = %q, want %q", tt.ct, got, tt.want)
			}
		})
	}
}

func TestCommand_Fields(t *testing.T) {
	cmd := Command{
		Type:    CmdSendMessage,
		AgentID: types.AgentID("agent-1"),
		PhaseID: types.PhaseID("phase-2A"),
		Text:    "hello",
	}

	if cmd.Type != CmdSendMessage {
		t.Errorf("Type = %v, want CmdSendMessage", cmd.Type)
	}
	if cmd.AgentID != "agent-1" {
		t.Errorf("AgentID = %q, want %q", cmd.AgentID, "agent-1")
	}
	if cmd.PhaseID != "phase-2A" {
		t.Errorf("PhaseID = %q, want %q", cmd.PhaseID, "phase-2A")
	}
	if cmd.Text != "hello" {
		t.Errorf("Text = %q, want %q", cmd.Text, "hello")
	}
}
