// Package docgen generates phase specification and prompt documents using
// the Anthropic API. It supports batch and streaming modes with token budget
// management, retry logic, and cost estimation.
package docgen

// DocType identifies the kind of phase document to generate.
type DocType string

const (
	// DocTypeSpec generates a phase specification document (PHASE<ID>.md).
	DocTypeSpec DocType = "spec"
	// DocTypePrompt generates a prompt execution document (PHASE<ID>-PROMPT.md).
	DocTypePrompt DocType = "prompt"
)

// DocRequest describes a single document generation request.
type DocRequest struct {
	// PhaseID is the phase identifier (e.g. "1", "2A").
	PhaseID string `json:"phase_id"`
	// DocType selects the document kind to generate.
	DocType DocType `json:"doc_type"`
	// Title is the human-readable phase title.
	Title string `json:"title"`
	// DependsOn lists phase IDs that must complete before this phase.
	DependsOn []string `json:"depends_on,omitempty"`
	// Description is a brief summary of the phase's purpose.
	Description string `json:"description"`
	// NumPrompts is the expected number of prompts (used for prompt docs).
	NumPrompts int `json:"num_prompts,omitempty"`
	// Context provides additional information for the generation model.
	Context string `json:"context,omitempty"`
}

// DocResult holds the output from a single document generation.
type DocResult struct {
	// PhaseID identifies which phase this result belongs to.
	PhaseID string `json:"phase_id"`
	// DocType is the kind of document that was generated.
	DocType DocType `json:"doc_type"`
	// Content is the generated markdown content.
	Content string `json:"content"`
	// InputTokens is the number of input tokens consumed.
	InputTokens int `json:"input_tokens"`
	// OutputTokens is the number of output tokens generated.
	OutputTokens int `json:"output_tokens"`
	// Model is the model identifier used for generation.
	Model string `json:"model"`
	// Err holds any error encountered during generation.
	Err error `json:"err,omitempty"`
}

// GenerateMode selects the API interaction pattern.
type GenerateMode string

const (
	// GenerateModeBatch uses the Anthropic Batch API for bulk generation.
	GenerateModeBatch GenerateMode = "batch"
	// GenerateModeStream uses streaming for real-time generation.
	GenerateModeStream GenerateMode = "stream"
)

// GenerateOptions configures a document generation run.
type GenerateOptions struct {
	// Mode selects batch or streaming generation.
	Mode GenerateMode `json:"mode"`
	// Model overrides the default model for generation.
	Model string `json:"model,omitempty"`
	// MaxTokens sets the maximum output tokens per request.
	MaxTokens int `json:"max_tokens,omitempty"`
	// OutputDir is the directory to write generated documents.
	OutputDir string `json:"output_dir,omitempty"`
	// DryRun estimates cost without calling the API (Prompt 3).
	DryRun bool `json:"dry_run,omitempty"`
	// RetryFailed retries only previously failed requests (Prompt 3).
	RetryFailed bool `json:"retry_failed,omitempty"`
}
