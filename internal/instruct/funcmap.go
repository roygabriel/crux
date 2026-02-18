package instruct

import (
	"fmt"
	"strings"
	"text/template"
	"unicode"
)

// DefaultFuncMap returns the template function map used by the instruction
// engine templates. All functions handle nil and empty inputs gracefully.
func DefaultFuncMap() template.FuncMap {
	return template.FuncMap{
		"join":          fnJoin,
		"indent":        fnIndent,
		"bullet":        fnBullet,
		"numbered":      fnNumbered,
		"ifdef":         fnIfdef,
		"truncate":      fnTruncate,
		"upper":         fnUpper,
		"lower":         fnLower,
		"title":         fnTitle,
		"contains":      fnContains,
		"hasRole":       fnHasRole,
		"hasPermission": fnHasPermission,
		"tokenCount":    fnTokenCount,
	}
}

// fnJoin wraps strings.Join with nil-safe handling.
func fnJoin(items []string, sep string) string {
	if len(items) == 0 {
		return ""
	}
	return strings.Join(items, sep)
}

// fnIndent prepends n spaces to each line in text.
func fnIndent(n int, text string) string {
	if text == "" || n <= 0 {
		return text
	}
	prefix := strings.Repeat(" ", n)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}

// fnBullet formats a string slice as a markdown bullet list.
func fnBullet(items []string) string {
	if len(items) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, item := range items {
		sb.WriteString("- ")
		sb.WriteString(item)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// fnNumbered formats a string slice as a numbered markdown list.
func fnNumbered(items []string) string {
	if len(items) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, item := range items {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, item)
	}
	return sb.String()
}

// fnIfdef returns true if the string is non-empty.
func fnIfdef(s string) bool {
	return s != ""
}

// fnTruncate truncates text to n characters, appending "..." if truncated.
func fnTruncate(n int, text string) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= n {
		return text
	}
	if n <= 3 {
		return string(runes[:n])
	}
	return string(runes[:n-3]) + "..."
}

// fnUpper converts text to uppercase.
func fnUpper(s string) string {
	return strings.ToUpper(s)
}

// fnLower converts text to lowercase.
func fnLower(s string) string {
	return strings.ToLower(s)
}

// fnTitle converts text to title case (first letter of each word uppercase).
// Uses a simple manual implementation to avoid x/text dependency.
func fnTitle(s string) string {
	if s == "" {
		return ""
	}
	words := strings.Fields(s)
	for i, w := range words {
		if w == "" {
			continue
		}
		runes := []rune(w)
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

// fnContains wraps strings.Contains.
func fnContains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// fnHasRole checks if the given RoleName matches the target string.
func fnHasRole(role RoleName, target string) bool {
	return string(role) == target
}

// fnHasPermission checks if the permission string matches the target.
func fnHasPermission(perm, target string) bool {
	return perm == target
}

// fnTokenCount estimates the token count using the len/4 heuristic.
func fnTokenCount(text string) int {
	return len(text) / 4
}
