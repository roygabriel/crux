package codex

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

const defaultRetryAfter = 60 * time.Second

// Compiled patterns for detecting Codex CLI state from pane output.
var (
	// ansiRe matches ANSI escape sequences for stripping from pane content.
	ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

	// busyPatternRe matches activity indicators in Codex CLI output:
	// braille spinner characters and common progress text.
	busyPatternRe = regexp.MustCompile(`[⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏]|(?i:thinking\.\.\.|working\.\.\.|processing\.\.\.|running\.\.\.)`)

	// errorMsgRe captures the error message from lines starting with an
	// error prefix or OpenAI-specific error types.
	errorMsgRe = regexp.MustCompile(`(?mi)^\s*(?:Error|ERROR|error|APIError|AuthenticationError):\s*(.+)$`)

	// panicRe matches Go panic and fatal error patterns at line start.
	panicRe = regexp.MustCompile(`(?m)^(panic:|fatal error:)\s*(.+)$`)

	// rateLimitRe matches concrete rate-limiting indicators in pane output.
	// It avoids matching benign marketing/help text like "rate limits".
	rateLimitRe = regexp.MustCompile(`(?i)(?:\brate[_\s-]?limit(?:ed)?(?:\s+exceeded)?\b|\b429\b|too many requests|quota exceeded)`)

	// retryAfterRe extracts a numeric duration from retry/wait messages.
	retryAfterRe = regexp.MustCompile(`(?i)(?:retry|wait)\s*(?:after\s+)?(\d+)\s*(s|sec|seconds?|m|min|minutes?)`)

	// fileChangedRe extracts file paths from modification notices in
	// Codex CLI output (e.g. "Applied edit to `path/file.go`").
	fileChangedRe = regexp.MustCompile("(?mi)(?:Created|Updated|Modified|Wrote(?: to)?|Edited|Applied edit to)\\s+`?([^\\s`'\"]+)`?")

	// readySignatureRe matches startup UI hints that indicate Codex is ready
	// for input even when the pane's final line is not a literal prompt.
	readySignatureRe = regexp.MustCompile(`(?i)(?:openai codex\s*\(v[0-9]|>_ openai codex)`)
)

// stripANSI removes ANSI escape sequences from s.
func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// lastNonEmptyLine returns the last non-empty line from s after stripping
// ANSI escape sequences and trimming whitespace.
func lastNonEmptyLine(s string) string {
	s = stripANSI(s)
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if trimmed := strings.TrimSpace(lines[i]); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// lastLines returns the last n lines of s as a single string.
func lastLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// isReadyPrompt reports whether line matches the Codex CLI ready prompt.
func isReadyPrompt(line string) bool {
	return line == ">" || line == "codex>"
}

// isReadySignature reports whether pane content includes known startup
// markers emitted by Codex's interactive shell when it is ready.
func isReadySignature(s string) bool {
	tail := stripANSI(strings.ToLower(lastLines(s, 30)))
	return readySignatureRe.MatchString(tail)
}

// parseRetryDuration extracts a retry duration from s, returning
// defaultRetryAfter if no parseable duration is found.
func parseRetryDuration(s string) time.Duration {
	matches := retryAfterRe.FindStringSubmatch(s)
	if matches == nil {
		return defaultRetryAfter
	}

	n, err := strconv.Atoi(matches[1])
	if err != nil {
		return defaultRetryAfter
	}

	unit := strings.ToLower(matches[2])
	switch {
	case strings.HasPrefix(unit, "m"):
		return time.Duration(n) * time.Minute
	default:
		return time.Duration(n) * time.Second
	}
}

// extractErrors collects error messages from pane content, checking
// panics first (higher severity) then standard error lines.
func extractErrors(s string) []string {
	var errs []string

	for _, match := range panicRe.FindAllStringSubmatch(s, -1) {
		prefix := strings.TrimSpace(match[1])
		msg := strings.TrimSpace(match[2])
		errs = append(errs, prefix+" "+msg)
	}

	for _, match := range errorMsgRe.FindAllStringSubmatch(s, -1) {
		if msg := strings.TrimSpace(match[1]); msg != "" {
			errs = append(errs, msg)
		}
	}

	return errs
}

// extractFilesChanged collects unique file paths from modification
// notices in pane content, preserving order of first appearance.
func extractFilesChanged(s string) []string {
	seen := make(map[string]struct{})
	var files []string

	for _, match := range fileChangedRe.FindAllStringSubmatch(s, -1) {
		path := strings.TrimSpace(match[1])
		if path == "" {
			continue
		}
		if _, ok := seen[path]; !ok {
			seen[path] = struct{}{}
			files = append(files, path)
		}
	}

	return files
}
