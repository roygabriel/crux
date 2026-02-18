package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/roygabriel/crux/internal/config"
	"github.com/roygabriel/crux/internal/phase"
	"github.com/roygabriel/crux/pkg/types"
	"github.com/spf13/cobra"
)

var (
	phaseJSONFlag    bool
	forceAdvanceYes  bool
	createID         string
	createName       string
	createDependsOn  []string
	createNumPrompts int
)

var phaseCmd = &cobra.Command{
	Use:   "phase",
	Short: "Manage project phases",
	Long:  "List, inspect, validate, create, and advance project phases.",
}

var phaseListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show all phases",
	RunE:  runPhaseList,
}

var phaseShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Display full phase spec",
	Args:  cobra.ExactArgs(1),
	RunE:  runPhaseShow,
}

var phaseValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Check specs for completeness",
	RunE:  runPhaseValidate,
}

var phaseCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Generate phase from templates",
	RunE:  runPhaseCreate,
}

var phaseAdvanceCmd = &cobra.Command{
	Use:   "advance <id>",
	Short: "Force-advance a phase (skips gates)",
	Args:  cobra.ExactArgs(1),
	RunE:  runPhaseAdvance,
}

func init() {
	phaseListCmd.Flags().BoolVar(&phaseJSONFlag, "json", false, "Output as JSON")

	phaseCreateCmd.Flags().StringVar(&createID, "id", "", "Phase ID (required)")
	phaseCreateCmd.Flags().StringVar(&createName, "name", "", "Phase name (required)")
	phaseCreateCmd.Flags().StringSliceVar(&createDependsOn, "depends-on", nil, "Dependent phase IDs")
	phaseCreateCmd.Flags().IntVar(&createNumPrompts, "prompts", 2, "Number of prompts to generate")
	_ = phaseCreateCmd.MarkFlagRequired("id")
	_ = phaseCreateCmd.MarkFlagRequired("name")

	phaseAdvanceCmd.Flags().BoolVarP(&forceAdvanceYes, "yes", "y", false, "Skip confirmation prompt")

	phaseCmd.AddCommand(phaseListCmd)
	phaseCmd.AddCommand(phaseShowCmd)
	phaseCmd.AddCommand(phaseValidateCmd)
	phaseCmd.AddCommand(phaseCreateCmd)
	phaseCmd.AddCommand(phaseAdvanceCmd)
}

// PhaseListOutput is the JSON representation for phase list.
type PhaseListOutput struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Status    string   `json:"status"`
	Completed int      `json:"completed"`
	Total     int      `json:"total"`
	DependsOn []string `json:"depends_on,omitempty"`
}

func runPhaseList(cmd *cobra.Command, args []string) error {
	log := setupLogger()

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	engine := loadPhaseEngine(cfg, log)
	if engine == nil {
		fmt.Println("No phase specs found.")
		return nil
	}

	order := engine.PhaseOrder()
	progress := engine.Progress()

	if phaseJSONFlag {
		return printPhaseListJSON(order, progress)
	}

	printPhaseListFormatted(order, progress)
	return nil
}

