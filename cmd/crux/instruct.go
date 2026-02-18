package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/roygabriel/crux/internal/config"
	"github.com/roygabriel/crux/internal/instruct"
	"github.com/roygabriel/crux/internal/roles"
	"github.com/spf13/cobra"
)

var (
	instructAgentFlag string
	instructForceFlag bool
	instructCLIFlag   string
	instructBudgetFlag int
)

var instructCmd = &cobra.Command{
	Use:   "instruct",
	Short: "Manage agent instruction files",
	Long:  "Generate, preview, diff, and inspect per-agent instruction files.",
}

var instructGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Render and write instruction files",
	Long:  "Generates instruction files for all agents, or a specific agent with --agent. Use --force to write even if content is unchanged.",
	RunE:  runInstructGenerate,
}

var instructPreviewCmd = &cobra.Command{
	Use:   "preview <role>",
	Short: "Render instructions to stdout without writing",
	Long:  "Renders instruction files for a role and prints them. Useful for reviewing before committing.",
	Args:  cobra.ExactArgs(1),
	RunE:  runInstructPreview,
}

var instructDiffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show diff between disk and rendered output",
	Long:  "Shows a unified diff between the current instruction files on disk and what would be rendered.",
	RunE:  runInstructDiff,
}

var instructStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show generation state for all agents",
	Long:  "Displays the last generation timestamp, token count, hash, and files for each configured agent.",
	RunE:  runInstructStatus,
}

var instructValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate generated instruction files",
	Long:  "Checks instruction files for existence, token budget, critical sections, template syntax leaks, and staleness.",
	RunE:  runInstructValidate,
}

func init() {
	instructGenerateCmd.Flags().StringVar(&instructAgentFlag, "agent", "", "Generate for a specific agent ID")
	instructGenerateCmd.Flags().BoolVar(&instructForceFlag, "force", false, "Write even if content hash is unchanged")

	instructPreviewCmd.Flags().StringVar(&instructCLIFlag, "cli", "claude", "Agent CLI to render for (claude, codex, gemini, copilot)")
	instructPreviewCmd.Flags().IntVar(&instructBudgetFlag, "budget", 0, "Token budget override (0 = use CLI default)")

	instructDiffCmd.Flags().StringVar(&instructAgentFlag, "agent", "", "Diff for a specific agent ID")

	instructValidateCmd.Flags().StringVar(&instructAgentFlag, "agent", "", "Validate a specific agent ID")

	instructCmd.AddCommand(instructGenerateCmd)
	instructCmd.AddCommand(instructPreviewCmd)
	instructCmd.AddCommand(instructDiffCmd)
	instructCmd.AddCommand(instructStatusCmd)
	instructCmd.AddCommand(instructValidateCmd)
}

func runInstructGenerate(cmd *cobra.Command, args []string) error {
	log := setupLogger()

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	dist := buildDistributor(cfg, log)
	ctx := context.Background()

	if instructAgentFlag != "" {
		if _, ok := cfg.Agents[instructAgentFlag]; !ok {
			return fmt.Errorf("unknown agent: %s", instructAgentFlag)
		}
		if instructForceFlag {
			if err := dist.GenerateForAgent(ctx, instructAgentFlag); err != nil {
				return err
			}
		} else {
			changed, err := dist.RegenerateIfStale(ctx, instructAgentFlag)
			if err != nil {
				return err
			}
			if !changed {
				// No prior state means first generation.
				if err := dist.GenerateForAgent(ctx, instructAgentFlag); err != nil {
					return err
				}
			}
		}
		state := dist.State()
		if s, ok := state[instructAgentFlag]; ok {
			printGenerationResult(instructAgentFlag, s)
		}
		return nil
	}

	// Generate for all agents.
	if err := dist.GenerateAll(ctx); err != nil {
		return err
	}

	state := dist.State()
	for _, id := range sortedAgentIDs(cfg) {
		if s, ok := state[id]; ok {
			printGenerationResult(id, s)
		}
	}
	return nil
}

