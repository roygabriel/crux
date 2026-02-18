package phase

// defaultConstraints are always injected into every prompt.
var defaultConstraints = []string{
	"Do not modify files outside the scope of this prompt.",
	"Update work notes after completing the task.",
	"Run all verification commands before considering the task complete.",
}

// DefaultConstraints returns a copy of the standard constraints.
func DefaultConstraints() []string {
	return append([]string(nil), defaultConstraints...)
}

// PermissionDescription returns a one-line description of a permission tier.
// Returns empty string for unknown tiers.
func PermissionDescription(perm string) string {
	return permissionDescriptions[perm]
}

var permissionDescriptions = map[string]string{
	"readonly":   "Read-only access. You may not write files or execute commands. Your output is advisory only.",
	"standard":   "Scoped write access. You may write files within the task scope and run allowlisted verification commands.",
	"elevated":   "Project-wide write access. You may write anywhere under the project root and run most commands.",
	"autonomous": "Full project access. You may write anywhere, run all non-destructive commands, and access the network.",
}
