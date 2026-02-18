package planner

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/roygabriel/crux/internal/instruct/prefs"
)

// roundTripFunc adapts a function into an http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// sseResponse builds an *http.Response with SSE body content.
func sseResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body: io.NopCloser(strings.NewReader(body)),
	}
}

func testProjectContext() ProjectContext {
	return ProjectContext{
		Name:        "TestProject",
		Description: "A test project",
		Language:    "Go",
		Frameworks:  []string{"cobra", "bubbletea"},
		RepoRoot:    "/tmp/test",
		KeyConcerns: []string{"security", "performance"},
	}
}

func testPreferences() *prefs.Preferences {
	return &prefs.Preferences{
		Version:  prefs.CurrentVersion,
		Preset:   prefs.PresetStrict,
		Language: "go",
		Testing: prefs.TestingPrefs{
			Style:          prefs.TestingTDD,
			CoverageTarget: 80,
			MockApproach:   "interfaces",
			TableDriven:    true,
		},
		ErrorHandling: prefs.ErrorHandlingPrefs{
			Style:       prefs.ErrorWrapping,
			EarlyReturn: true,
			WrapContext:  true,
		},
	}
}

func TestNewAgent_ValidConstruction(t *testing.T) {
	agent, err := NewAgent("test-key", "", testProjectContext(), testPreferences(), nil, 0)
	if err != nil {
		t.Fatalf("NewAgent: unexpected error: %v", err)
	}

	if agent.model != anthropic.Model(DefaultModel) {
		t.Errorf("model = %q, want %q", agent.model, DefaultModel)
	}

	if agent.systemPrompt == "" {
		t.Error("system prompt should not be empty")
	}

	if agent.logger == nil {
		t.Error("logger should not be nil")
	}
}

func TestNewAgent_MaxTokens(t *testing.T) {
	agent, err := NewAgent("test-key", "", testProjectContext(), nil, nil, 32000)
	if err != nil {
		t.Fatalf("NewAgent: unexpected error: %v", err)
	}
	if agent.maxTokens != 32000 {
		t.Errorf("maxTokens = %d, want %d", agent.maxTokens, 32000)
	}
}

func TestNewAgent_MaxTokensZeroDefault(t *testing.T) {
	agent, err := NewAgent("test-key", "", testProjectContext(), nil, nil, 0)
	if err != nil {
		t.Fatalf("NewAgent: unexpected error: %v", err)
	}
	if agent.maxTokens != 0 {
		t.Errorf("maxTokens = %d, want 0 (will use defaultMaxTokens at stream time)", agent.maxTokens)
	}
}

func TestNewAgent_CustomModel(t *testing.T) {
	agent, err := NewAgent("test-key", "claude-opus-4-6", testProjectContext(), nil, nil, 0)
	if err != nil {
		t.Fatalf("NewAgent: unexpected error: %v", err)
	}

	if agent.model != "claude-opus-4-6" {
		t.Errorf("model = %q, want %q", agent.model, "claude-opus-4-6")
	}
}

func TestNewAgent_EmptyAPIKey(t *testing.T) {
	_, err := NewAgent("", "", testProjectContext(), nil, nil, 0)
	if err == nil {
		t.Fatal("expected error for empty API key, got nil")
	}

	if !strings.Contains(err.Error(), "API key is required") {
		t.Errorf("error = %q, want message containing 'API key is required'", err)
	}
}

func TestNewAgent_NilLogger(t *testing.T) {
	agent, err := NewAgent("test-key", "", testProjectContext(), nil, nil, 0)
	if err != nil {
		t.Fatalf("NewAgent: unexpected error: %v", err)
	}
	if agent.logger == nil {
		t.Error("logger should fall back to slog.Default(), not nil")
	}
}

func TestBuildSystemPrompt_ContainsContext(t *testing.T) {
	ctx := testProjectContext()
	p := testPreferences()
	prompt := BuildSystemPrompt(ctx, p)

	checks := []struct {
		name     string
		contains string
	}{
		{"project name", "TestProject"},
		{"description", "A test project"},
		{"language", "Go"},
		{"frameworks", "cobra"},
		{"repo root", "/tmp/test"},
		{"key concerns", "security"},
		{"system prompt header", "Crux Planning Agent"},
		{"preferences present", "Testing"},
	}

	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(prompt, tc.contains) {
				t.Errorf("prompt should contain %q", tc.contains)
			}
		})
	}
}

