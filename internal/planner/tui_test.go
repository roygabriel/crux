package planner

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func newTestTUIModel(t *testing.T) TUIModel {
	t.Helper()
	agent, err := NewAgent("test-key", "", testProjectContext(), nil, nil, 0)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	return NewTUIModel(agent, t.TempDir())
}

func initModel(m TUIModel) TUIModel {
	// Simulate window size to make the model ready.
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	return updated.(TUIModel)
}

func TestNewTUIModel_Defaults(t *testing.T) {
	m := newTestTUIModel(t)

	if m.agent == nil {
		t.Error("agent should not be nil")
	}
	if m.streaming {
		t.Error("should not be streaming initially")
	}
	if len(m.messages) != 0 {
		t.Errorf("messages should be empty, got %d", len(m.messages))
	}
	if m.renderer == nil {
		t.Error("glamour renderer should not be nil")
	}
}

func TestTUIModel_Init(t *testing.T) {
	m := newTestTUIModel(t)
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init should return a command (textarea blink)")
	}
}

func TestTUIModel_WindowSize(t *testing.T) {
	m := newTestTUIModel(t)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 50})
	model := updated.(TUIModel)

	if !model.ready {
		t.Error("model should be ready after WindowSizeMsg")
	}
	if model.width != 120 {
		t.Errorf("width = %d, want 120", model.width)
	}
	if model.height != 50 {
		t.Errorf("height = %d, want 50", model.height)
	}
}

func TestTUIModel_ViewBeforeReady(t *testing.T) {
	m := newTestTUIModel(t)
	view := m.View()
	if !strings.Contains(view, "Initializing") {
		t.Errorf("view before ready should show initializing, got: %q", view)
	}
}

func TestTUIModel_ViewAfterReady(t *testing.T) {
	m := initModel(newTestTUIModel(t))
	view := m.View()

	if strings.Contains(view, "Initializing") {
		t.Error("view after ready should not show initializing")
	}
	if !strings.Contains(view, "Planning") {
		t.Error("status bar should contain 'Planning'")
	}
}

func TestTUIModel_StreamDone(t *testing.T) {
	m := initModel(newTestTUIModel(t))

	// Simulate a user message and streaming state.
	m.messages = append(m.messages, chatMessage{role: "user", content: "hello"})
	m.streaming = true
	m.streamBuf.WriteString("Hello, I'm the planner.")

	updated, _ := m.Update(StreamDoneMsg{})
	model := updated.(TUIModel)

	if model.streaming {
		t.Error("streaming should be false after StreamDoneMsg")
	}
	if len(model.messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(model.messages))
	}
	if model.messages[1].role != "assistant" {
		t.Errorf("second message role = %q, want 'assistant'", model.messages[1].role)
	}
	if model.messages[1].content != "Hello, I'm the planner." {
		t.Errorf("assistant content = %q", model.messages[1].content)
	}
	if model.messages[1].rendered == "" {
		t.Error("assistant message should have glamour-rendered content")
	}
}

func TestTUIModel_StreamDone_EmptyBuffer(t *testing.T) {
	m := initModel(newTestTUIModel(t))

	m.messages = append(m.messages, chatMessage{role: "user", content: "hello"})
	m.streaming = true
	// streamBuf is empty — no text was received before done.

	updated, _ := m.Update(StreamDoneMsg{})
	model := updated.(TUIModel)

	if model.streaming {
		t.Error("streaming should be false after StreamDoneMsg")
	}
	if len(model.messages) != 1 {
		t.Errorf("expected 1 message (user only), got %d", len(model.messages))
	}
}

func TestTUIModel_StreamBatch(t *testing.T) {
	m := initModel(newTestTUIModel(t))
	m.streaming = true

	ch := make(chan StreamChunk, 2)
	ch <- StreamChunk{Text: " world"}
	ch <- StreamChunk{Done: true}

	updated, cmd := m.Update(streamBatchMsg{text: "hello", ch: ch})
	model := updated.(TUIModel)

	if model.streamBuf.String() != "hello" {
		t.Errorf("stream buffer = %q, want 'hello'", model.streamBuf.String())
	}
	if cmd == nil {
		t.Error("should return a cmd to continue reading stream")
	}
}