func runInstructPreview(cmd *cobra.Command, args []string) error {
	log := setupLogger()

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	roleName := roles.NormalizeRole(args[0])
	cli := instruct.AgentCLI(instructCLIFlag)

	dist := buildDistributor(cfg, log)
	ctx := context.Background()

	files, result, err := dist.PreviewForRole(ctx, instruct.RoleName(roleName), cli, instructBudgetFlag)
	if err != nil {
		return err
	}

	for _, f := range files {
		fmt.Printf("--- %s (%s) ---\n", f.Path, f.Purpose)
		fmt.Println(f.Content)
		fmt.Println()
	}

	fmt.Printf("# Tokens: %d | Sections: %d | Dropped: %d\n",
		result.TotalTokens, len(result.Sections), len(result.Dropped))
	if len(result.Dropped) > 0 {
		fmt.Printf("# Dropped: %s\n", strings.Join(result.Dropped, ", "))
	}
	if len(result.Warnings) > 0 {
		for _, w := range result.Warnings {
			fmt.Printf("# Warning: %s\n", w)
		}
	}

	return nil
}

func runInstructDiff(cmd *cobra.Command, args []string) error {
	log := setupLogger()

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	dist := buildDistributor(cfg, log)
	ctx := context.Background()

	agents := sortedAgentIDs(cfg)
	if instructAgentFlag != "" {
		if _, ok := cfg.Agents[instructAgentFlag]; !ok {
			return fmt.Errorf("unknown agent: %s", instructAgentFlag)
		}
		agents = []string{instructAgentFlag}
	}

	anyDiff := false
	for _, id := range agents {
		files, _, err := dist.PreviewForAgent(ctx, id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error rendering %s: %v\n", id, err)
			continue
		}

		for _, f := range files {
			existing, readErr := os.ReadFile(f.Path)
			if readErr != nil {
				// File doesn't exist yet.
				fmt.Printf("--- /dev/null\n+++ %s\n", f.Path)
				lines := strings.Split(f.Content, "\n")
				for _, line := range lines {
					fmt.Printf("+%s\n", line)
				}
				fmt.Println()
				anyDiff = true
				continue
			}

			if string(existing) == f.Content {
				continue
			}

			// Simple unified diff.
			oldLines := strings.Split(string(existing), "\n")
			newLines := strings.Split(f.Content, "\n")
			diff := simpleDiff(oldLines, newLines)
			if diff != "" {
				fmt.Printf("--- %s (current)\n+++ %s (rendered)\n", f.Path, f.Path)
				fmt.Println(diff)
				anyDiff = true
			}
		}
	}

	if !anyDiff {
		fmt.Println("No differences found.")
	}

	return nil
}

func runInstructStatus(cmd *cobra.Command, args []string) error {
	log := setupLogger()

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	dist := buildDistributor(cfg, log)
	ctx := context.Background()

	// Do a dry-run render to check staleness.
	agents := sortedAgentIDs(cfg)

	fmt.Printf("%-14s %-8s %-6s %-20s %-6s %s\n", "Agent", "CLI", "Tokens", "Generated", "Stale", "Hash")
	fmt.Println(strings.Repeat("-", 80))

	for _, id := range agents {
		agentCfg := cfg.Agents[id]
		cli := agentCfg.Plugin

		// Check if there's a file on disk already.
		adapter, adErr := instruct.AdapterForCLI(instruct.AgentCLI(cli))
		var onDisk bool
		if adErr == nil {
			probe := &instruct.RenderResult{Content: "probe"}
			probeFiles, pErr := adapter.PrepareFiles(probe, cfg.Project.Root, nil)
			if pErr == nil && len(probeFiles) > 0 {
				if _, sErr := os.Stat(probeFiles[0].Path); sErr == nil {
					onDisk = true
				}
			}
		}

		// Try to render and compare hashes to detect staleness.
		stale := "?"
		tokens := 0
		hash := "-"
		generated := "never"

		if onDisk {
			files, result, err := dist.PreviewForAgent(ctx, id)
			if err == nil {
				tokens = result.TotalTokens
				newHash := fmt.Sprintf("%x", hashContent(result.Content))
				hash = newHash[:12]

				// Read current file and check content match.
				if len(files) > 0 {
					existing, rErr := os.ReadFile(files[0].Path)
					if rErr == nil {
						existingHash := fmt.Sprintf("%x", hashContent(string(existing)))
						if existingHash[:12] != hash {
							stale = "yes"
						} else {
							stale = "no"
						}
					}
				}

				info, sErr := os.Stat(files[0].Path)
				if sErr == nil {
					generated = info.ModTime().Format(time.RFC3339)
				}
			}
		}

		fmt.Printf("%-14s %-8s %-6d %-20s %-6s %s\n",
			id, cli, tokens, truncTime(generated, 20), stale, hash)
	}

	return nil
}

