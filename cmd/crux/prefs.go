package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/roygabriel/crux/internal/instruct/prefs"
	"github.com/spf13/cobra"
)

var prefsCmd = &cobra.Command{
	Use:   "prefs",
	Short: "Manage engineering preferences",
	Long:  "View, edit, and manage engineering preferences that control how agents write code.",
}

var prefsShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display current preferences",
	Long:  "Reads .crux/preferences.yaml and displays a formatted summary of all engineering preferences.",
	RunE:  runPrefsShow,
}

var prefsEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit preferences interactively",
	Long:  "Re-runs the preference questionnaire with current preferences as defaults. Saves the result to .crux/preferences.yaml.",
	RunE:  runPrefsEdit,
}

func init() {
	prefsCmd.AddCommand(prefsShowCmd)
	prefsCmd.AddCommand(prefsEditCmd)
}

func runPrefsShow(_ *cobra.Command, _ []string) error {
	cruxDir := filepath.Dir(cfgFile)
	store := prefs.NewStore(cruxDir, nil)

	if !store.Exists() {
		return fmt.Errorf("no preferences file found at %s — run 'crux prefs edit' or 'crux init' to create one",
			filepath.Join(cruxDir, "preferences.yaml"))
	}

	p, err := store.Load()
	if err != nil {
		return fmt.Errorf("load preferences: %w", err)
	}

	summary := prefs.RenderPreferencesSummary(p, "")
	fmt.Println(summary)

	return nil
}

func runPrefsEdit(_ *cobra.Command, _ []string) error {
	cruxDir := filepath.Dir(cfgFile)
	store := prefs.NewStore(cruxDir, slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	})))

	// Load existing preferences as defaults, or use pragmatic preset.
	var defaults *prefs.Preferences
	if store.Exists() {
		loaded, err := store.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not load existing preferences: %v\n", err)
		} else {
			defaults = loaded
		}
	}

	q := prefs.NewQuestionnaire(defaults)
	if err := q.Run(); err != nil {
		return fmt.Errorf("questionnaire: %w", err)
	}

	result := q.Result()
	if result == nil {
		return fmt.Errorf("preferences cancelled")
	}

	if err := store.Save(result); err != nil {
		return fmt.Errorf("save preferences: %w", err)
	}

	fmt.Printf("\u2713 Preferences saved to %s\n", filepath.Join(cruxDir, "preferences.yaml"))
	return nil
}