func TestTUIModel_StreamError(t *testing.T) {
	m := initModel(newTestTUIModel(t))
	m.streaming = true

	testErr := errors.New("API rate limited")
	updated, _ := m.Update(StreamErrMsg{Err: testErr})
	model := updated.(TUIModel)

	if model.streaming {
		t.Error("streaming should be false after StreamErrMsg")
	}
	if model.err == nil {
		t.Error("err should be set")
	}

	// Error should be appended as an inline chat message.
	found := false
	for _, msg := range model.messages {
		if msg.role == "error" {
			found = true
			if !strings.Contains(msg.content, "rate limited") {
				t.Errorf("error message should contain original error, got: %q", msg.content)
			}
			break
		}
	}
	if !found {
		t.Error("expected an error chat message to be appended")
	}

	view := model.View()
	if !strings.Contains(view, "rate limited") {
		t.Errorf("view should show error, got: %s", view)
	}
}

func TestTUIModel_ToolUseMsg(t *testing.T) {
	m := initModel(newTestTUIModel(t))

	chunk := ToolUseChunk{
		ID:    "tool_123",
		Name:  "read_file",
		Input: json.RawMessage(`{"path":"main.go"}`),
	}

	updated, cmd := m.Update(ToolUseMsg{Chunk: chunk})
	model := updated.(TUIModel)

	if len(model.messages) != 1 {
		t.Fatalf("expected 1 tool message, got %d", len(model.messages))
	}
	if model.messages[0].role != "tool" {
		t.Errorf("message role = %q, want 'tool'", model.messages[0].role)
	}
	if !strings.Contains(model.messages[0].content, "read_file") {
		t.Errorf("tool message should mention tool name, got: %q", model.messages[0].content)
	}
	if cmd == nil {
		t.Error("should return a cmd to execute the tool")
	}
}

func TestTUIModel_EscQuits(t *testing.T) {
	m := initModel(newTestTUIModel(t))

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	_ = updated

	// tea.Quit is a function, check it's not nil.
	if cmd == nil {
		t.Error("Esc should return a quit command")
	}
}

func TestTUIModel_CtrlCQuits(t *testing.T) {
	m := initModel(newTestTUIModel(t))

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("Ctrl+C should return a quit command")
	}
}

func TestTUIModel_CtrlR_Reset(t *testing.T) {
	m := initModel(newTestTUIModel(t))

	// Add some messages.
	m.messages = append(m.messages, chatMessage{role: "user", content: "hello"})
	m.messages = append(m.messages, chatMessage{role: "assistant", content: "hi"})
	m.phaseCount = 3

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	model := updated.(TUIModel)

	if len(model.messages) != 0 {
		t.Errorf("messages should be cleared, got %d", len(model.messages))
	}
	if model.phaseCount != 0 {
		t.Errorf("phaseCount should be 0, got %d", model.phaseCount)
	}
	if model.streaming {
		t.Error("streaming should be false after reset")
	}
}

func TestTUIModel_EnterNoInput(t *testing.T) {
	m := initModel(newTestTUIModel(t))

	// Enter with empty input should do nothing.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(TUIModel)

	if len(model.messages) != 0 {
		t.Error("empty enter should not add messages")
	}
	if cmd != nil {
		t.Error("empty enter should return nil cmd")
	}
}

func TestTUIModel_EnterWhileStreaming(t *testing.T) {
	m := initModel(newTestTUIModel(t))
	m.streaming = true

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(TUIModel)

	if len(model.messages) != 0 {
		t.Error("enter while streaming should not add messages")
	}
	if cmd != nil {
		t.Error("enter while streaming should return nil cmd")
	}
}