func runInstructValidate(cmd *cobra.Command, args []string) error {
	log := setupLogger()

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	dist := buildDistributor(cfg, log)
	ctx := context.Background()

	agents := sortedAgentIDs(cfg)
	if instructAgentFlag != "" {
		if _, ok := cfg.Agents[instructAgentFlag]; !ok {
			return fmt.Errorf("unknown agent: %s", instructAgentFlag)
		}
		agents = []string{instructAgentFlag}
	}

	anyInvalid := false
	for _, id := range agents {
		issues := validateAgent(ctx, dist, cfg, id)
		if len(issues) == 0 {
			fmt.Printf("  %s: OK\n", id)
		} else {
			anyInvalid = true
			fmt.Printf("  %s: INVALID\n", id)
			for _, issue := range issues {
				fmt.Printf("    - %s\n", issue)
			}
		}
	}

	if anyInvalid {
		return fmt.Errorf("validation failed for one or more agents")
	}
	return nil
}

// validateAgent checks a single agent's instruction files for common issues.
// Returns a list of issue descriptions; an empty slice means valid.
func validateAgent(ctx context.Context, dist *instruct.Distributor, cfg *config.Config, agentID string) []string {
	var issues []string

	agentCfg := cfg.Agents[agentID]
	cli := instruct.AgentCLI(agentCfg.Plugin)

	adapter, err := instruct.AdapterForCLI(cli)
	if err != nil {
		return append(issues, fmt.Sprintf("unknown CLI %q", agentCfg.Plugin))
	}

	// Probe for file paths.
	probe := &instruct.RenderResult{Content: "probe"}
	probeFiles, err := adapter.PrepareFiles(probe, cfg.Project.Root, nil)
	if err != nil {
		return append(issues, fmt.Sprintf("failed to determine file paths: %v", err))
	}

	// Check 1: Files exist.
	var primaryContent string
	for _, f := range probeFiles {
		data, readErr := os.ReadFile(f.Path)
		if readErr != nil {
			issues = append(issues, fmt.Sprintf("file missing: %s", f.Path))
		} else if f.Purpose == "primary" {
			primaryContent = string(data)
		}
	}

	// If primary file is missing, remaining checks are not meaningful.
	if primaryContent == "" {
		return issues
	}

	// Check 2: Token budget.
	tokens := instruct.EstimateTokens(primaryContent)
	budget := adapter.TokenBudget()
	if tokens > budget {
		issues = append(issues, fmt.Sprintf("token count %d exceeds budget %d", tokens, budget))
	}

	// Check 3: Critical sections.
	for _, header := range []string{"## Identity", "## Constraints", "## Session"} {
		if !strings.Contains(primaryContent, header) {
			issues = append(issues, fmt.Sprintf("missing critical section: %s", header))
		}
	}

	// Check 4: Template syntax leak.
	if strings.Contains(primaryContent, "[[") && strings.Contains(primaryContent, "]]") {
		issues = append(issues, "leaked template syntax: found [[ and ]] in content")
	}

	// Check 5: Staleness.
	files, _, previewErr := dist.PreviewForAgent(ctx, agentID)
	if previewErr == nil {
		for _, f := range files {
			existing, readErr := os.ReadFile(f.Path)
			if readErr == nil && string(existing) != f.Content {
				issues = append(issues, fmt.Sprintf("stale: %s differs from rendered output", f.Path))
			}
		}
	}

	// Check 6: Adapter validation.
	if valErr := adapter.ValidateOutput(primaryContent); valErr != nil {
		issues = append(issues, fmt.Sprintf("adapter validation: %v", valErr))
	}

	return issues
}

