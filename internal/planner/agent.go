package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/roygabriel/crux/internal/instruct/prefs"
)

// DefaultModel is the model used when none is specified.
const DefaultModel = "claude-sonnet-4-5-20250929"

// Message is a display-friendly representation of a conversation turn.
type Message struct {
	// Role is "user" or "assistant".
	Role string `json:"role"`
	// Content is the text content of the message.
	Content string `json:"content"`
}

// StreamChunk represents a single piece of a streaming response.
type StreamChunk struct {
	// Text is a text delta from the assistant's response.
	Text string
	// ToolUse is populated when the model invokes a tool.
	ToolUse *ToolUseChunk
	// Done indicates the stream has finished.
	Done bool
	// Err is set if the stream encountered an error.
	Err error
}

// ToolUseChunk contains the details of a tool invocation by the model.
type ToolUseChunk struct {
	// ID is the unique identifier for this tool use.
	ID string `json:"id"`
	// Name is the tool name.
	Name string `json:"name"`
	// Input is the raw JSON input to the tool.
	Input json.RawMessage `json:"input"`
}

// Agent wraps the Anthropic SDK for multi-turn streaming conversation with
// tool use. It manages conversation history and provides a channel-based
// streaming interface.
type Agent struct {
	client       anthropic.Client
	model        anthropic.Model
	systemPrompt string
	history      []anthropic.MessageParam
	tools        []anthropic.ToolUnionParam
	logger       *slog.Logger
	opts         []option.RequestOption
}

// NewAgent creates a new planning agent. It validates the API key, builds the
// system prompt from project context and preferences, and initialises the
// Anthropic client.
func NewAgent(apiKey, model string, projectCtx ProjectContext, p *prefs.Preferences, logger *slog.Logger, extraOpts ...option.RequestOption) (*Agent, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("planner: API key is required (set CRUX_ANTHROPIC_API_KEY or ANTHROPIC_API_KEY)")
	}

	if logger == nil {
		logger = slog.Default()
	}

	m := anthropic.Model(model)
	if model == "" {
		m = anthropic.Model(DefaultModel)
	}

	systemPrompt := BuildSystemPrompt(projectCtx, p)

	opts := append([]option.RequestOption{option.WithAPIKey(apiKey)}, extraOpts...)
	client := anthropic.NewClient(opts...)

	return &Agent{
		client:       client,
		model:        m,
		systemPrompt: systemPrompt,
		history:      make([]anthropic.MessageParam, 0),
		logger:       logger,
		opts:         opts,
	}, nil
}

// SendMessage sends a user message and returns a channel that streams the
// assistant's response. The channel is closed when the response completes.
func (a *Agent) SendMessage(ctx context.Context, userMsg string) (<-chan StreamChunk, error) {
	a.history = append(a.history, anthropic.NewUserMessage(
		anthropic.NewTextBlock(userMsg),
	))
	return a.stream(ctx)
}

// HandleToolResult sends a tool execution result back to the model and
// continues streaming. The assistant message from the prior stream is already
// in history; this appends the tool result as a user message.
func (a *Agent) HandleToolResult(ctx context.Context, toolUseID, result string, isError bool) (<-chan StreamChunk, error) {
	a.history = append(a.history, anthropic.NewUserMessage(
		anthropic.NewToolResultBlock(toolUseID, result, isError),
	))
	return a.stream(ctx)
}

// History converts the internal message history to display-friendly Message
// values. Only text content is included; tool use and tool result blocks are
// omitted.
func (a *Agent) History() []Message {
	msgs := make([]Message, 0, len(a.history))
	for _, mp := range a.history {
		role := string(mp.Role)
		text := extractText(mp)
		if text != "" {
			msgs = append(msgs, Message{Role: role, Content: text})
		}
	}
	return msgs
}

// Reset clears the conversation history.
func (a *Agent) Reset() {
	a.history = a.history[:0]
}

// SetTools stores tool definitions for API calls. Tools are included in every
// subsequent request to the model.
func (a *Agent) SetTools(tools []anthropic.ToolUnionParam) {
	a.tools = tools
}

// SystemPrompt returns the rendered system prompt for inspection or testing.
func (a *Agent) SystemPrompt() string {
	return a.systemPrompt
}

// stream starts a streaming API call and returns a channel of StreamChunks.
// It launches a goroutine that reads from the SDK stream, accumulates the
// message, sends text deltas and tool use chunks, and appends the completed
// message to history.
func (a *Agent) stream(ctx context.Context) (<-chan StreamChunk, error) {
	params := anthropic.MessageNewParams{
		Model:     a.model,
		MaxTokens: 8192,
		System: []anthropic.TextBlockParam{
			{Text: a.systemPrompt},
		},
		Messages: a.history,
	}
	if len(a.tools) > 0 {
		params.Tools = a.tools
	}

	s := a.client.Messages.NewStreaming(ctx, params)

	ch := make(chan StreamChunk, 64)
	go func() {
		defer close(ch)
		defer s.Close()

		msg := anthropic.Message{}
		for s.Next() {
			event := s.Current()
			if err := msg.Accumulate(event); err != nil {
				a.logger.Error("stream accumulate error", "error", err)
				ch <- StreamChunk{Err: fmt.Errorf("accumulate: %w", err)}
				return
			}

			switch ev := event.AsAny().(type) {
			case anthropic.ContentBlockDeltaEvent:
				switch delta := ev.Delta.AsAny().(type) {
				case anthropic.TextDelta:
					if delta.Text != "" {
						ch <- StreamChunk{Text: delta.Text}
					}
				}
			}
		}

		if err := s.Err(); err != nil {
			a.logger.Error("stream error", "error", err)
			ch <- StreamChunk{Err: fmt.Errorf("stream: %w", err)}
			return
		}

		// Append the accumulated message to history.
		a.history = append(a.history, msg.ToParam())

		// If the model stopped to use tools, send tool use chunks.
		if msg.StopReason == anthropic.StopReasonToolUse {
			for _, block := range msg.Content {
				if tu := block.AsAny(); tu != nil {
					if toolUse, ok := tu.(anthropic.ToolUseBlock); ok {
						ch <- StreamChunk{
							ToolUse: &ToolUseChunk{
								ID:    toolUse.ID,
								Name:  toolUse.Name,
								Input: toolUse.Input,
							},
						}
					}
				}
			}
		}

		ch <- StreamChunk{Done: true}
	}()

	return ch, nil
}

// extractText concatenates text content from a MessageParam's content blocks.
func extractText(mp anthropic.MessageParam) string {
	var text string
	for _, block := range mp.Content {
		if block.OfText != nil {
			text += block.OfText.Text
		}
	}
	return text
}
