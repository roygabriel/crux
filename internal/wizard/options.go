// Package wizard implements an interactive TUI wizard for crux init
// that guides users through building their config.yaml.
package wizard

import "github.com/charmbracelet/huh"

// pluginOption describes an available agent plugin.
type pluginOption struct {
	Value       string
	Label       string
	Description string
}

// roleOption describes an available agent role.
type roleOption struct {
	Value       string
	Label       string
	Description string
}

// permissionOption describes a security permission tier.
type permissionOption struct {
	Value       string
	Label       string
	Description string
}

var pluginOptions = []pluginOption{
	{"claude", "Claude Code", "Anthropic Claude Code CLI adapter"},
	{"codex", "Codex CLI", "OpenAI Codex CLI adapter"},
	{"gemini", "Gemini CLI", "Google Gemini CLI adapter"},
	{"generic", "Generic", "Configurable regex-based adapter for any CLI tool"},
}

var roleOptions = []roleOption{
	{"engineer", "Engineer", "Executes implementation prompts and writes code"},
	{"reviewer", "Reviewer", "Performs code review and quality checks"},
	{"project-manager", "Project Manager", "Coordinates task assignment and phase progression"},
	{"orchestrator", "Orchestrator", "Top-level coordination and decision-making"},
}

var permissionOptions = []permissionOption{
	{"standard", "Standard", "Scoped file writes and allowlisted commands"},
	{"readonly", "Read-only", "No writes, no shell, no network access"},
	{"elevated", "Elevated", "Project-root writes, most commands, localhost network"},
	{"autonomous", "Autonomous", "Project-root writes, non-destructive commands, full network"},
}

// toPluginHuhOptions converts plugin options to huh select options.
func toPluginHuhOptions() []huh.Option[string] {
	opts := make([]huh.Option[string], len(pluginOptions))
	for i, o := range pluginOptions {
		opts[i] = huh.NewOption(o.Label+" — "+o.Description, o.Value)
	}
	return opts
}

// toRoleHuhOptions converts role options to huh select options.
func toRoleHuhOptions() []huh.Option[string] {
	opts := make([]huh.Option[string], len(roleOptions))
	for i, o := range roleOptions {
		opts[i] = huh.NewOption(o.Label+" — "+o.Description, o.Value)
	}
	return opts
}

// toPermissionHuhOptions converts permission options to huh select options.
func toPermissionHuhOptions() []huh.Option[string] {
	opts := make([]huh.Option[string], len(permissionOptions))
	for i, o := range permissionOptions {
		opts[i] = huh.NewOption(o.Label+" — "+o.Description, o.Value)
	}
	return opts
}