func TestTUIModel_StatusBar(t *testing.T) {
	m := initModel(newTestTUIModel(t))
	bar := m.statusBar()

	if !strings.Contains(bar, "Planning") {
		t.Errorf("status bar should contain 'Planning', got: %q", bar)
	}
	if !strings.Contains(bar, "Ctrl+A") {
		t.Errorf("status bar should mention Ctrl+A, got: %q", bar)
	}
}

func TestTUIModel_StatusBarWithPhases(t *testing.T) {
	m := initModel(newTestTUIModel(t))
	m.phaseCount = 5
	bar := m.statusBar()

	if !strings.Contains(bar, "Phases: 5") {
		t.Errorf("status bar should show phase count, got: %q", bar)
	}
}

func TestTUIModel_InputViewWhileStreaming(t *testing.T) {
	m := initModel(newTestTUIModel(t))
	m.streaming = true

	view := m.inputView()
	if !strings.Contains(view, "Thinking") {
		t.Errorf("input view during streaming should show 'Thinking', got: %q", view)
	}
}

func TestTUIModel_InputViewWithError(t *testing.T) {
	m := initModel(newTestTUIModel(t))
	m.err = errors.New("some error")

	view := m.inputView()
	if !strings.Contains(view, "Error occurred") {
		t.Errorf("input view with error should show notice, got: %q", view)
	}
}

func TestTUIModel_RefreshViewport_UserMessage(t *testing.T) {
	m := initModel(newTestTUIModel(t))
	m.messages = append(m.messages, chatMessage{role: "user", content: "hello world"})
	m.refreshViewport()

	content := m.viewport.View()
	if !strings.Contains(content, "hello world") {
		t.Errorf("viewport should contain user message, got: %q", content)
	}
}

func TestTUIModel_RefreshViewport_AssistantMessage(t *testing.T) {
	m := initModel(newTestTUIModel(t))
	m.messages = append(m.messages, chatMessage{
		role:     "assistant",
		content:  "# Hello",
		rendered: "rendered hello",
	})
	m.refreshViewport()

	content := m.viewport.View()
	if !strings.Contains(content, "rendered hello") {
		t.Errorf("viewport should show rendered content, got: %q", content)
	}
}

func TestTUIModel_RefreshViewport_ToolMessage(t *testing.T) {
	m := initModel(newTestTUIModel(t))
	m.messages = append(m.messages, chatMessage{
		role:    "tool",
		content: "[tool: read_file]",
	})
	m.refreshViewport()

	content := m.viewport.View()
	if !strings.Contains(content, "read_file") {
		t.Errorf("viewport should show tool message, got: %q", content)
	}
}

func TestTUIModel_RefreshViewport_StreamingBuffer(t *testing.T) {
	m := initModel(newTestTUIModel(t))
	m.streaming = true
	m.streamBuf.WriteString("partial response...")
	m.refreshViewport()

	content := m.viewport.View()
	if !strings.Contains(content, "partial response...") {
		t.Errorf("viewport should show streaming buffer, got: %q", content)
	}
}

func TestTUIModel_RefreshViewport_ErrorMessage(t *testing.T) {
	m := initModel(newTestTUIModel(t))
	m.messages = append(m.messages, chatMessage{
		role:    "error",
		content: formatAPIError(fmt.Errorf("test error")),
	})
	m.refreshViewport()

	content := m.viewport.View()
	if !strings.Contains(content, "test error") {
		t.Errorf("viewport should show error message, got: %q", content)
	}
}

func TestTUIModel_RenderMarkdown(t *testing.T) {
	m := initModel(newTestTUIModel(t))

	// Render some markdown.
	result := m.renderMarkdown("**bold text**")
	if result == "" {
		t.Error("rendered markdown should not be empty")
	}
	if result == "**bold text**" {
		t.Error("rendered markdown should differ from raw input")
	}
}

