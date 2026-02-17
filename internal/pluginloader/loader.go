// Package pluginloader registers built-in and generic plugins into a
// plugin registry. It lives in a separate package to avoid import cycles
// between internal/plugin and the plugins/* packages.
package pluginloader

import (
	"fmt"

	"github.com/roygabriel/crux/internal/config"
	"github.com/roygabriel/crux/internal/plugin"
	"github.com/roygabriel/crux/plugins/claude"
	"github.com/roygabriel/crux/plugins/codex"
	"github.com/roygabriel/crux/plugins/gemini"
	"github.com/roygabriel/crux/plugins/generic"
)

// LoadPlugins registers built-in plugins (claude, codex, gemini) and
// any generic plugins defined in config into the given registry.
func LoadPlugins(cfg *config.Config, registry *plugin.Registry) error {
	builtins := map[string]plugin.PluginFactory{
		"claude": func() plugin.AgentPlugin { return claude.New() },
		"codex":  func() plugin.AgentPlugin { return codex.New() },
		"gemini": func() plugin.AgentPlugin { return gemini.New() },
	}

	for name, factory := range builtins {
		if err := registry.Register(name, factory); err != nil {
			return fmt.Errorf("register built-in plugin %q: %w", name, err)
		}
	}

	for name, gcfg := range cfg.GenericPlugins {
		gpCfg := generic.GenericPluginConfig{
			Name:             gcfg.Name,
			Binary:           gcfg.Binary,
			Args:             gcfg.Args,
			ReadyPattern:     gcfg.ReadyPattern,
			BusyPattern:      gcfg.BusyPattern,
			ErrorPattern:     gcfg.ErrorPattern,
			RateLimitPattern: gcfg.RateLimitPattern,
			Capabilities:     gcfg.Capabilities,
		}

		// Validate patterns at load time.
		p, err := generic.New(gpCfg)
		if err != nil {
			return fmt.Errorf("load generic plugin %q: %w", name, err)
		}

		// Capture p in closure for the factory.
		factory := func(plug *generic.Plugin) plugin.PluginFactory {
			return func() plugin.AgentPlugin { return plug }
		}(p)

		if err := registry.Register(name, factory); err != nil {
			return fmt.Errorf("register generic plugin %q: %w", name, err)
		}
	}

	return nil
}
