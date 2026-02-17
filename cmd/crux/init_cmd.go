package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize project directory structure",
	Long:  "Creates .crux/ state directory, copies default config, and creates docs/phases/ and work-notes/ directories.",
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	log := setupLogger()

	dirs := []string{
		".crux",
		"docs/phases",
		"work-notes",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
		log.Info("created directory", "path", dir)
	}

	cfgPath := filepath.Join(".crux", "config.yaml")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		src, err := os.ReadFile("configs/default.yaml")
		if err != nil {
			log.Warn("default config not found, skipping copy", "error", err)
		} else {
			if err := os.WriteFile(cfgPath, src, 0o644); err != nil {
				return fmt.Errorf("write config: %w", err)
			}
			log.Info("created config", "path", cfgPath)
		}
	} else {
		log.Info("config already exists, skipping", "path", cfgPath)
	}

	fmt.Println("project initialized")
	return nil
}
