package security

import (
	"log/slog"
	"strings"

	"github.com/roygabriel/crux/pkg/types"
)

// ActionType classifies the kind of action being checked.
type ActionType string

const (
	// ActionFileRead is a file read operation.
	ActionFileRead ActionType = "file_read"
	// ActionFileWrite is a file write operation.
	ActionFileWrite ActionType = "file_write"
	// ActionShellExec is a shell command execution.
	ActionShellExec ActionType = "shell_exec"
	// ActionNetworkAccess is a network access attempt.
	ActionNetworkAccess ActionType = "network_access"
	// ActionGitPush is a git push operation.
	ActionGitPush ActionType = "git_push"
	// ActionGitCommit is a git commit operation.
	ActionGitCommit ActionType = "git_commit"
	// ActionMessageSend is an internal orchestrator-to-agent message dispatch.
	ActionMessageSend ActionType = "message_send"
)

// PermissionResult captures the outcome of a permission check.
type PermissionResult struct {
	Allowed    bool             `json:"allowed"`
	Reason     string           `json:"reason"`
	Action     ActionType       `json:"action"`
	Target     string           `json:"target"`
	Permission types.Permission `json:"permission"`
}

// Enforcer checks agent actions against their permission tier and the
// filesystem sandbox.
type Enforcer struct {
	sandbox *Sandbox
	logger  *slog.Logger
}

// NewEnforcer creates an Enforcer backed by the given Sandbox.
func NewEnforcer(sandbox *Sandbox, logger *slog.Logger) *Enforcer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Enforcer{
		sandbox: sandbox,
		logger:  logger,
	}
}

// Check evaluates whether perm allows action on target.
func (e *Enforcer) Check(perm types.Permission, action ActionType, target string) PermissionResult {
	result := PermissionResult{
		Action:     action,
		Target:     target,
		Permission: perm,
	}

	switch action {
	case ActionFileRead:
		return e.checkFileRead(perm, target, result)
	case ActionFileWrite:
		return e.checkFileWrite(perm, target, result)
	case ActionShellExec:
		return e.ShellCommandAllowed(perm, target)
	case ActionNetworkAccess:
		return e.checkNetwork(perm, target, result)
	case ActionGitPush:
		return e.checkGitPush(perm, target, result)
	case ActionGitCommit:
		return e.checkGitCommit(perm, result)
	case ActionMessageSend:
		return e.checkMessageSend(result)
	default:
		result.Reason = "unknown action type"
		return result
	}
}

func (e *Enforcer) checkMessageSend(result PermissionResult) PermissionResult {
	// Messaging is an internal control-plane action (not filesystem/shell/network).
	// It must be allowed so orchestrator dispatch can reach agent panes.
	result.Allowed = true
	result.Reason = "message dispatch allowed"
	return result
}

func (e *Enforcer) checkFileRead(perm types.Permission, target string, result PermissionResult) PermissionResult {
	if err := e.sandbox.Check(target, OpRead); err != nil {
		result.Reason = "sandbox: " + err.Error()
		return result
	}
	result.Allowed = true
	result.Reason = "sandbox check passed"
	return result
}

func (e *Enforcer) checkFileWrite(perm types.Permission, target string, result PermissionResult) PermissionResult {
	if perm == types.PermReadonly {
		result.Reason = "readonly permission denies file writes"
		return result
	}

	op := OpWrite
	if err := e.sandbox.Check(target, op); err != nil {
		result.Reason = "sandbox: " + err.Error()
		return result
	}

	result.Allowed = true
	result.Reason = "file write allowed"
	return result
}

func (e *Enforcer) checkNetwork(perm types.Permission, target string, result PermissionResult) PermissionResult {
	switch perm {
	case types.PermReadonly, types.PermStandard:
		result.Reason = perm.String() + " permission denies network access"
		return result
	case types.PermElevated:
		if isLocalhost(target) {
			result.Allowed = true
			result.Reason = "elevated allows localhost network"
			return result
		}
		result.Reason = "elevated permission denies external network access"
		return result
	case types.PermAutonomous:
		result.Allowed = true
		result.Reason = "autonomous allows network access"
		return result
	default:
		result.Reason = "unknown permission"
		return result
	}
}

func (e *Enforcer) checkGitPush(perm types.Permission, target string, result PermissionResult) PermissionResult {
	switch perm {
	case types.PermReadonly, types.PermStandard:
		result.Reason = perm.String() + " permission denies git push"
		return result
	case types.PermElevated, types.PermAutonomous:
		if isProtectedBranch(target) {
			result.Reason = "push to protected branch " + target + " denied"
			return result
		}
		result.Allowed = true
		result.Reason = "feature branch push allowed"
		return result
	default:
		result.Reason = "unknown permission"
		return result
	}
}

func (e *Enforcer) checkGitCommit(perm types.Permission, result PermissionResult) PermissionResult {
	switch perm {
	case types.PermReadonly, types.PermStandard:
		result.Reason = perm.String() + " permission denies git commit"
		return result
	case types.PermElevated, types.PermAutonomous:
		result.Allowed = true
		result.Reason = "git commit allowed"
		return result
	default:
		result.Reason = "unknown permission"
		return result
	}
}