func TestTUIModel_RenderMarkdown_Empty(t *testing.T) {
	m := initModel(newTestTUIModel(t))
	result := m.renderMarkdown("")
	if result != "" {
		t.Errorf("empty input should return empty, got: %q", result)
	}
}

func TestTUIModel_RecalcSizes(t *testing.T) {
	m := newTestTUIModel(t)
	m.width = 120
	m.height = 40
	m.recalcSizes()

	expectedVPHeight := 40 - 5 - 1 // height - input - status
	if m.viewport.Height != expectedVPHeight {
		t.Errorf("viewport height = %d, want %d", m.viewport.Height, expectedVPHeight)
	}
	if m.viewport.Width != 120 {
		t.Errorf("viewport width = %d, want 120", m.viewport.Width)
	}
}

func TestReadStreamMsg_Done(t *testing.T) {
	ch := make(chan StreamChunk, 1)
	ch <- StreamChunk{Done: true}

	msg := readStreamMsg(ch)
	if _, ok := msg.(StreamDoneMsg); !ok {
		t.Errorf("expected StreamDoneMsg, got %T", msg)
	}
}

func TestReadStreamMsg_Error(t *testing.T) {
	ch := make(chan StreamChunk, 1)
	ch <- StreamChunk{Err: errors.New("fail")}

	msg := readStreamMsg(ch)
	if errMsg, ok := msg.(StreamErrMsg); !ok {
		t.Errorf("expected StreamErrMsg, got %T", msg)
	} else if errMsg.Err.Error() != "fail" {
		t.Errorf("error = %v, want 'fail'", errMsg.Err)
	}
}

func TestReadStreamMsg_ToolUse(t *testing.T) {
	ch := make(chan StreamChunk, 1)
	ch <- StreamChunk{
		ToolUse: &ToolUseChunk{
			ID:   "t1",
			Name: "validate_spec",
		},
	}

	msg := readStreamMsg(ch)
	if toolMsg, ok := msg.(ToolUseMsg); !ok {
		t.Errorf("expected ToolUseMsg, got %T", msg)
	} else if toolMsg.Chunk.Name != "validate_spec" {
		t.Errorf("tool name = %q, want 'validate_spec'", toolMsg.Chunk.Name)
	}
}

func TestReadStreamMsg_Text(t *testing.T) {
	ch := make(chan StreamChunk, 1)
	ch <- StreamChunk{Text: "hello"}

	msg := readStreamMsg(ch)
	if batch, ok := msg.(streamBatchMsg); !ok {
		t.Errorf("expected streamBatchMsg, got %T", msg)
	} else if batch.text != "hello" {
		t.Errorf("text = %q, want 'hello'", batch.text)
	}
}

func TestReadStreamMsg_ClosedChannel(t *testing.T) {
	ch := make(chan StreamChunk)
	close(ch)

	msg := readStreamMsg(ch)
	if _, ok := msg.(StreamDoneMsg); !ok {
		t.Errorf("closed channel should return StreamDoneMsg, got %T", msg)
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		input string
		width int
		want  string
	}{
		{"hi", 5, "hi   "},
		{"hello world", 5, "hello"},
		{"abc", 3, "abc"},
	}
	for _, tc := range tests {
		got := padRight(tc.input, tc.width)
		if got != tc.want {
			t.Errorf("padRight(%q, %d) = %q, want %q", tc.input, tc.width, got, tc.want)
		}
	}
}

func TestPadLeft(t *testing.T) {
	tests := []struct {
		input string
		width int
		want  string
	}{
		{"hi", 5, "   hi"},
		{"hello world", 5, "world"},
		{"abc", 3, "abc"},
	}
	for _, tc := range tests {
		got := padLeft(tc.input, tc.width)
		if got != tc.want {
			t.Errorf("padLeft(%q, %d) = %q, want %q", tc.input, tc.width, got, tc.want)
		}
	}
}

func TestNewTUIModel_SpinnerInitialized(t *testing.T) {
	m := newTestTUIModel(t)
	// The spinner should produce non-empty View output.
	view := m.spinner.View()
	if view == "" {
		t.Error("spinner should produce non-empty view")
	}
}

