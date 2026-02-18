package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/roygabriel/crux/internal/config"
	"github.com/roygabriel/crux/internal/instruct/prefs"
	"github.com/roygabriel/crux/internal/planner"
	"github.com/spf13/cobra"
)

var (
	fromDescription string
	validatePlan    bool
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Start an interactive planning session",
	Long:  "Launch the planning agent TUI to interactively design phase docs for your project.",
	RunE:  runPlan,
}

func init() {
	planCmd.Flags().StringVar(&fromDescription, "from-description", "", "Seed the conversation with an initial project description")
	planCmd.Flags().BoolVar(&validatePlan, "validate", false, "Validate existing phase docs without interactive session")
}

func runPlan(cmd *cobra.Command, args []string) error {
	log := setupLogger()

	if validatePlan {
		return runPlanValidation()
	}

	// Resolve API key.
	apiKey := resolveAPIKey()
	if apiKey == "" {
		fmt.Println("No Anthropic API key found.")
		fmt.Println("Set CRUX_ANTHROPIC_API_KEY or ANTHROPIC_API_KEY to use the planning agent.")
		return nil
	}

	// Load config.
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Resolve project root.
	projectRoot, err := filepath.Abs(cfg.Project.Root)
	if err != nil {
		return fmt.Errorf("resolve project root: %w", err)
	}

	// Build project context from config.
	projectCtx := planner.ProjectContext{
		Name:     cfg.Project.Name,
		RepoRoot: projectRoot,
	}

	// Load preferences if available.
	cruxDir := filepath.Dir(cfgFile)
	store := prefs.NewStore(cruxDir, log)
	var preferences *prefs.Preferences
	if store.Exists() {
		p, loadErr := store.Load()
		if loadErr != nil {
			log.Warn("failed to load preferences", "error", loadErr)
		} else {
			preferences = p
		}
	}

	// Initialize agent.
	agent, err := planner.NewAgent(apiKey, "", projectCtx, preferences, log, cfg.Planner.MaxTokens)
	if err != nil {
		return fmt.Errorf("create planning agent: %w", err)
	}

	// Register tools.
	planner.RegisterTools(agent, projectRoot)

	// Create TUI model.
	model := planner.NewTUIModel(agent, projectRoot)

	// Seed conversation if --from-description is set.
	if fromDescription != "" {
		model.SetInitialMessage(fromDescription)
	}

	// Launch TUI.
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("planner tui: %w", err)
	}

	return nil
}

// runPlanValidation validates existing phase docs in docs/phases/.
func runPlanValidation() error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	projectRoot, err := filepath.Abs(cfg.Project.Root)
	if err != nil {
		return fmt.Errorf("resolve project root: %w", err)
	}

	phasesDir := filepath.Join(projectRoot, "docs", "phases")
	entries, err := os.ReadDir(phasesDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No docs/phases/ directory found.")
			return nil
		}
		return fmt.Errorf("reading phases directory: %w", err)
	}

	totalFiles := 0
	totalIssues := 0

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".md") {
			continue
		}
		if !strings.HasPrefix(name, "PHASE") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(phasesDir, name))
		if err != nil {
			return fmt.Errorf("reading %s: %w", name, err)
		}
		content := string(data)
		totalFiles++

		var issues []string
		if strings.HasSuffix(name, "-PROMPT.md") {
			issues = planner.ValidatePromptContent(content)
		} else {
			issues = planner.ValidateSpecContent(content)
		}

		if len(issues) == 0 {
			fmt.Printf("\u2713 %s: valid\n", name)
		} else {
			totalIssues += len(issues)
			fmt.Printf("\u2717 %s: %d issue(s)\n", name, len(issues))
			for _, issue := range issues {
				fmt.Printf("  - %s\n", issue)
			}
		}
	}

	if totalFiles == 0 {
		fmt.Println("No phase docs found in docs/phases/.")
		return nil
	}

	fmt.Printf("\nValidated %d file(s), %d issue(s) found.\n", totalFiles, totalIssues)
	return nil
}

// resolveAPIKey checks environment variables for an Anthropic API key.
func resolveAPIKey() string {
	if key := os.Getenv("CRUX_ANTHROPIC_API_KEY"); key != "" {
		return key
	}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return key
	}
	return ""
}
