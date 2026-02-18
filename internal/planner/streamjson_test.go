package planner

import (
	"testing"
)

func TestCLIStreamParser_TextDelta(t *testing.T) {
	p := NewCLIStreamParser()
	line := []byte(`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello world"}}}`)

	chunks, err := p.ParseLine(line)
	if err != nil {
		t.Fatalf("ParseLine: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Text != "Hello world" {
		t.Errorf("text = %q, want 'Hello world'", chunks[0].Text)
	}
}

func TestCLIStreamParser_ToolUse(t *testing.T) {
	p := NewCLIStreamParser()

	lines := [][]byte{
		[]byte(`{"type":"stream_event","event":{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_01abc","name":"read_file","input":{}}}}`),
		[]byte(`{"type":"stream_event","event":{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"path\":"}}}`),
		[]byte(`{"type":"stream_event","event":{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"\"main.go\"}"}}}`),
		[]byte(`{"type":"stream_event","event":{"type":"content_block_stop","index":1}}`),
	}

	var allChunks []StreamChunk
	for _, line := range lines {
		chunks, err := p.ParseLine(line)
		if err != nil {
			t.Fatalf("ParseLine: %v", err)
		}
		allChunks = append(allChunks, chunks...)
	}

	if len(allChunks) != 1 {
		t.Fatalf("expected 1 chunk (tool_use), got %d", len(allChunks))
	}

	tu := allChunks[0].ToolUse
	if tu == nil {
		t.Fatal("expected ToolUse chunk")
	}
	if tu.ID != "toolu_01abc" {
		t.Errorf("ID = %q, want 'toolu_01abc'", tu.ID)
	}
	if tu.Name != "read_file" {
		t.Errorf("Name = %q, want 'read_file'", tu.Name)
	}
	if string(tu.Input) != `{"path":"main.go"}` {
		t.Errorf("Input = %s, want {\"path\":\"main.go\"}", string(tu.Input))
	}
}

func TestCLIStreamParser_MessageStop(t *testing.T) {
	p := NewCLIStreamParser()
	line := []byte(`{"type":"stream_event","event":{"type":"message_stop"}}`)

	chunks, err := p.ParseLine(line)
	if err != nil {
		t.Fatalf("ParseLine: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if !chunks[0].Done {
		t.Error("expected Done = true")
	}
}

func TestCLIStreamParser_MessageDeltaEndTurn(t *testing.T) {
	p := NewCLIStreamParser()
	line := []byte(`{"type":"stream_event","event":{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":15}}}`)

	chunks, err := p.ParseLine(line)
	if err != nil {
		t.Fatalf("ParseLine: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if !chunks[0].Done {
		t.Error("expected Done = true for end_turn")
	}
}

func TestCLIStreamParser_MessageDeltaToolUse(t *testing.T) {
	p := NewCLIStreamParser()
	// message_delta with stop_reason=tool_use should not emit Done.
	line := []byte(`{"type":"stream_event","event":{"type":"message_delta","delta":{"stop_reason":"tool_use"}}}`)

	chunks, err := p.ParseLine(line)
	if err != nil {
		t.Fatalf("ParseLine: %v", err)
	}
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for tool_use stop reason, got %d", len(chunks))
	}
}

func TestCLIStreamParser_Ping(t *testing.T) {
	p := NewCLIStreamParser()
	line := []byte(`{"type":"ping"}`)

	chunks, err := p.ParseLine(line)
	if err != nil {
		t.Fatalf("ParseLine: %v", err)
	}
	if len(chunks) != 0 {
		t.Errorf("ping should produce no chunks, got %d", len(chunks))
	}
}

func TestCLIStreamParser_EmptyLine(t *testing.T) {
	p := NewCLIStreamParser()
	chunks, err := p.ParseLine([]byte{})
	if err != nil {
		t.Fatalf("ParseLine: %v", err)
	}
	if len(chunks) != 0 {
		t.Errorf("empty line should produce no chunks, got %d", len(chunks))
	}
}

func TestCLIStreamParser_InvalidJSON(t *testing.T) {
	p := NewCLIStreamParser()
	_, err := p.ParseLine([]byte(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestCLIStreamParser_MessageStart(t *testing.T) {
	p := NewCLIStreamParser()
	line := []byte(`{"type":"stream_event","event":{"type":"message_start","message":{"id":"msg_01","model":"claude-sonnet-4-5-20250929","role":"assistant","content":[]}}}`)

	chunks, err := p.ParseLine(line)
	if err != nil {
		t.Fatalf("ParseLine: %v", err)
	}
	if len(chunks) != 0 {
		t.Errorf("message_start should produce no chunks, got %d", len(chunks))
	}
}

func TestCLIStreamParser_ContentBlockStopText(t *testing.T) {
	p := NewCLIStreamParser()

	// Start a text block.
	_, _ = p.ParseLine([]byte(`{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}}`))

	// Stop it — no tool_use, so no chunk.
	chunks, err := p.ParseLine([]byte(`{"type":"stream_event","event":{"type":"content_block_stop","index":0}}`))
	if err != nil {
		t.Fatalf("ParseLine: %v", err)
	}
	if len(chunks) != 0 {
		t.Errorf("text block stop should produce no chunks, got %d", len(chunks))
	}
}

func TestCLIStreamParser_FullConversation(t *testing.T) {
	p := NewCLIStreamParser()

	lines := [][]byte{
		[]byte(`{"type":"stream_event","event":{"type":"message_start","message":{"id":"msg_01","model":"claude-sonnet-4-5-20250929","role":"assistant","content":[]}}}`),
		[]byte(`{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}}`),
		[]byte(`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Let me "}}}`),
		[]byte(`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"help you."}}}`),
		[]byte(`{"type":"stream_event","event":{"type":"content_block_stop","index":0}}`),
		[]byte(`{"type":"stream_event","event":{"type":"message_delta","delta":{"stop_reason":"end_turn"}}}`),
		[]byte(`{"type":"stream_event","event":{"type":"message_stop"}}`),
	}

	var texts []string
	var doneCount int
	for _, line := range lines {
		chunks, err := p.ParseLine(line)
		if err != nil {
			t.Fatalf("ParseLine: %v", err)
		}
		for _, c := range chunks {
			if c.Text != "" {
				texts = append(texts, c.Text)
			}
			if c.Done {
				doneCount++
			}
		}
	}

	if len(texts) != 2 {
		t.Fatalf("expected 2 text chunks, got %d", len(texts))
	}
	if texts[0] != "Let me " {
		t.Errorf("text[0] = %q, want 'Let me '", texts[0])
	}
	if texts[1] != "help you." {
		t.Errorf("text[1] = %q, want 'help you.'", texts[1])
	}
	// Done emitted from both message_delta(end_turn) and message_stop.
	if doneCount < 1 {
		t.Error("expected at least 1 Done chunk")
	}
}

func TestCLIStreamParser_UnknownType(t *testing.T) {
	p := NewCLIStreamParser()
	line := []byte(`{"type":"unknown_event","data":"something"}`)

	chunks, err := p.ParseLine(line)
	if err != nil {
		t.Fatalf("ParseLine: %v", err)
	}
	if len(chunks) != 0 {
		t.Errorf("unknown event type should produce no chunks, got %d", len(chunks))
	}
}

func TestCLIStreamParser_ToolUseEmptyInput(t *testing.T) {
	p := NewCLIStreamParser()

	lines := [][]byte{
		[]byte(`{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_01","name":"list_files","input":{}}}}`),
		[]byte(`{"type":"stream_event","event":{"type":"content_block_stop","index":0}}`),
	}

	var allChunks []StreamChunk
	for _, line := range lines {
		chunks, err := p.ParseLine(line)
		if err != nil {
			t.Fatalf("ParseLine: %v", err)
		}
		allChunks = append(allChunks, chunks...)
	}

	if len(allChunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(allChunks))
	}
	if string(allChunks[0].ToolUse.Input) != `{}` {
		t.Errorf("empty input should default to {}, got: %s", allChunks[0].ToolUse.Input)
	}
}
