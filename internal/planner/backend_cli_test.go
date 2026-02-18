package planner

import (
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestNewCLIBackend_UnknownAgent(t *testing.T) {
	_, err := NewCLIBackend("unknown", "system prompt", t.TempDir(), nil)
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
	if !strings.Contains(err.Error(), "unknown CLI agent") {
		t.Errorf("error = %q, want message about unknown agent", err)
	}
}

func TestNewCLIBackend_ValidAgents(t *testing.T) {
	// Skip if binaries aren't available — just test the config lookup.
	for _, name := range []string{"claude", "codex", "gemini"} {
		t.Run(name, func(t *testing.T) {
			_, err := NewCLIBackend(name, "test prompt", t.TempDir(), nil)
			if err != nil {
				// Expected if binary not in PATH.
				if strings.Contains(err.Error(), "not found in PATH") {
					t.Skipf("%s not in PATH, skipping", name)
				}
				t.Fatalf("NewCLIBackend(%q): %v", name, err)
			}
		})
	}
}

func TestCLIBackend_BuildPrompt(t *testing.T) {
	b := &CLIBackend{
		systemPrompt: "You are a helpful assistant.",
		history: []Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
			{Role: "user", Content: "How are you?"},
		},
	}

	prompt := b.buildPrompt()

	checks := []struct {
		name     string
		contains string
	}{
		{"system prompt", "You are a helpful assistant."},
		{"system tags", "<system>"},
		{"user message 1", "Hello"},
		{"assistant response", "Hi there!"},
		{"user message 2", "How are you?"},
		{"user tags", "<user>"},
		{"assistant tags", "<assistant>"},
	}

	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(prompt, tc.contains) {
				t.Errorf("prompt should contain %q, got:\n%s", tc.contains, prompt)
			}
		})
	}
}

func TestCLIBackend_BuildPrompt_Empty(t *testing.T) {
	b := &CLIBackend{
		systemPrompt: "test",
		history:      []Message{},
	}

	prompt := b.buildPrompt()
	if !strings.Contains(prompt, "<system>") {
		t.Error("empty history prompt should still contain system tags")
	}
	if strings.Contains(prompt, "<user>") {
		t.Error("empty history prompt should not contain user tags")
	}
}

func TestCLIBackend_History(t *testing.T) {
	b := &CLIBackend{
		history: []Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi"},
		},
	}

	h := b.History()
	if len(h) != 2 {
		t.Fatalf("History length = %d, want 2", len(h))
	}

	// Ensure it's a copy, not a reference.
	h[0].Content = "modified"
	if b.history[0].Content == "modified" {
		t.Error("History() should return a copy")
	}
}

func TestCLIBackend_Reset(t *testing.T) {
	b := &CLIBackend{
		history: []Message{
			{Role: "user", Content: "hello"},
		},
	}

	b.Reset()
	if len(b.history) != 0 {
		t.Errorf("after Reset, history length = %d, want 0", len(b.history))
	}
}

func TestCLIBackend_SystemPrompt(t *testing.T) {
	b := &CLIBackend{
		systemPrompt: "test system prompt",
	}
	if b.SystemPrompt() != "test system prompt" {
		t.Errorf("SystemPrompt() = %q, want 'test system prompt'", b.SystemPrompt())
	}
}

func TestCLIBackend_HandleToolResult_History(t *testing.T) {
	b := &CLIBackend{
		binary:       "echo",
		args:         []string{},
		systemPrompt: "test",
		history:      []Message{},
		projectRoot:  t.TempDir(),
		logger:       slog.Default(),
	}

	// HandleToolResult should append to history even if the agent fails.
	// We use "echo" as a no-op binary; it won't produce valid stream-json
	// but the history append of the user message happens synchronously.
	_, _ = b.HandleToolResult(context.Background(), "tool_1", "file contents", false)

	if len(b.history) < 1 {
		t.Fatal("expected at least 1 message in history")
	}

	last := b.history[len(b.history)-1]
	if last.Role != "user" {
		t.Errorf("tool result role = %q, want 'user'", last.Role)
	}
	if !strings.Contains(last.Content, "tool_1") {
		t.Errorf("tool result should contain tool ID, got: %q", last.Content)
	}
	if !strings.Contains(last.Content, "Tool result") {
		t.Errorf("tool result should contain 'Tool result', got: %q", last.Content)
	}
}

func TestCLIBackend_HandleToolResult_Error(t *testing.T) {
	b := &CLIBackend{
		binary:       "echo",
		args:         []string{},
		systemPrompt: "test",
		history:      []Message{},
		projectRoot:  t.TempDir(),
		logger:       slog.Default(),
	}

	_, _ = b.HandleToolResult(context.Background(), "tool_2", "something went wrong", true)

	if len(b.history) < 1 {
		t.Fatal("expected at least 1 message in history")
	}
	last := b.history[len(b.history)-1]
	if !strings.Contains(last.Content, "Tool error") {
		t.Errorf("error tool result should contain 'Tool error', got: %q", last.Content)
	}
}

func TestCLIBackend_SendMessage_History(t *testing.T) {
	b := &CLIBackend{
		binary:       "echo",
		args:         []string{},
		systemPrompt: "test",
		history:      []Message{},
		projectRoot:  t.TempDir(),
		logger:       slog.Default(),
	}

	_, _ = b.SendMessage(context.Background(), "hello world")

	if len(b.history) < 1 {
		t.Fatal("expected at least 1 message in history")
	}
	if b.history[0].Role != "user" {
		t.Errorf("first message role = %q, want 'user'", b.history[0].Role)
	}
	if b.history[0].Content != "hello world" {
		t.Errorf("first message content = %q, want 'hello world'", b.history[0].Content)
	}
}

func TestNewAgentWithBackend(t *testing.T) {
	b := &CLIBackend{
		systemPrompt: "test prompt",
		history:      []Message{},
	}

	agent := NewAgentWithBackend(b)

	if agent.backend != b {
		t.Error("NewAgentWithBackend should set the backend")
	}
	if agent.sdkBackend != nil {
		t.Error("NewAgentWithBackend should not set sdkBackend")
	}
	if agent.SystemPrompt() != "test prompt" {
		t.Errorf("SystemPrompt() = %q, want 'test prompt'", agent.SystemPrompt())
	}
}
