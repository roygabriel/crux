// Package config handles YAML configuration loading, environment variable
// overrides, and validation for the crux orchestrator.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the top-level orchestrator configuration.
type Config struct {
	// Project holds project-level identification and paths.
	Project ProjectConfig `yaml:"project" json:"project"`
	// Agents maps agent names to their configuration.
	Agents map[string]AgentConfig `yaml:"agents" json:"agents"`
	// Memory configures the storage backends.
	Memory MemoryConfig `yaml:"memory" json:"memory"`
	// Phases configures the phase engine.
	Phases PhaseConfig `yaml:"phases" json:"phases"`
	// Security configures sandboxing, auditing, and rate limiting.
	Security SecurityConfig `yaml:"security" json:"security"`
	// GenericPlugins maps custom plugin names to their regex-based configuration.
	GenericPlugins map[string]GenericPluginConfig `yaml:"generic_plugins" json:"generic_plugins,omitempty"`
}

// GenericPluginConfig holds user-supplied configuration for a generic
// plugin adapter with regex-based detection patterns.
type GenericPluginConfig struct {
	// Name is the display name for this plugin.
	Name string `yaml:"name" json:"name"`
	// Binary is the executable to launch.
	Binary string `yaml:"binary" json:"binary"`
	// Args are default CLI arguments.
	Args []string `yaml:"args" json:"args,omitempty"`
	// ReadyPattern is a regex matched against the last non-empty line.
	ReadyPattern string `yaml:"ready_pattern" json:"ready_pattern"`
	// BusyPattern is a regex matched against the tail of pane content.
	BusyPattern string `yaml:"busy_pattern" json:"busy_pattern"`
	// ErrorPattern is a regex with a capture group for the error message.
	ErrorPattern string `yaml:"error_pattern" json:"error_pattern"`
	// RateLimitPattern is a regex for rate-limit detection.
	RateLimitPattern string `yaml:"rate_limit_pattern" json:"rate_limit_pattern,omitempty"`
	// Capabilities lists the capability strings this plugin supports.
	Capabilities []string `yaml:"capabilities" json:"capabilities,omitempty"`
}

// ProjectConfig holds project-level identification and paths.
type ProjectConfig struct {
	// Name is the human-readable project name.
	Name string `yaml:"name" json:"name"`
	// Root is the project root directory.
	Root string `yaml:"root" json:"root"`
	// StateDir is the directory for crux state files.
	StateDir string `yaml:"state_dir" json:"state_dir"`
}

// AgentConfig defines the settings for a single agent instance.
type AgentConfig struct {
	// Plugin is the adapter to use (claude, codex, gemini, generic).
	Plugin string `yaml:"plugin" json:"plugin"`
	// Role is the agent's functional role (orchestrator, project-manager, engineer).
	Role string `yaml:"role" json:"role"`
	// Permission is the security tier (readonly, standard, elevated, autonomous).
	Permission string `yaml:"permission" json:"permission"`
	// Model is the optional model identifier passed to the plugin.
	Model string `yaml:"model" json:"model,omitempty"`
}

// MemoryConfig configures the storage backends.
type MemoryConfig struct {
	// SQLitePath is the path to the SQLite database file.
	SQLitePath string `yaml:"sqlite_path" json:"sqlite_path"`
	// VectorDir is the directory for vector index persistence.
	VectorDir string `yaml:"vector_dir" json:"vector_dir"`
}

// PhaseConfig configures the phase engine.
type PhaseConfig struct {
	// SpecDir is the directory containing phase specification files.
	SpecDir string `yaml:"spec_dir" json:"spec_dir"`
}

// SecurityConfig configures sandboxing, auditing, and rate limiting.
type SecurityConfig struct {
	// AuditLog is the path to the structured audit log file.
	AuditLog string `yaml:"audit_log" json:"audit_log"`
	// MaxCmdsPerMin is the maximum commands an agent may execute per minute.
	MaxCmdsPerMin int `yaml:"max_cmds_per_min" json:"max_cmds_per_min"`
	// MaxFilesPerSession is the maximum files an agent may modify per session.
	MaxFilesPerSession int `yaml:"max_files_per_session" json:"max_files_per_session"`
	// AllowedPaths restricts file operations to these path prefixes.
	AllowedPaths []string `yaml:"allowed_paths" json:"allowed_paths,omitempty"`
}

