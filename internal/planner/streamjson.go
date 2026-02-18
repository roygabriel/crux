package planner

import (
	"encoding/json"
	"fmt"
)

// CLIStreamEvent represents a single JSON event from a CLI agent's
// stream-json output. CLI agents (Claude Code, Codex, Gemini) emit
// newline-delimited JSON where each line is a stream event.
//
// Claude Code wraps Anthropic API events in {"type":"stream_event","event":{...}}.
// The inner event follows the Anthropic streaming format.
type CLIStreamEvent struct {
	// Type is the outer event type ("stream_event", "ping", "result").
	Type string `json:"type"`
	// Event holds the inner Anthropic API event (for "stream_event" type).
	Event *innerEvent `json:"event,omitempty"`
}

// innerEvent mirrors the Anthropic streaming event structure.
type innerEvent struct {
	// Type is the API event type (message_start, content_block_start,
	// content_block_delta, content_block_stop, message_delta, message_stop).
	Type string `json:"type"`
	// Index is the content block index.
	Index int `json:"index,omitempty"`
	// ContentBlock is present for content_block_start events.
	ContentBlock *contentBlock `json:"content_block,omitempty"`
	// Delta is present for content_block_delta and message_delta events.
	Delta *eventDelta `json:"delta,omitempty"`
	// Message is present for message_start events.
	Message *eventMessage `json:"message,omitempty"`
}

// contentBlock describes a content block in content_block_start events.
type contentBlock struct {
	Type  string          `json:"type"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Text  string          `json:"text,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// eventDelta describes the delta in content_block_delta or message_delta events.
type eventDelta struct {
	Type        string `json:"type,omitempty"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
}

// eventMessage describes the message in message_start events.
type eventMessage struct {
	ID    string `json:"id,omitempty"`
	Model string `json:"model,omitempty"`
	Role  string `json:"role,omitempty"`
}

// parsedBlock accumulates a tool_use content block across start+delta events.
type parsedBlock struct {
	blockType string
	id        string
	name      string
	inputBuf  []byte
}

// CLIStreamParser accumulates stream-json events and produces StreamChunks.
// It tracks content block state to reassemble tool_use inputs from deltas.
type CLIStreamParser struct {
	blocks map[int]*parsedBlock
}

// NewCLIStreamParser creates a new stream-json parser.
func NewCLIStreamParser() *CLIStreamParser {
	return &CLIStreamParser{
		blocks: make(map[int]*parsedBlock),
	}
}

// ParseLine parses a single line of stream-json output and returns zero or
// more StreamChunks. Most lines produce zero or one chunk; content_block_stop
// for tool_use blocks produces a ToolUse chunk.
func (p *CLIStreamParser) ParseLine(line []byte) ([]StreamChunk, error) {
	if len(line) == 0 {
		return nil, nil
	}

	var ev CLIStreamEvent
	if err := json.Unmarshal(line, &ev); err != nil {
		return nil, fmt.Errorf("parse stream-json: %w", err)
	}

	switch ev.Type {
	case "stream_event":
		return p.handleStreamEvent(ev.Event)
	case "ping":
		return nil, nil
	default:
		return nil, nil
	}
}

// handleStreamEvent processes the inner Anthropic API event.
func (p *CLIStreamParser) handleStreamEvent(ev *innerEvent) ([]StreamChunk, error) {
	if ev == nil {
		return nil, nil
	}

	switch ev.Type {
	case "content_block_start":
		if ev.ContentBlock != nil {
			p.blocks[ev.Index] = &parsedBlock{
				blockType: ev.ContentBlock.Type,
				id:        ev.ContentBlock.ID,
				name:      ev.ContentBlock.Name,
			}
		}
		return nil, nil

	case "content_block_delta":
		if ev.Delta == nil {
			return nil, nil
		}
		switch ev.Delta.Type {
		case "text_delta":
			if ev.Delta.Text != "" {
				return []StreamChunk{{Text: ev.Delta.Text}}, nil
			}
		case "input_json_delta":
			if b, ok := p.blocks[ev.Index]; ok {
				b.inputBuf = append(b.inputBuf, []byte(ev.Delta.PartialJSON)...)
			}
		}
		return nil, nil

	case "content_block_stop":
		b, ok := p.blocks[ev.Index]
		if !ok {
			return nil, nil
		}
		delete(p.blocks, ev.Index)

		if b.blockType == "tool_use" && b.id != "" {
			input := json.RawMessage(b.inputBuf)
			if len(input) == 0 {
				input = json.RawMessage(`{}`)
			}
			return []StreamChunk{{
				ToolUse: &ToolUseChunk{
					ID:    b.id,
					Name:  b.name,
					Input: input,
				},
			}}, nil
		}
		return nil, nil

	case "message_delta":
		if ev.Delta != nil && ev.Delta.StopReason == "end_turn" {
			return []StreamChunk{{Done: true}}, nil
		}
		return nil, nil

	case "message_stop":
		return []StreamChunk{{Done: true}}, nil

	default:
		return nil, nil
	}
}
