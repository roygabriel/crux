package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/roygabriel/crux/internal/config"
	"github.com/roygabriel/crux/internal/memory/worknotes"
	"github.com/roygabriel/crux/pkg/types"
	"github.com/spf13/cobra"
)

var notesCmd = &cobra.Command{
	Use:   "notes",
	Short: "Manage per-phase work notes",
	Long:  "Show, list, and edit per-phase work notes.",
}

var notesShowCmd = &cobra.Command{
	Use:               "show <phase-id>",
	Short:             "Display work notes for a phase",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: phaseIDCompletion,
	RunE:              runNotesShow,
}

var notesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List work notes status for all phases",
	RunE:  runNotesList,
}

var notesEditCmd = &cobra.Command{
	Use:               "edit <phase-id>",
	Short:             "Open work notes in editor",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: phaseIDCompletion,
	RunE:              runNotesEdit,
}

func init() {
	notesCmd.AddCommand(notesShowCmd)
	notesCmd.AddCommand(notesListCmd)
	notesCmd.AddCommand(notesEditCmd)
}

func runNotesShow(cmd *cobra.Command, args []string) error {
	log := setupLogger()

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	notesDir := filepath.Join(cfg.Project.StateDir, "notes")
	mgr := worknotes.NewManager(notesDir, log)

	notes, err := mgr.Read(args[0])
	if err != nil {
		return fmt.Errorf("read notes: %w", err)
	}

	fmt.Print(mgr.Render(notes))
	return nil
}

func runNotesList(cmd *cobra.Command, args []string) error {
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

	notesDir := filepath.Join(cfg.Project.StateDir, "notes")
	mgr := worknotes.NewManager(notesDir, log)

	order := engine.PhaseOrder()
	progress := engine.Progress()

	fmt.Printf("%-6s %-24s %s\n", "Phase", "Name", "Notes Status")
	fmt.Println(strings.Repeat("\u2500", 50))

	for _, id := range order {
		prog := progress[id]
		name := ""
		if prog.Spec != nil {
			name = prog.Spec.Name
		}

		notes, err := mgr.Read(string(id))
		status := "no notes"
		if err == nil && notes != nil {
			status = notes.Status
		}

		fmt.Printf("%-6s %-24s %s\n", string(id), padOrTruncate(name, 24), status)
	}

	return nil
}

func runNotesEdit(cmd *cobra.Command, args []string) error {
	log := setupLogger()

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	notesDir := filepath.Join(cfg.Project.StateDir, "notes")
	mgr := worknotes.NewManager(notesDir, log)

	phaseID := args[0]
	notesPath := filepath.Join(notesDir, fmt.Sprintf("PHASE%s.md", phaseID))

	// Initialize notes file if it doesn't exist.
	if _, err := os.Stat(notesPath); os.IsNotExist(err) {
		// Try to get phase name from engine.
		phaseName := phaseID
		engine := loadPhaseEngine(cfg, log)
		if engine != nil {
			progress := engine.Progress()
			if prog, ok := progress[types.PhaseID(phaseID)]; ok && prog.Spec != nil {
				phaseName = prog.Spec.Name
			}
		}
		if err := mgr.Init(phaseID, phaseName); err != nil {
			return fmt.Errorf("init notes: %w", err)
		}
	}

	editor := getEditor()
	c := exec.Command(editor, notesPath)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// getEditor returns the user's preferred editor.
func getEditor() string {
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	if e := os.Getenv("VISUAL"); e != "" {
		return e
	}
	return "vi"
}