// buildDistributor creates a Distributor from config for CLI use.
func buildDistributor(cfg *config.Config, logger *slog.Logger) *instruct.Distributor {

	templateFS, err := instruct.TemplatesFS()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load templates: %v\n", err)
	}

	renderer, err := instruct.NewRenderer(templateFS, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not create renderer: %v\n", err)
	}

	rp := &configRoleProvider{}
	agg := instruct.NewAggregator(instruct.AggregatorDeps{
		Config: instruct.AggregatorConfig{
			ProjectName:        cfg.Project.Name,
			Language:           languageFromConfig(cfg),
			RepoRoot:           cfg.Project.Root,
		},
		Roles:   rp,
		Version: version,
		Logger:  logger,
	})

	agents := &configAgentProvider{cfg: cfg}

	return instruct.NewDistributor(instruct.DistributorDeps{
		Aggregator:  agg,
		Renderer:    renderer,
		Agents:      agents,
		ProjectRoot: cfg.Project.Root,
		Logger:      logger,
	})
}

// configAgentProvider adapts config.Config to instruct.AgentConfigProvider.
type configAgentProvider struct {
	cfg *config.Config
}

func (p *configAgentProvider) AgentIDs() []string {
	return sortedAgentIDs(p.cfg)
}

func (p *configAgentProvider) AgentRole(id string) instruct.RoleName {
	if a, ok := p.cfg.Agents[id]; ok {
		return instruct.RoleName(roles.NormalizeRole(a.Role))
	}
	return instruct.RoleSoftwareEngineer
}

func (p *configAgentProvider) AgentCLI(id string) instruct.AgentCLI {
	if a, ok := p.cfg.Agents[id]; ok {
		return instruct.AgentCLI(a.Plugin)
	}
	return instruct.CLIClaude
}

func (p *configAgentProvider) AgentPaneID(_ string) string {
	return "" // No pane IDs available in CLI context.
}

// configRoleProvider looks up role definitions from the embedded roles package.
type configRoleProvider struct{}

func (p *configRoleProvider) GetRole(name instruct.RoleName) instruct.RoleContext {
	rc, err := roles.BuildRoleContext(name)
	if err != nil {
		return instruct.RoleContext{Name: name, Title: string(name)}
	}
	return rc
}

// sortedAgentIDs returns agent IDs from config in sorted order.
func sortedAgentIDs(cfg *config.Config) []string {
	ids := make([]string, 0, len(cfg.Agents))
	for id := range cfg.Agents {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// languageFromConfig extracts language from config or returns a sensible default.
func languageFromConfig(cfg *config.Config) string {
	// Check if there's a go.mod in the project root.
	if _, err := os.Stat(cfg.Project.Root + "/go.mod"); err == nil {
		return "Go"
	}
	if _, err := os.Stat(cfg.Project.Root + "/package.json"); err == nil {
		return "JavaScript/TypeScript"
	}
	return ""
}

// printGenerationResult prints a summary line for a generated agent.
func printGenerationResult(agentID string, s *instruct.GenerationState) {
	fmt.Printf("  %s (%s): %d tokens, %d files\n",
		agentID, s.CLI, s.TokenCount, len(s.FilesWritten))
	for _, f := range s.FilesWritten {
		fmt.Printf("    -> %s\n", f)
	}
}

// simpleDiff produces a simple line-based diff between old and new lines.
func simpleDiff(oldLines, newLines []string) string {
	var sb strings.Builder
	maxLen := len(oldLines)
	if len(newLines) > maxLen {
		maxLen = len(newLines)
	}

	i, j := 0, 0
	for i < len(oldLines) || j < len(newLines) {
		if i < len(oldLines) && j < len(newLines) && oldLines[i] == newLines[j] {
			i++
			j++
			continue
		}
		if i < len(oldLines) {
			sb.WriteString("-")
			sb.WriteString(oldLines[i])
			sb.WriteString("\n")
			i++
		}
		if j < len(newLines) {
			sb.WriteString("+")
			sb.WriteString(newLines[j])
			sb.WriteString("\n")
			j++
		}
	}

	return sb.String()
}

// hashContent computes SHA-256 of content for status display.
func hashContent(content string) [32]byte {
	return sha256.Sum256([]byte(content))
}

// truncTime truncates a time string to maxLen characters.
func truncTime(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
