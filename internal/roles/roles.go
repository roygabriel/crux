// Package roles provides embedded role definitions for agent prompts.
package roles

import (
	"embed"
	"fmt"
	"strings"

	"github.com/roygabriel/crux/internal/instruct"
)

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

// legacyRoleMap maps old role names to their new equivalents.
var legacyRoleMap = map[string]string{
	"engineer": "software-engineer",
	"reviewer": "code-reviewer",
}

// NormalizeRole maps legacy role names to their current equivalents.
// Unknown names pass through unchanged.
func NormalizeRole(name string) string {
	if mapped, ok := legacyRoleMap[name]; ok {
		return mapped
	}
	return name
}

// BuildRoleContext loads the embedded definition for the given role name and
// parses it into a RoleContext. Returns an error if the role is unknown.
func BuildRoleContext(roleName instruct.RoleName) (instruct.RoleContext, error) {
	normalized := NormalizeRole(string(roleName))
	content := Definition(normalized)
	if content == "" {
		return instruct.RoleContext{}, fmt.Errorf("unknown role: %q", roleName)
	}
	return ParseRoleContext(instruct.RoleName(normalized), content), nil
}

// ParseRoleContext parses a role definition markdown string into a RoleContext.
// It extracts sections by splitting on ## headings and parsing bullet lists.
// The title is taken from the first # heading line.
func ParseRoleContext(name instruct.RoleName, content string) instruct.RoleContext {
	rc := instruct.RoleContext{
		Name: name,
	}

	// Extract title from first # heading.
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "## ") {
			rc.Title = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
			break
		}
	}

	// Split content into sections by ## headings.
	sections := splitSections(content)

	for heading, body := range sections {
		switch heading {
		case "Identity":
			rc.Identity = parseParagraph(body)
		case "Responsibilities":
			rc.Responsibilities = parseBullets(body)
		case "Constraints":
			rc.Constraints = parseBullets(body)
		case "Communication":
			rc.Communication = parseBullets(body)
		case "Planning Rules":
			rc.PlanningRules = parseBullets(body)
		case "Review Focus":
			rc.ReviewFocus = parseBullets(body)
		}
	}

	return rc
}

// splitSections splits markdown content into a map of heading → body text.
// It looks for lines starting with "## " as section boundaries.
func splitSections(content string) map[string]string {
	sections := make(map[string]string)
	lines := strings.Split(content, "\n")

	var currentHeading string
	var currentBody []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			// Save previous section if any.
			if currentHeading != "" {
				sections[currentHeading] = strings.Join(currentBody, "\n")
			}
			currentHeading = strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			currentBody = nil
		} else if currentHeading != "" {
			currentBody = append(currentBody, line)
		}
	}

	// Save last section.
	if currentHeading != "" {
		sections[currentHeading] = strings.Join(currentBody, "\n")
	}

	return sections
}

// parseParagraph extracts the paragraph text from a section body,
// trimming leading/trailing whitespace.
func parseParagraph(body string) string {
	return strings.TrimSpace(body)
}

// parseBullets extracts bullet items (lines starting with "- ") from a section body.
// Multi-line bullets that continue with indentation are joined with the preceding bullet.
func parseBullets(body string) []string {
	var bullets []string
	lines := strings.Split(body, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") {
			bullets = append(bullets, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
		}
	}

	return bullets
}