// ShellCommandAllowed checks whether perm allows executing command.
func (e *Enforcer) ShellCommandAllowed(perm types.Permission, command string) PermissionResult {
	result := PermissionResult{
		Action:     ActionShellExec,
		Target:     command,
		Permission: perm,
	}

	fields := strings.Fields(command)
	if len(fields) == 0 {
		result.Reason = "empty command denied"
		return result
	}

	first := fields[0]
	twoToken := ""
	if len(fields) >= 2 {
		twoToken = first + " " + fields[1]
	}

	switch perm {
	case types.PermReadonly:
		result.Reason = "readonly permission denies shell execution"
		return result

	case types.PermStandard:
		return e.checkStandardShell(first, twoToken, result)

	case types.PermElevated:
		return e.checkElevatedShell(first, twoToken, fields, result)

	case types.PermAutonomous:
		return e.checkAutonomousShell(first, twoToken, fields, result)

	default:
		result.Reason = "unknown permission"
		return result
	}
}

// Standard allowlist: first-token commands allowed for standard tier.
var standardAllowlist = map[string]bool{
	"go":     true,
	"make":   true,
	"npm":    true,
	"node":   true,
	"python": true,
	"pytest": true,
	"cargo":  true,
	"rustc":  true,
	"cat":    true,
	"head":   true,
	"tail":   true,
	"grep":   true,
	"rg":     true,
	"find":   true,
	"ls":     true,
	"wc":     true,
	"diff":   true,
	"git":    true,
}

// Standard git subcommand allowlist (two-token).
var standardGitAllowlist = map[string]bool{
	"git status":   true,
	"git diff":     true,
	"git log":      true,
	"git add":      true,
	"git commit":   true,
	"git branch":   true,
	"git checkout": true,
	"git switch":   true,
	"git stash":    true,
	"git show":     true,
}

// Elevated/Autonomous denylist: first-token commands denied for elevated+autonomous.
var elevatedDenylist = map[string]bool{
	"sudo":     true,
	"shutdown": true,
	"reboot":   true,
	"mkfs":     true,
	"dd":       true,
	"fdisk":    true,
	"rm":       true,
}

// Autonomous denylist: same as elevated but without "rm".
var autonomousDenylist = map[string]bool{
	"sudo":     true,
	"shutdown": true,
	"reboot":   true,
	"mkfs":     true,
	"dd":       true,
	"fdisk":    true,
}

// Git subcommand denylist patterns (for elevated+autonomous).
var gitSubcommandDenylist = []string{
	"git push --force",
	"git reset --hard",
	"git clean",
}

func (e *Enforcer) checkStandardShell(first, twoToken string, result PermissionResult) PermissionResult {
	if !standardAllowlist[first] {
		result.Reason = "command " + first + " not in standard allowlist"
		return result
	}

	// Git commands need further subcommand checking.
	if first == "git" {
		if twoToken == "" || !standardGitAllowlist[twoToken] {
			result.Reason = "git subcommand not in standard allowlist"
			return result
		}
	}

	result.Allowed = true
	result.Reason = "command allowed by standard allowlist"
	return result
}

func (e *Enforcer) checkElevatedShell(first, twoToken string, fields []string, result PermissionResult) PermissionResult {
	if elevatedDenylist[first] {
		result.Reason = "command " + first + " denied for elevated"
		return result
	}

	if first == "git" && matchGitDenylist(fields) {
		result.Reason = "git subcommand denied for elevated"
		return result
	}

	result.Allowed = true
	result.Reason = "command allowed by elevated denylist"
	return result
}

func (e *Enforcer) checkAutonomousShell(first, twoToken string, fields []string, result PermissionResult) PermissionResult {
	if autonomousDenylist[first] {
		result.Reason = "command " + first + " denied for autonomous"
		return result
	}

	if first == "git" && matchGitDenylist(fields) {
		result.Reason = "git subcommand denied for autonomous"
		return result
	}

	result.Allowed = true
	result.Reason = "command allowed by autonomous denylist"
	return result
}

// matchGitDenylist checks if a git command matches any denied subcommand pattern.
func matchGitDenylist(fields []string) bool {
	cmd := strings.Join(fields, " ")
	for _, pattern := range gitSubcommandDenylist {
		if strings.HasPrefix(cmd, pattern) {
			return true
		}
	}
	return false
}

// isLocalhost checks if target refers to a local address.
func isLocalhost(target string) bool {
	lower := strings.ToLower(target)
	return lower == "localhost" ||
		strings.HasPrefix(lower, "localhost:") ||
		lower == "127.0.0.1" ||
		strings.HasPrefix(lower, "127.0.0.1:") ||
		lower == "::1" ||
		strings.HasPrefix(lower, "[::1]")
}

// isProtectedBranch checks if a branch name is protected.
func isProtectedBranch(branch string) bool {
	protected := map[string]bool{
		"main":    true,
		"master":  true,
		"develop": true,
	}
	return protected[branch]
}
