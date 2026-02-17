package types

// Permission defines the security tier granted to an agent.
type Permission string

const (
	// PermReadonly allows no writes, no shell, no network.
	PermReadonly Permission = "readonly"
	// PermStandard allows scoped file writes and allowlisted commands.
	PermStandard Permission = "standard"
	// PermElevated allows project-root writes, most commands, and localhost network.
	PermElevated Permission = "elevated"
	// PermAutonomous allows project-root writes, all non-destructive commands, and network.
	PermAutonomous Permission = "autonomous"
)

// String returns the string representation of a Permission.
func (p Permission) String() string { return string(p) }

// IsValid reports whether p is a recognized permission tier.
func (p Permission) IsValid() bool {
	switch p {
	case PermReadonly, PermStandard, PermElevated, PermAutonomous:
		return true
	default:
		return false
	}
}
