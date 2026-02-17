package pluginloader_test

import (
	"testing"

	"github.com/roygabriel/crux/internal/config"
	"github.com/roygabriel/crux/internal/plugin"
	"github.com/roygabriel/crux/internal/pluginloader"
)

func TestLoadPluginsBuiltins(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	registry := plugin.NewRegistry()

	if err := pluginloader.LoadPlugins(cfg, registry); err != nil {
		t.Fatalf("LoadPlugins() error: %v", err)
	}

	want := []string{"claude", "codex", "gemini"}
	got := registry.List()

	if len(got) != len(want) {
		t.Fatalf("registry has %d plugins, want %d: %v", len(got), len(want), got)
	}
	for i, name := range want {
		if got[i] != name {
			t.Errorf("registry.List()[%d] = %q, want %q", i, got[i], name)
		}
	}
}

func TestLoadPluginsBuiltinInstances(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	registry := plugin.NewRegistry()

	if err := pluginloader.LoadPlugins(cfg, registry); err != nil {
		t.Fatalf("LoadPlugins() error: %v", err)
	}

	tests := []struct {
		name     string
		wantName string
	}{
		{"claude", "claude"},
		{"codex", "codex"},
		{"gemini", "gemini"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p, err := registry.Get(tt.name)
			if err != nil {
				t.Fatalf("Get(%q) error: %v", tt.name, err)
			}
			if p.Name() != tt.wantName {
				t.Errorf("Name() = %q, want %q", p.Name(), tt.wantName)
			}
		})
	}
}

func TestLoadPluginsWithGeneric(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.GenericPlugins = map[string]config.GenericPluginConfig{
		"my-agent": {
			Name:         "my-agent",
			Binary:       "my-agent-bin",
			ReadyPattern: `^>$`,
			BusyPattern:  `(?i:working\.\.\.)`,
			ErrorPattern: `(?mi)^\s*error:\s*(.+)$`,
		},
	}

	registry := plugin.NewRegistry()

	if err := pluginloader.LoadPlugins(cfg, registry); err != nil {
		t.Fatalf("LoadPlugins() error: %v", err)
	}

	got := registry.List()
	// Should have 4: claude, codex, gemini, my-agent.
	if len(got) != 4 {
		t.Fatalf("registry has %d plugins, want 4: %v", len(got), got)
	}

	p, err := registry.Get("my-agent")
	if err != nil {
		t.Fatalf("Get(my-agent) error: %v", err)
	}
	if p.Name() != "my-agent" {
		t.Errorf("Name() = %q, want %q", p.Name(), "my-agent")
	}
}

func TestLoadPluginsInvalidGenericPattern(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.GenericPlugins = map[string]config.GenericPluginConfig{
		"bad-agent": {
			Name:         "bad-agent",
			Binary:       "bad-bin",
			ReadyPattern: "[invalid",
		},
	}

	registry := plugin.NewRegistry()

	err := pluginloader.LoadPlugins(cfg, registry)
	if err == nil {
		t.Fatal("LoadPlugins() expected error for invalid pattern, got nil")
	}
}

func TestLoadPluginsDuplicateNameError(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	// Try to register a generic plugin with a name that collides with a built-in.
	cfg.GenericPlugins = map[string]config.GenericPluginConfig{
		"claude": {
			Name:         "claude",
			Binary:       "claude-custom",
			ReadyPattern: `^>$`,
			BusyPattern:  `working`,
			ErrorPattern: `error`,
		},
	}

	registry := plugin.NewRegistry()

	err := pluginloader.LoadPlugins(cfg, registry)
	if err == nil {
		t.Fatal("LoadPlugins() expected error for duplicate name, got nil")
	}
}
