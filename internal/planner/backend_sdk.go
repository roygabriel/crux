package planner

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// SDKBackend implements Backend using the Anthropic Go SDK for direct API
// streaming. It manages conversation history as SDK-native MessageParam
// values and handles tool registration.
type SDKBackend struct {
	client       anthropic.Client
	model        anthropic.Model
	systemPrompt string
	history      []anthropic.MessageParam
	tools        []anthropic.ToolUnionParam
	logger       *slog.Logger
	maxTokens    int
}

// NewSDKBackend creates a new SDK-based conversation backend. It validates
// the API key and initialises the Anthropic client.
func NewSDKBackend(apiKey, model, systemPrompt string, logger *slog.Logger, maxTokens int, extraOpts ...option.RequestOption) (*SDKBackend, error) {
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

	opts := append([]option.RequestOption{option.WithAPIKey(apiKey)}, extraOpts...)
	client := anthropic.NewClient(opts...)

	return &SDKBackend{
		client:       client,
		model:        m,
		systemPrompt: systemPrompt,
		history:      make([]anthropic.MessageParam, 0),
		logger:       logger,
		maxTokens:    maxTokens,
	}, nil
}

// SetTools stores tool definitions for API calls.
func (b *SDKBackend) SetTools(tools []anthropic.ToolUnionParam) {
	b.tools = tools
}

// SendMessage sends a user message and returns a channel that streams the
// assistant's response.
func (b *SDKBackend) SendMessage(ctx context.Context, userMsg string) (<-chan StreamChunk, error) {
	b.history = append(b.history, anthropic.NewUserMessage(
		anthropic.NewTextBlock(userMsg),
	))
	return b.stream(ctx)
}

// HandleToolResult sends a tool execution result back to the model and
// continues streaming.
func (b *SDKBackend) HandleToolResult(ctx context.Context, toolUseID, result string, isError bool) (<-chan StreamChunk, error) {
	b.history = append(b.history, anthropic.NewUserMessage(
		anthropic.NewToolResultBlock(toolUseID, result, isError),
	))
	return b.stream(ctx)
}

// History converts the internal message history to display-friendly Message
// values.
func (b *SDKBackend) History() []Message {
	msgs := make([]Message, 0, len(b.history))
	for _, mp := range b.history {
		role := string(mp.Role)
		text := extractText(mp)
		if text != "" {
			msgs = append(msgs, Message{Role: role, Content: text})
		}
	}
	return msgs
}

// Reset clears the conversation history.
func (b *SDKBackend) Reset() {
	b.history = b.history[:0]
}

// SystemPrompt returns the rendered system prompt.
func (b *SDKBackend) SystemPrompt() string {
	return b.systemPrompt
}

// stream starts a streaming API call and returns a channel of StreamChunks.
func (b *SDKBackend) stream(ctx context.Context) (<-chan StreamChunk, error) {
	maxTok := b.maxTokens
	if maxTok <= 0 {
		maxTok = defaultMaxTokens
	}

	ctx, cancel := context.WithTimeout(ctx, streamTimeout)

	params := anthropic.MessageNewParams{
		Model:     b.model,
		MaxTokens: int64(maxTok),
		System: []anthropic.TextBlockParam{
			{Text: b.systemPrompt},
		},
		Messages: b.history,
	}
	if len(b.tools) > 0 {
		params.Tools = b.tools
	}

	s := b.client.Messages.NewStreaming(ctx, params)

	ch := make(chan StreamChunk, 64)
	go func() {
		defer cancel()
		defer close(ch)
		defer s.Close()

		connectTimer := time.AfterFunc(connectTimeout, func() {
			b.logger.Warn("connect timeout waiting for API to accept request")
			cancel()
		})
		defer connectTimer.Stop()

		msg := anthropic.Message{}
		for s.Next() {
			event := s.Current()
			if err := msg.Accumulate(event); err != nil {
				b.logger.Error("stream accumulate error", "error", err)
				ch <- StreamChunk{Err: fmt.Errorf("accumulate: %w", err)}
				return
			}

			switch ev := event.AsAny().(type) {
			case anthropic.MessageStartEvent:
				connectTimer.Stop()
				b.logger.Debug("stream accepted by API", "model", ev.Message.Model)
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
			b.logger.Error("stream error", "error", err)
			ch <- StreamChunk{Err: fmt.Errorf("stream: %w", err)}
			return
		}

		// Handle truncation BEFORE appending to history.
		if msg.StopReason == anthropic.StopReasonMaxTokens {
			b.history = append(b.history, filterTextOnlyParam(msg))
			if b.maxTokens > 0 {
				ch <- StreamChunk{
					Err: fmt.Errorf("response truncated: max_tokens (%d) reached — increase planner.max_tokens in config or set CRUX_PLANNER_MAX_TOKENS", maxTok),
				}
				return
			}
			b.logger.Warn("response reached default max_tokens, signaling truncation", "max_tokens", maxTok)
			ch <- StreamChunk{Truncated: true}
			return
		}

		// Safe to append full message — not truncated.
		b.history = append(b.history, msg.ToParam())

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
