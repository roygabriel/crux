// Package roles provides embedded role definitions for agent prompts.
package roles

import "embed"

//go:embed definitions/*.md
var definitions embed.FS

// Definition returns the embedded markdown for the given role name.
// Returns empty string for unknown roles.
func Definition(role string) string {
	data, err := definitions.ReadFile("definitions/" + role + ".md")
	if err != nil {
		return ""
	}
	return string(data)
}