func TestBuildSystemPrompt_NilPrefs(t *testing.T) {
	prompt := BuildSystemPrompt(testProjectContext(), nil)

	if !strings.Contains(prompt, "No engineering preferences configured") {
		t.Error("nil prefs should produce 'No engineering preferences configured' text")
	}

	if !strings.Contains(prompt, "TestProject") {
		t.Error("project context should still be present with nil prefs")
	}
}

func TestAgent_History_Empty(t *testing.T) {
	agent, err := NewAgent("test-key", "", testProjectContext(), nil, nil, 0)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	history := agent.History()
	if len(history) != 0 {
		t.Errorf("initial history length = %d, want 0", len(history))
	}
}

func TestAgent_Reset(t *testing.T) {
	agent, err := NewAgent("test-key", "", testProjectContext(), nil, nil, 0)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	// Manually add a message to history.
	agent.history = append(agent.history, anthropic.NewUserMessage(
		anthropic.NewTextBlock("hello"),
	))
	if len(agent.History()) != 1 {
		t.Fatal("expected 1 message in history")
	}

	agent.Reset()
	if len(agent.History()) != 0 {
		t.Errorf("after Reset, history length = %d, want 0", len(agent.History()))
	}
}

func TestAgent_SetTools(t *testing.T) {
	agent, err := NewAgent("test-key", "", testProjectContext(), nil, nil, 0)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	tools := []anthropic.ToolUnionParam{
		{
			OfTool: &anthropic.ToolParam{
				Name:        "read_file",
				Description: anthropic.String("Read a file"),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: map[string]any{
						"path": map[string]string{
							"type":        "string",
							"description": "File path",
						},
					},
					Required: []string{"path"},
				},
			},
		},
	}

	agent.SetTools(tools)

	if len(agent.tools) != 1 {
		t.Errorf("tools length = %d, want 1", len(agent.tools))
	}
	if agent.tools[0].OfTool.Name != "read_file" {
		t.Errorf("tool name = %q, want %q", agent.tools[0].OfTool.Name, "read_file")
	}
}

func TestAgent_SystemPrompt(t *testing.T) {
	agent, err := NewAgent("test-key", "", testProjectContext(), testPreferences(), nil, 0)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}

	if agent.SystemPrompt() == "" {
		t.Error("SystemPrompt() should not be empty")
	}
	if !strings.Contains(agent.SystemPrompt(), "Crux Planning Agent") {
		t.Error("SystemPrompt() should contain 'Crux Planning Agent'")
	}
}

func TestStreamChunk_Types(t *testing.T) {
	tests := []struct {
		name  string
		chunk StreamChunk
	}{
		{
			name:  "text chunk",
			chunk: StreamChunk{Text: "Hello"},
		},
		{
			name: "tool use chunk",
			chunk: StreamChunk{
				ToolUse: &ToolUseChunk{
					ID:    "tool_123",
					Name:  "read_file",
					Input: json.RawMessage(`{"path":"main.go"}`),
				},
			},
		},
		{
			name:  "done chunk",
			chunk: StreamChunk{Done: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			switch {
			case tc.chunk.Text != "":
				if tc.chunk.Text != "Hello" {
					t.Errorf("text = %q, want %q", tc.chunk.Text, "Hello")
				}
			case tc.chunk.ToolUse != nil:
				if tc.chunk.ToolUse.ID != "tool_123" {
					t.Errorf("tool ID = %q, want %q", tc.chunk.ToolUse.ID, "tool_123")
				}
				if tc.chunk.ToolUse.Name != "read_file" {
					t.Errorf("tool name = %q, want %q", tc.chunk.ToolUse.Name, "read_file")
				}
			case tc.chunk.Done:
				// Pass — done chunk is valid.
			default:
				t.Error("chunk should match at least one condition")
			}
		})
	}
}

// textSSE builds an SSE payload for a simple text-only assistant response.
func textSSE(text string) string {
	// The Anthropic SSE format for a simple text response.
	return `event: message_start
data: {"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-5-20250929","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"` + text + `"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":5}}

event: message_stop
data: {"type":"message_stop"}

`
}