func printPhaseListJSON(order []types.PhaseID, progress map[types.PhaseID]phase.PhaseProgress) error {
	lines := make([]PhaseListOutput, 0, len(order))
	for _, id := range order {
		prog := progress[id]
		name := ""
		status := ""
		var deps []string
		if prog.Spec != nil {
			name = prog.Spec.Name
			status = string(prog.Spec.Status)
			for _, d := range prog.Spec.DependsOn {
				deps = append(deps, string(d))
			}
		}
		lines = append(lines, PhaseListOutput{
			ID:        string(id),
			Name:      name,
			Status:    status,
			Completed: prog.CompletedPrompts,
			Total:     len(prog.Prompts),
			DependsOn: deps,
		})
	}
	data, err := json.MarshalIndent(lines, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal phase list: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func printPhaseListFormatted(order []types.PhaseID, progress map[types.PhaseID]phase.PhaseProgress) {
	fmt.Printf("%-6s %-24s %-10s %-10s %s\n", "ID", "Name", "Status", "Prompts", "Depends")
	fmt.Println(strings.Repeat("\u2500", 72))
	for _, id := range order {
		prog := progress[id]
		name := ""
		status := ""
		var deps []string
		if prog.Spec != nil {
			name = prog.Spec.Name
			status = string(prog.Spec.Status)
			for _, d := range prog.Spec.DependsOn {
				deps = append(deps, string(d))
			}
		}
		depStr := strings.Join(deps, ", ")
		if depStr == "" {
			depStr = "\u2014"
		}
		promptStr := fmt.Sprintf("%d/%d", prog.CompletedPrompts, len(prog.Prompts))
		fmt.Printf("%-6s %-24s %-10s %-10s %s\n",
			string(id),
			padOrTruncate(name, 24),
			padOrTruncate(status, 10),
			promptStr,
			depStr,
		)
	}
}

func runPhaseShow(cmd *cobra.Command, args []string) error {
	log := setupLogger()

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	engine := loadPhaseEngine(cfg, log)
	if engine == nil {
		return fmt.Errorf("no phase specs found")
	}

	id := types.PhaseID(args[0])
	progress := engine.Progress()
	prog, ok := progress[id]
	if !ok {
		return fmt.Errorf("unknown phase: %s", id)
	}

	printPhaseSpec(prog.Spec, prog)
	return nil
}

func printPhaseSpec(spec *phase.PhaseSpec, progress phase.PhaseProgress) {
	fmt.Printf("Phase %s: %s\n", spec.ID, spec.Name)
	fmt.Println(strings.Repeat("\u2550", 40))

	fmt.Printf("Status:     %s\n", spec.Status)
	fmt.Printf("Prompts:    %d/%d completed\n", progress.CompletedPrompts, len(progress.Prompts))
	fmt.Printf("Gates:      %d/%d passed\n", progress.GatesPassed, progress.GatesTotal)

	if len(spec.DependsOn) > 0 {
		deps := make([]string, len(spec.DependsOn))
		for i, d := range spec.DependsOn {
			deps[i] = string(d)
		}
		fmt.Printf("Depends On: %s\n", strings.Join(deps, ", "))
	}

	if spec.Rationale != "" {
		fmt.Println()
		fmt.Println("Rationale:")
		fmt.Println(spec.Rationale)
	}

	if len(spec.Tasks) > 0 {
		fmt.Println()
		fmt.Println("Tasks:")
		for _, tg := range spec.Tasks {
			if tg.PromptNumber > 0 {
				fmt.Printf("  Prompt %d:\n", tg.PromptNumber)
			} else if tg.Description != "" {
				fmt.Printf("  %s:\n", tg.Description)
			}
			for _, item := range tg.Items {
				fmt.Printf("    - %s\n", item)
			}
		}
	}

	if len(spec.FilesNew) > 0 || len(spec.FilesModified) > 0 || len(spec.FilesRef) > 0 {
		fmt.Println()
		fmt.Println("Files:")
		for _, f := range spec.FilesNew {
			fmt.Printf("  + %s\n", f)
		}
		for _, f := range spec.FilesModified {
			fmt.Printf("  ~ %s\n", f)
		}
		for _, f := range spec.FilesRef {
			fmt.Printf("  . %s\n", f)
		}
	}

	if len(spec.ExitCriteria) > 0 {
		fmt.Println()
		fmt.Println("Exit Criteria:")
		for _, gate := range spec.ExitCriteria {
			indicator := "\u25cb"
			if gate.Type == phase.GateAutomated {
				indicator = "\u2713"
			}
			if gate.Command != "" {
				fmt.Printf("  %s `%s` %s\n", indicator, gate.Command, gate.Expected)
			} else {
				fmt.Printf("  %s %s\n", indicator, gate.Expected)
			}
		}
	}

	if len(spec.ProgressNotes) > 0 {
		fmt.Println()
		fmt.Println("Progress Notes:")
		for _, note := range spec.ProgressNotes {
			fmt.Printf("  - %s\n", note)
		}
	}
}

func runPhaseValidate(cmd *cobra.Command, args []string) error {
	log := setupLogger()

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	engine := loadPhaseEngine(cfg, log)
	if engine == nil {
		return fmt.Errorf("no phase specs found")
	}

	progress := engine.Progress()
	order := engine.PhaseOrder()

	var failures []string
	passed := 0

	// Check: every spec has exit criteria.
	for _, id := range order {
		prog := progress[id]
		if prog.Spec != nil && len(prog.Spec.ExitCriteria) == 0 {
			failures = append(failures, fmt.Sprintf("Phase %s: missing exit criteria", id))
		} else {
			passed++
		}
	}

	// Check: every prompt has verification.
	for _, id := range order {
		prog := progress[id]
		for _, p := range prog.Prompts {
			if len(p.Verification) == 0 {
				failures = append(failures, fmt.Sprintf("Phase %s Prompt %d: missing verification", id, p.PromptNumber))
			} else {
				passed++
			}
		}
	}

	// Check: parallelism — gather active phases and validate no file conflicts.
	var activePhaseIDs []types.PhaseID
	for _, id := range order {
		prog := progress[id]
		if prog.Spec != nil && prog.Spec.Status == types.PhaseActive {
			activePhaseIDs = append(activePhaseIDs, id)
		}
	}
	if len(activePhaseIDs) > 1 {
		if err := engine.ValidateParallelism(activePhaseIDs); err != nil {
			failures = append(failures, fmt.Sprintf("Parallelism: %s", err))
		} else {
			passed++
		}
	}

	// Print results.
	fmt.Printf("Validation: %d passed", passed)
	if len(failures) > 0 {
		fmt.Printf(", %d failed\n\n", len(failures))
		for _, f := range failures {
			fmt.Printf("  \u2715 %s\n", f)
		}
		return fmt.Errorf("validation failed: %d issues", len(failures))
	}
	fmt.Println(", 0 failed")
	return nil
}

func runPhaseCreate(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	specDir := cfg.Phases.SpecDir
	specPath := filepath.Join(specDir, fmt.Sprintf("PHASE%s.md", createID))
	promptPath := filepath.Join(specDir, fmt.Sprintf("PHASE%s-PROMPT.md", createID))

	// Error if files exist.
	if _, err := os.Stat(specPath); err == nil {
		return fmt.Errorf("spec file already exists: %s", specPath)
	}
	if _, err := os.Stat(promptPath); err == nil {
		return fmt.Errorf("prompt file already exists: %s", promptPath)
	}

	if err := os.MkdirAll(specDir, 0o755); err != nil {
		return fmt.Errorf("create spec dir: %w", err)
	}

	specContent := renderSpecTemplate(createID, createName, createDependsOn, createNumPrompts)
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		return fmt.Errorf("write spec: %w", err)
	}
	fmt.Printf("\u2713 Created %s\n", specPath)

	promptContent := renderPromptTemplate(createID, createNumPrompts)
	if err := os.WriteFile(promptPath, []byte(promptContent), 0o644); err != nil {
		return fmt.Errorf("write prompt: %w", err)
	}
	fmt.Printf("\u2713 Created %s\n", promptPath)

	return nil
}

func renderSpecTemplate(id, name string, dependsOn []string, numPrompts int) string {
	depStr := "None"
	if len(dependsOn) > 0 {
		deps := make([]string, len(dependsOn))
		for i, d := range dependsOn {
			deps[i] = fmt.Sprintf("- Phase %s", d)
		}
		depStr = strings.Join(deps, "\n")
	}

	var tasks strings.Builder
	for i := 1; i <= numPrompts; i++ {
		tasks.WriteString(fmt.Sprintf("### Prompt %d\n- <Task description>\n\n", i))
	}

	return fmt.Sprintf(`# Phase %s: %s

## Status
Planned

## Depends On
%s

## Design Rationale
<Why this phase exists, what it isolates, why now.>

## Tasks

%s## Files

### New
- <file path>

### Modified
- <file path>

### Referenced (read-only)
- <file path>

## Exit Criteria
- [ ] `+"`go build ./...`"+` exits 0
- [ ] `+"`go vet ./...`"+` exits 0
- [ ] `+"`go test -race ./...`"+` exits 0

## Progress Notes
`, id, name, depStr, tasks.String())
}

func renderPromptTemplate(id string, numPrompts int) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Phase %s Implementation Prompts\n", id))

	for i := 1; i <= numPrompts; i++ {
		b.WriteString(fmt.Sprintf(`
## Prompt %d of %d: <Title>

### Required Reading (read these files before writing code)
- <file path>

### Task

1. <Step>

### Verification
`+"```bash"+`
go build ./...
go vet ./...
go test -race ./...
`+"```"+`

### Acceptance Criteria
- <Criterion>

---
`, i, numPrompts))
	}
	return b.String()
}

func runPhaseAdvance(cmd *cobra.Command, args []string) error {
	log := setupLogger()

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	engine := loadPhaseEngine(cfg, log)
	if engine == nil {
		return fmt.Errorf("no phase specs found")
	}

	id := types.PhaseID(args[0])

	if !forceAdvanceYes {
		fmt.Printf("Force advance Phase %s? Gates will be skipped. [y/n] ", id)
		reader := bufio.NewReader(os.Stdin)
		answer, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading input: %w", err)
		}
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	ctx := context.Background()
	if err := engine.ForceAdvance(ctx, id); err != nil {
		return fmt.Errorf("force advance: %w", err)
	}

	fmt.Printf("Phase %s advanced.\n", id)
	return nil
}