func TestFormatAPIError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		contains string
	}{
		{
			name:     "context_canceled",
			err:      errors.New("stream: context canceled"),
			contains: "interrupted",
		},
		{
			name:     "timeout",
			err:      errors.New("context deadline exceeded"),
			contains: "timed out",
		},
		{
			name:     "unauthorized",
			err:      errors.New("401 Unauthorized"),
			contains: "Authentication failed",
		},
		{
			name:     "rate_limit",
			err:      errors.New("429 rate limit exceeded"),
			contains: "Rate limited",
		},
		{
			name:     "overloaded",
			err:      errors.New("529 API overloaded"),
			contains: "overloaded",
		},
		{
			name:     "connection_refused",
			err:      errors.New("connection refused"),
			contains: "Cannot reach",
		},
		{
			name:     "server_error",
			err:      errors.New("500 internal server error"),
			contains: "server error",
		},
		{
			name:     "unknown",
			err:      errors.New("something weird happened"),
			contains: "An API error occurred",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := formatAPIError(tc.err)
			if !strings.Contains(result, tc.contains) {
				t.Errorf("formatAPIError(%q) = %q, want it to contain %q", tc.err, result, tc.contains)
			}
			// All results should include the raw error in Details.
			if !strings.Contains(result, tc.err.Error()) {
				t.Errorf("formatAPIError should include raw error in details, got: %q", result)
			}
		})
	}
}

func TestTUIModel_StreamErrAppendsMessage(t *testing.T) {
	m := initModel(newTestTUIModel(t))
	m.streaming = true

	testErr := errors.New("connection refused")
	updated, _ := m.Update(StreamErrMsg{Err: testErr})
	model := updated.(TUIModel)

	if len(model.messages) != 1 {
		t.Fatalf("expected 1 message (error), got %d", len(model.messages))
	}
	if model.messages[0].role != "error" {
		t.Errorf("message role = %q, want 'error'", model.messages[0].role)
	}
	if !strings.Contains(model.messages[0].content, "Cannot reach") {
		t.Errorf("error message should contain friendly text, got: %q", model.messages[0].content)
	}
	if !strings.Contains(model.messages[0].content, "connection refused") {
		t.Errorf("error message should contain raw error, got: %q", model.messages[0].content)
	}
}

func TestTUIModel_StreamTruncated_AutoContinues(t *testing.T) {
	m := initModel(newTestTUIModel(t))
	m.streaming = true
	m.streamBuf.WriteString("partial response")

	updated, cmd := m.Update(StreamTruncatedMsg{})
	model := updated.(TUIModel)

	// Partial response should be finalized as an assistant message.
	if len(model.messages) != 1 {
		t.Fatalf("expected 1 message (assistant), got %d", len(model.messages))
	}
	if model.messages[0].role != "assistant" {
		t.Errorf("message role = %q, want 'assistant'", model.messages[0].role)
	}
	if model.messages[0].content != "partial response" {
		t.Errorf("content = %q, want 'partial response'", model.messages[0].content)
	}

	// Should still be streaming (auto-continue in progress).
	if !model.streaming {
		t.Error("streaming should remain true during auto-continue")
	}

	// Continue count should be incremented.
	if model.continueCount != 1 {
		t.Errorf("continueCount = %d, want 1", model.continueCount)
	}

	// Should return a cmd (auto-continue).
	if cmd == nil {
		t.Error("expected a cmd for auto-continue")
	}
}