// toolUseSSE builds an SSE payload for a tool-use response.
func toolUseSSE(toolID, toolName, inputJSON string) string {
	return `event: message_start
data: {"type":"message_start","message":{"id":"msg_tool","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-5-20250929","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"` + toolID + `","name":"` + toolName + `","input":{}}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"` + strings.ReplaceAll(inputJSON, `"`, `\"`) + `"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"output_tokens":20}}

event: message_stop
data: {"type":"message_stop"}

`
}

func newMockAgent(t *testing.T, transport http.RoundTripper) *Agent {
	t.Helper()
	agent, err := NewAgent(
		"test-key", "",
		testProjectContext(), nil, nil, 0,
		option.WithHTTPClient(&http.Client{Transport: transport}),
		option.WithMaxRetries(0),
	)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	return agent
}

func TestAgent_SendMessage_TextStream(t *testing.T) {
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return sseResponse(textSSE("Hello, world!")), nil
	})

	agent := newMockAgent(t, transport)
	ch, err := agent.SendMessage(context.Background(), "Hi")
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	var texts []string
	var gotDone bool
	for chunk := range ch {
		if chunk.Err != nil {
			t.Fatalf("stream error: %v", chunk.Err)
		}
		if chunk.Text != "" {
			texts = append(texts, chunk.Text)
		}
		if chunk.Done {
			gotDone = true
		}
	}

	if !gotDone {
		t.Error("expected Done chunk")
	}
	combined := strings.Join(texts, "")
	if combined != "Hello, world!" {
		t.Errorf("streamed text = %q, want %q", combined, "Hello, world!")
	}

	// History should have user + assistant messages.
	history := agent.History()
	if len(history) != 2 {
		t.Fatalf("history length = %d, want 2", len(history))
	}
	if history[0].Role != "user" || history[0].Content != "Hi" {
		t.Errorf("history[0] = %+v, want user/Hi", history[0])
	}
	if history[1].Role != "assistant" || history[1].Content != "Hello, world!" {
		t.Errorf("history[1] = %+v, want assistant/'Hello, world!'", history[1])
	}
}

func TestAgent_SendMessage_ToolUse(t *testing.T) {
	callCount := 0
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return sseResponse(toolUseSSE("tool_abc", "read_file", `{"path":"main.go"}`)), nil
		}
		return sseResponse(textSSE("File content: package main")), nil
	})

	agent := newMockAgent(t, transport)
	agent.SetTools([]anthropic.ToolUnionParam{
		{
			OfTool: &anthropic.ToolParam{
				Name:        "read_file",
				Description: anthropic.String("Read a file"),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: map[string]any{
						"path": map[string]string{"type": "string"},
					},
					Required: []string{"path"},
				},
			},
		},
	})

	ch, err := agent.SendMessage(context.Background(), "Read main.go")
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	var toolChunk *ToolUseChunk
	for chunk := range ch {
		if chunk.Err != nil {
			t.Fatalf("stream error: %v", chunk.Err)
		}
		if chunk.ToolUse != nil {
			toolChunk = chunk.ToolUse
		}
	}

	if toolChunk == nil {
		t.Fatal("expected a tool use chunk")
	}
	if toolChunk.ID != "tool_abc" {
		t.Errorf("tool ID = %q, want %q", toolChunk.ID, "tool_abc")
	}
	if toolChunk.Name != "read_file" {
		t.Errorf("tool name = %q, want %q", toolChunk.Name, "read_file")
	}

	// Send tool result.
	ch2, err := agent.HandleToolResult(context.Background(), "tool_abc", "package main\n", false)
	if err != nil {
		t.Fatalf("HandleToolResult: %v", err)
	}

	var gotText bool
	for chunk := range ch2 {
		if chunk.Err != nil {
			t.Fatalf("stream error: %v", chunk.Err)
		}
		if chunk.Text != "" {
			gotText = true
		}
	}
	if !gotText {
		t.Error("expected text in tool result response")
	}
}

func TestAgent_History_GrowsAfterInteraction(t *testing.T) {
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return sseResponse(textSSE("response")), nil
	})

	agent := newMockAgent(t, transport)
	if len(agent.History()) != 0 {
		t.Fatal("expected empty history initially")
	}

	ch, err := agent.SendMessage(context.Background(), "hello")
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	// Drain the channel.
	for range ch {
	}

	history := agent.History()
	if len(history) != 2 {
		t.Errorf("history length = %d, want 2", len(history))
	}
}
