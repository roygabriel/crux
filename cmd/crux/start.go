package main

import (
	"fmt"

	"github.com/roygabriel/crux/internal/config"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the orchestration loop",
	RunE: func(cmd *cobra.Command, args []string) error {
		log := setupLogger()

		log.Info("loading configuration", "path", cfgFile)
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		log.Info("starting orchestration", "project", cfg.Project.Name)
		fmt.Println("starting orchestration")
		return nil
	},
}
