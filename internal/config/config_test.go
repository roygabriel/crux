package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadValidYAML(t *testing.T) {
	t.Parallel()

	yaml := `
project:
  name: "test-project"
  root: "/tmp/test"
  state_dir: ".crux"

agents:
  eng-1:
    plugin: claude
    role: engineer
    permission: standard

memory:
  sqlite_path: ".crux/memory.db"
  vector_dir: ".crux/vectors"
  embedding_provider: "ollama"
  embedding_model: "all-minilm"

phases:
  spec_dir: "docs/phases"

security:
  audit_log: ".crux/audit.log"
  max_cmds_per_min: 30
  max_files_per_session: 50
`
	path := writeTempYAML(t, yaml)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Project.Name != "test-project" {
		t.Errorf("Project.Name = %q, want %q", cfg.Project.Name, "test-project")
	}
	if cfg.Project.Root != "/tmp/test" {
		t.Errorf("Project.Root = %q, want %q", cfg.Project.Root, "/tmp/test")
	}
	if cfg.Agents["eng-1"].Plugin != "claude" {
		t.Errorf("Agents[eng-1].Plugin = %q, want %q", cfg.Agents["eng-1"].Plugin, "claude")
	}
	if cfg.Security.MaxCmdsPerMin != 30 {
		t.Errorf("Security.MaxCmdsPerMin = %d, want %d", cfg.Security.MaxCmdsPerMin, 30)
	}
	if cfg.Memory.EmbeddingProvider != "ollama" {
		t.Errorf("Memory.EmbeddingProvider = %q, want %q", cfg.Memory.EmbeddingProvider, "ollama")
	}
	if cfg.Memory.EmbeddingModel != "all-minilm" {
		t.Errorf("Memory.EmbeddingModel = %q, want %q", cfg.Memory.EmbeddingModel, "all-minilm")
	}
}

func TestLoadEnvOverrides(t *testing.T) {

	yaml := `
project:
  name: "original"
  root: "."
  state_dir: ".crux"

memory:
  sqlite_path: ".crux/memory.db"
  vector_dir: ".crux/vectors"
`
	path := writeTempYAML(t, yaml)

	t.Setenv("CRUX_PROJECT_NAME", "overridden")
	t.Setenv("CRUX_MEMORY_SQLITE_PATH", "/custom/path.db")
	t.Setenv("CRUX_SECURITY_MAX_CMDS_PER_MIN", "120")
	t.Setenv("CRUX_MEMORY_EMBEDDING_PROVIDER", "ollama")
	t.Setenv("CRUX_MEMORY_EMBEDDING_MODEL", "custom-model")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Project.Name != "overridden" {
		t.Errorf("Project.Name = %q, want %q", cfg.Project.Name, "overridden")
	}
	if cfg.Memory.SQLitePath != "/custom/path.db" {
		t.Errorf("Memory.SQLitePath = %q, want %q", cfg.Memory.SQLitePath, "/custom/path.db")
	}
	if cfg.Security.MaxCmdsPerMin != 120 {
		t.Errorf("Security.MaxCmdsPerMin = %d, want %d", cfg.Security.MaxCmdsPerMin, 120)
	}
	if cfg.Memory.EmbeddingProvider != "ollama" {
		t.Errorf("Memory.EmbeddingProvider = %q, want %q", cfg.Memory.EmbeddingProvider, "ollama")
	}
	if cfg.Memory.EmbeddingModel != "custom-model" {
		t.Errorf("Memory.EmbeddingModel = %q, want %q", cfg.Memory.EmbeddingModel, "custom-model")
	}
}

func TestLoadPlannerEnvOverride(t *testing.T) {
	yaml := `
project:
  name: "test"
  root: "."
  state_dir: ".crux"

memory:
  sqlite_path: ".crux/memory.db"
  vector_dir: ".crux/vectors"
`
	path := writeTempYAML(t, yaml)

	t.Setenv("CRUX_PLANNER_MAX_TOKENS", "32768")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Planner.MaxTokens != 32768 {
		t.Errorf("Planner.MaxTokens = %d, want %d", cfg.Planner.MaxTokens, 32768)
	}
}

func TestLoadPlannerYAML(t *testing.T) {
	yaml := `
project:
  name: "test"
  root: "."
  state_dir: ".crux"

memory:
  sqlite_path: ".crux/memory.db"
  vector_dir: ".crux/vectors"

planner:
  max_tokens: 24000
`
	path := writeTempYAML(t, yaml)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Planner.MaxTokens != 24000 {
		t.Errorf("Planner.MaxTokens = %d, want %d", cfg.Planner.MaxTokens, 24000)
	}
}

