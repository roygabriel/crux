package planner

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/roygabriel/crux/internal/instruct/prefs"
)

// DefaultModel is the model used when none is specified.
const DefaultModel = "claude-sonnet-4-5-20250929"

// defaultMaxTokens is the fallback output token limit when none is configured.
const defaultMaxTokens = 16384

// connectTimeout is the maximum time to wait for the API to accept a request
// (i.e. receive the first message_start event).
const connectTimeout = 2 * time.Minute

// streamTimeout is the maximum total duration for a streaming API response
// once the API has accepted the request.
const streamTimeout = 10 * time.Minute

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

// Agent wraps a Backend for multi-turn streaming conversation with tool use.
// It delegates all conversation logic to the underlying Backend and provides
// a stable API surface for the TUI and tool registration.
type Agent struct {
	backend     Backend
	sdkBackend  *SDKBackend // non-nil only for SDK backend (tool registration)
	projectRoot string
}

// NewAgent creates a new planning agent using the SDK backend. It validates
// the API key, builds the system prompt from project context and preferences,
// and initialises the Anthropic client.
func NewAgent(apiKey, model string, projectCtx ProjectContext, p *prefs.Preferences, logger *slog.Logger, maxTokens int, extraOpts ...option.RequestOption) (*Agent, error) {
	systemPrompt := BuildSystemPrompt(projectCtx, p)
	sdk, err := NewSDKBackend(apiKey, model, systemPrompt, logger, maxTokens, extraOpts...)
	if err != nil {
		return nil, err
	}

	return &Agent{
		backend:    sdk,
		sdkBackend: sdk,
	}, nil
}

// NewAgentWithBackend creates a new planning agent with an arbitrary Backend
// implementation. This is used for CLI agent backends.
func NewAgentWithBackend(backend Backend) *Agent {
	return &Agent{
		backend: backend,
	}
}

// SendMessage sends a user message and returns a channel that streams the
// assistant's response. The channel is closed when the response completes.
func (a *Agent) SendMessage(ctx context.Context, userMsg string) (<-chan StreamChunk, error) {
	return a.backend.SendMessage(ctx, userMsg)
}

// HandleToolResult sends a tool execution result back to the model and
// continues streaming. The assistant message from the prior stream is already
// in history; this appends the tool result as a user message.
func (a *Agent) HandleToolResult(ctx context.Context, toolUseID, result string, isError bool) (<-chan StreamChunk, error) {
	return a.backend.HandleToolResult(ctx, toolUseID, result, isError)
}

// History converts the internal message history to display-friendly Message
// values. Only text content is included; tool use and tool result blocks are
// omitted.
func (a *Agent) History() []Message {
	return a.backend.History()
}

// Reset clears the conversation history.
func (a *Agent) Reset() {
	a.backend.Reset()
}

// SetTools stores tool definitions for API calls. Tools are included in every
// subsequent request to the model. Only works with the SDK backend.
func (a *Agent) SetTools(tools []anthropic.ToolUnionParam) {
	if a.sdkBackend != nil {
		a.sdkBackend.SetTools(tools)
	}
}

// SystemPrompt returns the rendered system prompt for inspection or testing.
func (a *Agent) SystemPrompt() string {
	return a.backend.SystemPrompt()
}

// filterTextOnlyParam builds a MessageParam from a Message, keeping only text
// content blocks. This is used for truncated responses where tool_use blocks
// may have incomplete JSON input that would cause 400 errors on the next call.
func filterTextOnlyParam(msg anthropic.Message) anthropic.MessageParam {
	var blocks []anthropic.ContentBlockParamUnion
	for _, block := range msg.Content {
		if block.Type == "text" {
			blocks = append(blocks, anthropic.NewTextBlock(block.Text))
		}
	}
	if len(blocks) == 0 {
		blocks = append(blocks, anthropic.NewTextBlock("[Response truncated]"))
	}
	return anthropic.MessageParam{
		Role:    anthropic.MessageParamRoleAssistant,
		Content: blocks,
	}
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
