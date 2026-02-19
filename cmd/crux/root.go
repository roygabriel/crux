package main

import (
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

var (
	cfgFile   string
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
	vcsDirty  = "unknown"
)

var rootCmd = &cobra.Command{
	Use:     "crux",
	Short:   "Multi-agent CLI orchestrator",
	Long:    "Crux orchestrates multiple AI coding agents in tmux sessions with phase-based execution, persistent memory, and verification gates.",
	Version: version,
}

func init() {
	loadBuildInfoFallbacks()
	rootCmd.Version = formatVersionOutput(version, commit, buildDate, vcsDirty)
	rootCmd.SetVersionTemplate("{{.Version}}\n")
	rootCmd.InitDefaultVersionFlag()
	if versionFlag := rootCmd.Flags().Lookup("version"); versionFlag != nil {
		versionFlag.Shorthand = "v"
		versionFlag.Usage = "print version/build information"
	}

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
	rootCmd.AddCommand(prefsCmd)
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(generateCmd)
}

func loadBuildInfoFallbacks() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if commit == "" || commit == "unknown" {
				commit = shortCommit(s.Value)
			}
		case "vcs.time":
			if buildDate == "" || buildDate == "unknown" {
				buildDate = s.Value
			}
		case "vcs.modified":
			if vcsDirty == "" || vcsDirty == "unknown" {
				vcsDirty = s.Value
			}
		}
	}
}

func shortCommit(in string) string {
	if len(in) <= 12 {
		return in
	}
	return in[:12]
}

func formatVersionOutput(ver, sha, date, dirty string) string {
	if strings.TrimSpace(ver) == "" {
		ver = "dev"
	}
	if strings.TrimSpace(sha) == "" {
		sha = "unknown"
	}
	if strings.TrimSpace(date) == "" {
		date = "unknown"
	}
	if strings.TrimSpace(dirty) == "" {
		dirty = "unknown"
	}
	return fmt.Sprintf("crux %s (commit=%s date=%s dirty=%s)", ver, sha, date, dirty)
}

func setupLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}
