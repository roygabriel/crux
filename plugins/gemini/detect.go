package gemini

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

const defaultRetryAfter = 60 * time.Second

// Compiled patterns for detecting Gemini CLI state from pane output.
var (
	// ansiRe matches ANSI escape sequences for stripping from pane content.
	ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

	// busyPatternRe matches activity indicators in Gemini CLI output:
	// braille spinner characters and common progress text.
	busyPatternRe = regexp.MustCompile(`[⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏]|(?i:thinking\.\.\.|generating\.\.\.|searching\.\.\.)`)

	// errorMsgRe captures the error message from lines starting with an
	// error prefix or Google-specific error types.
	errorMsgRe = regexp.MustCompile(`(?mi)^\s*(?:Error|ERROR|error|APIError|GoogleAPIError|PermissionDenied):\s*(.+)$`)

	// panicRe matches Go panic and fatal error patterns at line start.
	panicRe = regexp.MustCompile(`(?m)^(panic:|fatal error:)\s*(.+)$`)

	// rateLimitRe matches rate-limiting indicators in pane output.
	rateLimitRe = regexp.MustCompile(`(?i)(?:rate[_\s-]?limit|429|too many requests|resource exhausted|quota exceeded)`)

	// retryAfterRe extracts a numeric duration from retry/wait messages.
	retryAfterRe = regexp.MustCompile(`(?i)(?:retry|wait)\s*(?:after\s+)?(\d+)\s*(s|sec|seconds?|m|min|minutes?)`)

	// fileChangedRe extracts file paths from modification notices in
	// Gemini CLI output (e.g. "Created `path/file.go`").
	fileChangedRe = regexp.MustCompile("(?mi)(?:Created|Updated|Modified|Wrote(?: to)?|Edited|Applied edit to)\\s+`?([^\\s`'\"]+)`?")

	// readySignatureRe matches startup UI hints that indicate Gemini CLI is
	// ready for user input even if the last line is not just ">".
	readySignatureRe = regexp.MustCompile(`(?i)(?:type your message or @path/to/file|\? for shortcuts|/model auto \(gemini)`)
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

// isReadyPrompt reports whether line matches the Gemini CLI ready prompt.
func isReadyPrompt(line string) bool {
	return line == ">" || line == "gemini>"
}

// isReadySignature reports whether pane content includes known startup
// markers emitted by Gemini CLI when it is idle and interactive.
func isReadySignature(s string) bool {
	tail := stripANSI(strings.ToLower(lastLines(s, 40)))
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