func TestTUIModel_StreamTruncated_ExhaustsLimit(t *testing.T) {
	m := initModel(newTestTUIModel(t))
	m.streaming = true
	m.continueCount = maxAutoContinues // Already at the limit.
	m.streamBuf.WriteString("final partial")

	updated, _ := m.Update(StreamTruncatedMsg{})
	model := updated.(TUIModel)

	// Streaming should stop.
	if model.streaming {
		t.Error("streaming should be false after exhausting auto-continues")
	}

	// Should have assistant message + truncation note.
	if len(model.messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(model.messages))
	}
	if model.messages[0].role != "assistant" {
		t.Errorf("messages[0].role = %q, want 'assistant'", model.messages[0].role)
	}
	if !strings.Contains(model.messages[1].content, "token limit") {
		t.Errorf("expected truncation note, got: %q", model.messages[1].content)
	}
}

func TestTUIModel_ContinueCount_ResetsOnUserMessage(t *testing.T) {
	m := initModel(newTestTUIModel(t))
	m.continueCount = 2

	// sendMessageCmd resets the counter.
	m.sendMessageCmd("hello")

	if m.continueCount != 0 {
		t.Errorf("continueCount = %d, want 0 after sendMessageCmd", m.continueCount)
	}
}

func TestTUIModel_ContinueCount_ResetsOnReset(t *testing.T) {
	m := initModel(newTestTUIModel(t))
	m.continueCount = 2

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	model := updated.(TUIModel)

	if model.continueCount != 0 {
		t.Errorf("continueCount = %d, want 0 after Ctrl+R", model.continueCount)
	}
}

func TestTUIModel_CtrlA_SendsGenerateMessage(t *testing.T) {
	m := initModel(newTestTUIModel(t))

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	model := updated.(TUIModel)

	// Should have added a user message.
	if len(model.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(model.messages))
	}
	if model.messages[0].role != "user" {
		t.Errorf("message role = %q, want 'user'", model.messages[0].role)
	}
	if !strings.Contains(model.messages[0].content, "generate_single_phase") {
		t.Errorf("Ctrl+A message should mention generate_single_phase, got: %q", model.messages[0].content)
	}
	if !strings.Contains(model.messages[0].content, "one phase at a time") {
		t.Errorf("Ctrl+A message should mention 'one phase at a time', got: %q", model.messages[0].content)
	}
	if cmd == nil {
		t.Error("expected a cmd from Ctrl+A")
	}
}

func TestTUIModel_StatusBar_GenerateLabel(t *testing.T) {
	m := initModel(newTestTUIModel(t))
	bar := m.statusBar()

	if !strings.Contains(bar, "Ctrl+A: generate") {
		t.Errorf("status bar should show 'Ctrl+A: generate', got: %q", bar)
	}
}

func TestReadStreamMsg_Truncated(t *testing.T) {
	ch := make(chan StreamChunk, 1)
	ch <- StreamChunk{Truncated: true}

	msg := readStreamMsg(ch)
	if _, ok := msg.(StreamTruncatedMsg); !ok {
		t.Errorf("expected StreamTruncatedMsg, got %T", msg)
	}
}

func TestTUIModel_GenerationTracking_FirstToolCall(t *testing.T) {
	m := initModel(newTestTUIModel(t))

	chunk := ToolUseChunk{
		ID:    "tool_gen1",
		Name:  "generate_single_phase",
		Input: json.RawMessage(`{"id":"1A"}`),
	}

	updated, cmd := m.Update(ToolUseMsg{Chunk: chunk})
	model := updated.(TUIModel)

	if !model.generating {
		t.Error("generating should be true after generate_single_phase ToolUseMsg")
	}
	if !model.genInFlight {
		t.Error("genInFlight should be true after generate_single_phase ToolUseMsg")
	}
	if model.genCurrentPhase != "1A" {
		t.Errorf("genCurrentPhase = %q, want '1A'", model.genCurrentPhase)
	}
	if len(model.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(model.messages))
	}
	if !strings.Contains(model.messages[0].content, "generating phase 1A") {
		t.Errorf("message should contain 'generating phase 1A', got: %q", model.messages[0].content)
	}
	if !strings.Contains(model.messages[0].content, "0 completed") {
		t.Errorf("message should contain '0 completed', got: %q", model.messages[0].content)
	}
	if cmd == nil {
		t.Error("should return a cmd to execute the tool")
	}
}