// Load reads a YAML configuration file, applies environment variable
// overrides with the CRUX_ prefix, and validates the result.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	applyEnvOverrides(cfg)

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}

// Validate checks that the configuration has all required fields and
// that enum-like values are within their allowed sets.
func (c *Config) Validate() error {
	if c.Project.Name == "" {
		return fmt.Errorf("project.name is required")
	}
	if c.Project.Root == "" {
		return fmt.Errorf("project.root is required")
	}
	if c.Project.StateDir == "" {
		return fmt.Errorf("project.state_dir is required")
	}
	if c.Memory.SQLitePath == "" {
		return fmt.Errorf("memory.sqlite_path is required")
	}
	if c.Memory.VectorDir == "" {
		return fmt.Errorf("memory.vector_dir is required")
	}

	validPlugins := map[string]bool{
		"claude": true, "codex": true, "gemini": true, "generic": true,
	}
	for name := range c.GenericPlugins {
		validPlugins[name] = true
	}
	validRoles := map[string]bool{
		"orchestrator": true, "project-manager": true, "engineer": true,
	}
	validPerms := map[string]bool{
		"readonly": true, "standard": true, "elevated": true, "autonomous": true,
	}

	for name, agent := range c.Agents {
		if agent.Plugin == "" {
			return fmt.Errorf("agents.%s.plugin is required", name)
		}
		if !validPlugins[agent.Plugin] {
			return fmt.Errorf("agents.%s.plugin: unknown plugin %q", name, agent.Plugin)
		}
		if agent.Role == "" {
			return fmt.Errorf("agents.%s.role is required", name)
		}
		if !validRoles[agent.Role] {
			return fmt.Errorf("agents.%s.role: unknown role %q", name, agent.Role)
		}
		if agent.Permission == "" {
			return fmt.Errorf("agents.%s.permission is required", name)
		}
		if !validPerms[agent.Permission] {
			return fmt.Errorf("agents.%s.permission: unknown permission %q", name, agent.Permission)
		}
	}

	if c.Security.MaxCmdsPerMin < 0 {
		return fmt.Errorf("security.max_cmds_per_min must be non-negative")
	}
	if c.Security.MaxFilesPerSession < 0 {
		return fmt.Errorf("security.max_files_per_session must be non-negative")
	}

	return nil
}

// applyEnvOverrides maps CRUX_ prefixed environment variables to config
// fields. The mapping flattens nested YAML keys with underscores:
// CRUX_PROJECT_NAME -> project.name, CRUX_MEMORY_SQLITE_PATH -> memory.sqlite_path.
func applyEnvOverrides(cfg *Config) {
	overrides := map[string]*string{
		"CRUX_PROJECT_NAME":      &cfg.Project.Name,
		"CRUX_PROJECT_ROOT":      &cfg.Project.Root,
		"CRUX_PROJECT_STATE_DIR": &cfg.Project.StateDir,
		"CRUX_MEMORY_SQLITE_PATH": &cfg.Memory.SQLitePath,
		"CRUX_MEMORY_VECTOR_DIR":  &cfg.Memory.VectorDir,
		"CRUX_PHASES_SPEC_DIR":    &cfg.Phases.SpecDir,
		"CRUX_SECURITY_AUDIT_LOG": &cfg.Security.AuditLog,
	}

	for env, field := range overrides {
		if v, ok := os.LookupEnv(env); ok {
			*field = v
		}
	}

	intOverrides := map[string]*int{
		"CRUX_SECURITY_MAX_CMDS_PER_MIN":    &cfg.Security.MaxCmdsPerMin,
		"CRUX_SECURITY_MAX_FILES_PER_SESSION": &cfg.Security.MaxFilesPerSession,
	}

	for env, field := range intOverrides {
		if v, ok := os.LookupEnv(env); ok {
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				*field = n
			}
		}
	}
}
