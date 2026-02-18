package wizard

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/roygabriel/crux/internal/config"
)

// collectAgents runs a loop that lets the user add agents one at a time.
// It returns the agent map for the config.
func collectAgents() (map[string]config.AgentConfig, error) {
	agents := make(map[string]config.AgentConfig)

	for {
		agent, err := collectOneAgent(len(agents) + 1)
		if err != nil {
			return nil, fmt.Errorf("collect agent: %w", err)
		}

		agents[agent.name] = config.AgentConfig{
			Plugin:     agent.plugin,
			Role:       agent.role,
			Permission: agent.permission,
		}

		var addAnother bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Add another agent?").
					Value(&addAnother),
			),
		)
		if err := form.Run(); err != nil {
			return nil, fmt.Errorf("confirm agent: %w", err)
		}

		if !addAnother {
			break
		}
	}

	return agents, nil
}

// agentInput holds the raw input from a single agent form.
type agentInput struct {
	name       string
	plugin     string
	role       string
	permission string
}

// collectOneAgent presents a form to configure a single agent.
func collectOneAgent(num int) (*agentInput, error) {
	input := &agentInput{
		name:       fmt.Sprintf("agent-%d", num),
		plugin:     "claude",
		role:       "engineer",
		permission: "standard",
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Agent name").
				Description(fmt.Sprintf("Unique identifier for agent #%d", num)).
				Value(&input.name),
			huh.NewSelect[string]().
				Title("Plugin").
				Description("Which AI CLI tool to use").
				Options(toPluginHuhOptions()...).
				Value(&input.plugin),
			huh.NewSelect[string]().
				Title("Role").
				Description("Functional role in the orchestration").
				Options(toRoleHuhOptions()...).
				Value(&input.role),
			huh.NewSelect[string]().
				Title("Permission").
				Description("Security permission tier").
				Options(toPermissionHuhOptions()...).
				Value(&input.permission),
		),
	)

	if err := form.Run(); err != nil {
		return nil, err
	}

	return input, nil
}