func TestTUIModel_GenerationTracking_ToolResult(t *testing.T) {
	m := initModel(newTestTUIModel(t))
	m.generating = true
	m.genInFlight = true
	m.genCurrentPhase = "1A"

	updated, _ := m.Update(ToolResultMsg{
		ToolID:  "tool_gen1",
		Result:  "ok",
		IsError: false,
	})
	model := updated.(TUIModel)

	if model.genCompleted != 1 {
		t.Errorf("genCompleted = %d, want 1", model.genCompleted)
	}
	if model.genInFlight {
		t.Error("genInFlight should be false after ToolResultMsg")
	}
}

func TestTUIModel_GenerationTracking_CompletionOnDone(t *testing.T) {
	m := initModel(newTestTUIModel(t))
	m.streaming = true
	m.generating = true
	m.genCompleted = 3

	updated, _ := m.Update(StreamDoneMsg{})
	model := updated.(TUIModel)

	if model.generating {
		t.Error("generating should be false after StreamDoneMsg")
	}
	if model.phaseCount != 3 {
		t.Errorf("phaseCount = %d, want 3", model.phaseCount)
	}
}

func TestTUIModel_GenerationTracking_NonGenToolUnaffected(t *testing.T) {
	m := initModel(newTestTUIModel(t))

	chunk := ToolUseChunk{
		ID:    "tool_rf1",
		Name:  "read_file",
		Input: json.RawMessage(`{"path":"main.go"}`),
	}

	updated, _ := m.Update(ToolUseMsg{Chunk: chunk})
	model := updated.(TUIModel)

	if model.generating {
		t.Error("generating should remain false for non-generation tool")
	}
	if model.genInFlight {
		t.Error("genInFlight should remain false for non-generation tool")
	}
	if len(model.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(model.messages))
	}
	if !strings.Contains(model.messages[0].content, "[tool: read_file]") {
		t.Errorf("message should use generic format, got: %q", model.messages[0].content)
	}
}

func TestTUIModel_GenerationTracking_NonGenToolDuringGeneration(t *testing.T) {
	m := initModel(newTestTUIModel(t))
	m.generating = true
	m.genCompleted = 2
	m.genInFlight = false // not in flight — a read_file result arrives

	updated, _ := m.Update(ToolResultMsg{
		ToolID:  "tool_rf1",
		Result:  "file contents",
		IsError: false,
	})
	model := updated.(TUIModel)

	if model.genCompleted != 2 {
		t.Errorf("genCompleted = %d, want 2 (should not increment for non-gen tool)", model.genCompleted)
	}
}

func TestTUIModel_InputView_Generating(t *testing.T) {
	m := initModel(newTestTUIModel(t))
	m.streaming = true
	m.generating = true
	m.genCompleted = 2
	m.genCurrentPhase = "2A"

	view := m.inputView()
	if !strings.Contains(view, "Generating phases") {
		t.Errorf("input view should show 'Generating phases', got: %q", view)
	}
	if !strings.Contains(view, "2 completed") {
		t.Errorf("input view should show '2 completed', got: %q", view)
	}
	if !strings.Contains(view, "writing 2A") {
		t.Errorf("input view should show 'writing 2A', got: %q", view)
	}
}

func TestTUIModel_StatusBar_Generating(t *testing.T) {
	m := initModel(newTestTUIModel(t))
	m.generating = true
	m.genCompleted = 2
	m.genCurrentPhase = "2A"

	bar := m.statusBar()
	if !strings.Contains(bar, "Generating:") {
		t.Errorf("status bar should contain 'Generating:', got: %q", bar)
	}
	if !strings.Contains(bar, "2 completed") {
		t.Errorf("status bar should contain '2 completed', got: %q", bar)
	}
	if !strings.Contains(bar, "writing 2A") {
		t.Errorf("status bar should contain 'writing 2A', got: %q", bar)
	}
}
