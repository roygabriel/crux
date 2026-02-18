package main

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile string
	version = "dev"
)

var rootCmd = &cobra.Command{
	Use:     "crux",
	Short:   "Multi-agent CLI orchestrator",
	Long:    "Crux orchestrates multiple AI coding agents in tmux sessions with phase-based execution, persistent memory, and verification gates.",
	Version: version,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", ".crux/config.yaml", "config file path")

	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(phaseCmd)
	rootCmd.AddCommand(decisionsCmd)
	rootCmd.AddCommand(notesCmd)
	rootCmd.AddCommand(replayCmd)
	rootCmd.AddCommand(auditCmd)
	rootCmd.AddCommand(instructCmd)
}

func setupLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}
