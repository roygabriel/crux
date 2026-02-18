package planner

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// CLIBackend implements Backend by spawning CLI agent subprocesses (Claude
// Code, Codex, Gemini) in --print mode with stream-json output. Each turn
// relaunches the subprocess with the full conversation history in the prompt.
type CLIBackend struct {
	binary       string
	args         []string
	systemPrompt string
	history      []Message
	projectRoot  string
	logger       *slog.Logger
}

// cliAgentConfig maps agent names to their binary and default arguments.
var cliAgentConfig = map[string]struct {
	binary string
	args   []string
}{
	"claude": {
		binary: "claude",
		args:   []string{"--print", "--output-format", "stream-json", "--dangerously-skip-permissions"},
	},
	"codex": {
		binary: "codex",
		args:   []string{"--print", "--output-format", "stream-json"},
	},
	"gemini": {
		binary: "gemini",
		args:   []string{"--print", "--output-format", "stream-json"},
	},
}

// NewCLIBackend creates a new CLI agent backend. The agentName must be one of
// "claude", "codex", or "gemini".
func NewCLIBackend(agentName, systemPrompt, projectRoot string, logger *slog.Logger) (*CLIBackend, error) {
	cfg, ok := cliAgentConfig[agentName]
	if !ok {
		return nil, fmt.Errorf("planner: unknown CLI agent %q (supported: claude, codex, gemini)", agentName)
	}

	if logger == nil {
		logger = slog.Default()
	}

	// Verify the binary is available.
	if _, err := exec.LookPath(cfg.binary); err != nil {
		return nil, fmt.Errorf("planner: %s not found in PATH: %w", cfg.binary, err)
	}

	return &CLIBackend{
		binary:       cfg.binary,
		args:         cfg.args,
		systemPrompt: systemPrompt,
		history:      make([]Message, 0),
		projectRoot:  projectRoot,
		logger:       logger,
	}, nil
}

// SendMessage sends a user message by launching a CLI subprocess with the
// full conversation context and returns a channel of streaming chunks.
func (b *CLIBackend) SendMessage(ctx context.Context, userMsg string) (<-chan StreamChunk, error) {
	b.history = append(b.history, Message{Role: "user", Content: userMsg})
	return b.runAgent(ctx)
}

// HandleToolResult appends the tool result to history and relaunches the
// agent. The tool result is formatted as a user message containing the
// tool output.
func (b *CLIBackend) HandleToolResult(ctx context.Context, toolUseID, result string, isError bool) (<-chan StreamChunk, error) {
	prefix := "Tool result"
	if isError {
		prefix = "Tool error"
	}
	b.history = append(b.history, Message{
		Role:    "user",
		Content: fmt.Sprintf("[%s for %s]: %s", prefix, toolUseID, result),
	})
	return b.runAgent(ctx)
}

// History returns the conversation history.
func (b *CLIBackend) History() []Message {
	msgs := make([]Message, len(b.history))
	copy(msgs, b.history)
	return msgs
}

// Reset clears the conversation history.
func (b *CLIBackend) Reset() {
	b.history = b.history[:0]
}

// SystemPrompt returns the system prompt.
func (b *CLIBackend) SystemPrompt() string {
	return b.systemPrompt
}

// buildPrompt constructs the full prompt from system prompt and conversation
// history. Each turn relaunches the subprocess with the full context.
func (b *CLIBackend) buildPrompt() string {
	var sb strings.Builder

	sb.WriteString("<system>\n")
	sb.WriteString(b.systemPrompt)
	sb.WriteString("\n</system>\n\n")

	for _, msg := range b.history {
		switch msg.Role {
		case "user":
			sb.WriteString("<user>\n")
			sb.WriteString(msg.Content)
			sb.WriteString("\n</user>\n\n")
		case "assistant":
			sb.WriteString("<assistant>\n")
			sb.WriteString(msg.Content)
			sb.WriteString("\n</assistant>\n\n")
		}
	}

	return sb.String()
}

// runAgent spawns the CLI subprocess and streams its output back as chunks.
func (b *CLIBackend) runAgent(ctx context.Context) (<-chan StreamChunk, error) {
	prompt := b.buildPrompt()
	args := append(b.args, "-p", prompt)

	cmd := exec.CommandContext(ctx, b.binary, args...)
	cmd.Dir = b.projectRoot

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("planner: stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("planner: start %s: %w", b.binary, err)
	}

	b.logger.Debug("CLI agent started", "binary", b.binary, "pid", cmd.Process.Pid)

	ch := make(chan StreamChunk, 64)
	go func() {
		defer close(ch)

		parser := NewCLIStreamParser()
		scanner := bufio.NewScanner(stdout)
		// Increase buffer for large JSON lines.
		scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

		var fullText strings.Builder

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			chunks, parseErr := parser.ParseLine(line)
			if parseErr != nil {
				b.logger.Warn("stream-json parse error", "error", parseErr, "line", string(line))
				continue
			}

			for _, chunk := range chunks {
				if chunk.Text != "" {
					fullText.WriteString(chunk.Text)
				}
				ch <- chunk
			}
		}

		if scanErr := scanner.Err(); scanErr != nil {
			b.logger.Error("scanner error", "error", scanErr)
			ch <- StreamChunk{Err: fmt.Errorf("scanner: %w", scanErr)}
		}

		if err := cmd.Wait(); err != nil {
			b.logger.Warn("CLI agent exited with error", "binary", b.binary, "error", err)
		}

		// Append assistant response to history.
		if response := fullText.String(); response != "" {
			b.history = append(b.history, Message{Role: "assistant", Content: response})
		}
	}()

	return ch, nil
}
