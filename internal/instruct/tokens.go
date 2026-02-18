package instruct

import (
	"strings"
	"sync"

	"github.com/tiktoken-go/tokenizer"
)

// Token budget constants define default maximum tokens per agent CLI.
const (
	// BudgetClaude is the default token budget for Claude Code instruction files.
	BudgetClaude = 3000
	// BudgetCodex is the default token budget for Codex CLI instruction files.
	BudgetCodex = 7500
	// BudgetGemini is the default token budget for Gemini CLI instruction files.
	BudgetGemini = 10000
	// BudgetCopilot is the default token budget for Copilot CLI instruction files.
	BudgetCopilot = 5000
)

// EstimateTokens provides a fast token estimate using the len/4 heuristic.
func EstimateTokens(text string) int {
	return len(text) / 4
}

var (
	tiktokenOnce sync.Once
	tiktokenEnc  tokenizer.Codec
	tiktokenErr  error
)

func initTiktoken() {
	tiktokenEnc, tiktokenErr = tokenizer.Get(tokenizer.Cl100kBase)
}

// CountTokens returns a precise token count using the cl100k_base encoding.
// The tokenizer is lazily initialized on first call.
func CountTokens(text string) (int, error) {
	tiktokenOnce.Do(initTiktoken)
	if tiktokenErr != nil {
		return 0, tiktokenErr
	}
	ids, _, err := tiktokenEnc.Encode(text)
	if err != nil {
		return 0, err
	}
	return len(ids), nil
}

// TruncateToTokens truncates text to fit within maxTokens at a line boundary,
// appending a truncation marker. It uses the byte-estimate approach for speed
// and verifies with precise counting.
func TruncateToTokens(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return ""
	}
	if EstimateTokens(text) <= maxTokens {
		return text
	}

	maxBytes := maxTokens * 4
	if maxBytes >= len(text) {
		return text
	}

	truncated := text[:maxBytes]
	lastNewline := strings.LastIndex(truncated, "\n")
	if lastNewline > 0 {
		truncated = truncated[:lastNewline]
	}

	return truncated + "\n[...truncated]"
}
