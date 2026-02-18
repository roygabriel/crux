package wizard

import (
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/roygabriel/crux/internal/config"
)

// Result holds the output of the wizard flow.
type Result struct {
	// Config is the assembled configuration.
	Config *config.Config
	// SeedExample indicates the user wants to seed an example project.
	SeedExample bool
}

// Run executes the multi-step interactive wizard and returns the
// assembled configuration. It displays forms for project setup, agent
// configuration, phase settings, optional memory settings, and a final
// confirmation with a YAML preview.
func Run() (*Result, error) {
	cfg := config.DefaultConfig()

	// Phase 1: Project setup.
	dirName := filepath.Base(".")
	if abs, err := filepath.Abs("."); err == nil {
		dirName = filepath.Base(abs)
	}
	cfg.Project.Name = dirName

	projectForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Project name").
				Description("Human-readable name for this project").
				Value(&cfg.Project.Name),
			huh.NewInput().
				Title("State directory").
				Description("Directory for crux state files").
				Value(&cfg.Project.StateDir),
		).Title("Project Setup"),
	)
	if err := projectForm.Run(); err != nil {
		return nil, fmt.Errorf("project setup: %w", err)
	}

	// Update dependent paths based on state dir.
	cfg.Memory.SQLitePath = filepath.Join(cfg.Project.StateDir, "memory.db")
	cfg.Memory.VectorDir = filepath.Join(cfg.Project.StateDir, "vectors")
	cfg.Security.AuditLog = filepath.Join(cfg.Project.StateDir, "audit.log")

	// Phase 2: Agent builder.
	agents, err := collectAgents()
	if err != nil {
		return nil, fmt.Errorf("agent builder: %w", err)
	}
	cfg.Agents = agents

	// Phase 3: Phase config.
	var seedExample bool
	phaseForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Phase spec directory").
				Description("Directory containing phase specification files").
				Value(&cfg.Phases.SpecDir),
			huh.NewConfirm().
				Title("Seed with HTTP API example?").
				Description("Populates docs/phases/ with a starter project").
				Value(&seedExample),
		).Title("Phase Configuration"),
	)
	if err := phaseForm.Run(); err != nil {
		return nil, fmt.Errorf("phase config: %w", err)
	}

	// Phase 4: Optional memory config.
	var advancedMemory bool
	advForm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Configure advanced memory settings?").
				Description("SQLite path, vector directory (defaults are usually fine)").
				Value(&advancedMemory),
		),
	)
	if err := advForm.Run(); err != nil {
		return nil, fmt.Errorf("memory config prompt: %w", err)
	}

	if advancedMemory {
		memForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("SQLite database path").
					Value(&cfg.Memory.SQLitePath),
				huh.NewInput().
					Title("Vector index directory").
					Value(&cfg.Memory.VectorDir),
			).Title("Memory Configuration"),
		)
		if err := memForm.Run(); err != nil {
			return nil, fmt.Errorf("memory config: %w", err)
		}
	}

	// Phase 5: Summary and confirmation.
	summary := RenderSummary(cfg)
	fmt.Println("\n" + summary)

	var confirmed bool
	confirmForm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Write this configuration?").
				Affirmative("Yes, create config").
				Negative("No, cancel").
				Value(&confirmed),
		),
	)
	if err := confirmForm.Run(); err != nil {
		return nil, fmt.Errorf("confirmation: %w", err)
	}

	if !confirmed {
		return nil, fmt.Errorf("wizard cancelled by user")
	}

	return &Result{
		Config:      cfg,
		SeedExample: seedExample,
	}, nil
}
