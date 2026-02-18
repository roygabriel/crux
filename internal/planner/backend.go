package planner

import (
	"context"
	"encoding/json"
)

// Backend defines the conversation backend for the planning agent. It
// abstracts over the Anthropic SDK and CLI agent implementations, allowing
// the TUI to work with either.
type Backend interface {
	// SendMessage sends a user message and returns a channel that streams
	// the assistant's response. The channel is closed when the response completes.
	SendMessage(ctx context.Context, userMsg string) (<-chan StreamChunk, error)
	// HandleToolResult sends a tool execution result back to the model and
	// continues streaming.
	HandleToolResult(ctx context.Context, toolUseID, result string, isError bool) (<-chan StreamChunk, error)
	// History returns the conversation history as display-friendly messages.
	History() []Message
	// Reset clears the conversation history.
	Reset()
	// SystemPrompt returns the rendered system prompt.
	SystemPrompt() string
}

// ToolDef is a backend-agnostic tool definition for registration with
// conversation backends.
type ToolDef struct {
	// Name is the tool name.
	Name string `json:"name"`
	// Description is a human-readable description of the tool.
	Description string `json:"description"`
	// InputSchema is the JSON Schema for the tool's input parameters.
	InputSchema json.RawMessage `json:"input_schema"`
}