func TestLoadPlannerAgentYAML(t *testing.T) {
	yaml := `
project:
  name: "test"
  root: "."
  state_dir: ".crux"

memory:
  sqlite_path: ".crux/memory.db"
  vector_dir: ".crux/vectors"

planner:
  agent: claude
`
	path := writeTempYAML(t, yaml)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Planner.Agent != "claude" {
		t.Errorf("Planner.Agent = %q, want %q", cfg.Planner.Agent, "claude")
	}
}

func TestLoadPlannerAgentEnvOverride(t *testing.T) {
	yaml := `
project:
  name: "test"
  root: "."
  state_dir: ".crux"

memory:
  sqlite_path: ".crux/memory.db"
  vector_dir: ".crux/vectors"
`
	path := writeTempYAML(t, yaml)

	t.Setenv("CRUX_PLANNER_AGENT", "gemini")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Planner.Agent != "gemini" {
		t.Errorf("Planner.Agent = %q, want %q", cfg.Planner.Agent, "gemini")
	}
}

func TestLoadPlannerAgentDefault(t *testing.T) {
	yaml := `
project:
  name: "test"
  root: "."
  state_dir: ".crux"

memory:
  sqlite_path: ".crux/memory.db"
  vector_dir: ".crux/vectors"
`
	path := writeTempYAML(t, yaml)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Planner.Agent != "" {
		t.Errorf("Planner.Agent = %q, want empty (default)", cfg.Planner.Agent)
	}
}

func TestValidateMissingRequiredFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Config)
		errMsg string
	}{
		{
			name:   "missing project name",
			mutate: func(c *Config) { c.Project.Name = "" },
			errMsg: "project.name is required",
		},
		{
			name:   "missing project root",
			mutate: func(c *Config) { c.Project.Root = "" },
			errMsg: "project.root is required",
		},
		{
			name:   "missing state dir",
			mutate: func(c *Config) { c.Project.StateDir = "" },
			errMsg: "project.state_dir is required",
		},
		{
			name:   "missing sqlite path",
			mutate: func(c *Config) { c.Memory.SQLitePath = "" },
			errMsg: "memory.sqlite_path is required",
		},
		{
			name:   "missing vector dir",
			mutate: func(c *Config) { c.Memory.VectorDir = "" },
			errMsg: "memory.vector_dir is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := DefaultConfig()
			tt.mutate(cfg)
			err := cfg.Validate()
			if err == nil {
				t.Fatal("Validate() expected error, got nil")
			}
			if err.Error() != tt.errMsg {
				t.Errorf("Validate() error = %q, want %q", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestValidateInvalidAgentFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		agent  AgentConfig
		errSub string
	}{
		{
			name:   "unknown plugin",
			agent:  AgentConfig{Plugin: "bad", Role: "engineer", Permission: "standard"},
			errSub: "unknown plugin",
		},
		{
			name:   "unknown role",
			agent:  AgentConfig{Plugin: "claude", Role: "bad", Permission: "standard"},
			errSub: "unknown role",
		},
		{
			name:   "unknown permission",
			agent:  AgentConfig{Plugin: "claude", Role: "engineer", Permission: "bad"},
			errSub: "unknown permission",
		},
		{
			name:   "missing plugin",
			agent:  AgentConfig{Role: "engineer", Permission: "standard"},
			errSub: "plugin is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := DefaultConfig()
			cfg.Agents["test"] = tt.agent
			err := cfg.Validate()
			if err == nil {
				t.Fatal("Validate() expected error, got nil")
			}
			if !contains(err.Error(), tt.errSub) {
				t.Errorf("Validate() error = %q, want substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestValidateNegativeRateLimits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Config)
		errSub string
	}{
		{
			name:   "negative max cmds per min",
			mutate: func(c *Config) { c.Security.MaxCmdsPerMin = -1 },
			errSub: "max_cmds_per_min must be non-negative",
		},
		{
			name:   "negative max files per session",
			mutate: func(c *Config) { c.Security.MaxFilesPerSession = -1 },
			errSub: "max_files_per_session must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := DefaultConfig()
			tt.mutate(cfg)
			err := cfg.Validate()
			if err == nil {
				t.Fatal("Validate() expected error, got nil")
			}
			if !contains(err.Error(), tt.errSub) {
				t.Errorf("Validate() error = %q, want substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestDefaultConfigIsValid(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("DefaultConfig().Validate() = %v, want nil", err)
	}
}

func TestLoadFileNotFound(t *testing.T) {
	t.Parallel()

	_, err := Load("/nonexistent/path.yaml")
	if err == nil {
		t.Fatal("Load() expected error for missing file, got nil")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	t.Parallel()

	path := writeTempYAML(t, `{invalid: yaml: [`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() expected error for invalid YAML, got nil")
	}
}

func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp YAML: %v", err)
	}
	return path
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
