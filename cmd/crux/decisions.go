package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/roygabriel/crux/internal/config"
	"github.com/roygabriel/crux/internal/memory/journal"
	"github.com/roygabriel/crux/internal/memory/store"
	"github.com/roygabriel/crux/pkg/types"
	"github.com/spf13/cobra"
)

var (
	decisionsJSONFlag bool
	decisionsLimit    int
)

var decisionsCmd = &cobra.Command{
	Use:   "decisions",
	Short: "Query and export decision journal",
	Long:  "Search, list, export, and inspect decisions recorded during orchestration.",
}

var decisionsSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Full-text search decisions",
	Args:  cobra.ExactArgs(1),
	RunE:  runDecisionsSearch,
}

var decisionsRecentCmd = &cobra.Command{
	Use:   "recent",
	Short: "Show recent decisions",
	RunE:  runDecisionsRecent,
}

var decisionsExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export all decisions as JSONL",
	RunE:  runDecisionsExport,
}

var decisionsShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Display a single decision",
	Args:  cobra.ExactArgs(1),
	RunE:  runDecisionsShow,
}

func init() {
	decisionsSearchCmd.Flags().IntVar(&decisionsLimit, "limit", 10, "Maximum results")
	decisionsSearchCmd.Flags().BoolVar(&decisionsJSONFlag, "json", false, "Output as JSON")

	decisionsRecentCmd.Flags().IntVar(&decisionsLimit, "limit", 20, "Maximum results")
	decisionsRecentCmd.Flags().BoolVar(&decisionsJSONFlag, "json", false, "Output as JSON")

	decisionsCmd.AddCommand(decisionsSearchCmd)
	decisionsCmd.AddCommand(decisionsRecentCmd)
	decisionsCmd.AddCommand(decisionsExportCmd)
	decisionsCmd.AddCommand(decisionsShowCmd)
}

// openJournal creates a store and journal from config. Caller must close the store.
func openJournal(cfg *config.Config) (*store.Store, *journal.Journal, error) {
	st, err := store.NewStore(cfg.Memory.SQLitePath)
	if err != nil {
		return nil, nil, fmt.Errorf("open store: %w", err)
	}
	if err := st.Migrate(); err != nil {
		st.Close()
		return nil, nil, fmt.Errorf("migrate store: %w", err)
	}
	j := journal.NewJournal(st, nil)
	return st, j, nil
}

func runDecisionsSearch(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	st, j, err := openJournal(cfg)
	if err != nil {
		return err
	}
	defer st.Close()

	ctx := context.Background()
	decisions, err := j.Search(ctx, args[0], decisionsLimit)
	if err != nil {
		return fmt.Errorf("search decisions: %w", err)
	}

	if decisionsJSONFlag {
		return printDecisionsJSON(decisions)
	}
	printDecisions(decisions)
	return nil
}

func runDecisionsRecent(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	st, j, err := openJournal(cfg)
	if err != nil {
		return err
	}
	defer st.Close()

	ctx := context.Background()
	decisions, err := j.Recent(ctx, decisionsLimit)
	if err != nil {
		return fmt.Errorf("recent decisions: %w", err)
	}

	if decisionsJSONFlag {
		return printDecisionsJSON(decisions)
	}
	printDecisions(decisions)
	return nil
}

func runDecisionsExport(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	st, j, err := openJournal(cfg)
	if err != nil {
		return err
	}
	defer st.Close()

	ctx := context.Background()
	return j.Export(ctx, os.Stdout)
}

func runDecisionsShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	st, err := store.NewStore(cfg.Memory.SQLitePath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	if err := st.Migrate(); err != nil {
		return fmt.Errorf("migrate store: %w", err)
	}

	ctx := context.Background()
	d, err := st.GetDecision(ctx, args[0])
	if err != nil {
		return fmt.Errorf("get decision: %w", err)
	}
	if d == nil {
		return fmt.Errorf("decision not found: %s", args[0])
	}

	printDecision(*d)
	return nil
}

func printDecisions(decisions []types.Decision) {
	if len(decisions) == 0 {
		fmt.Println("No decisions found.")
		return
	}
	fmt.Printf("%-20s %-6s %-10s %-12s %s\n", "Timestamp", "Phase", "Agent", "Action", "Rationale")
	fmt.Println(strings.Repeat("\u2500", 80))
	for _, d := range decisions {
		ts := d.Timestamp.Format(time.DateTime)
		rationale := d.Rationale
		if len(rationale) > 40 {
			rationale = rationale[:37] + "..."
		}
		action := d.Action
		if len(action) > 12 {
			action = action[:9] + "..."
		}
		fmt.Printf("%-20s %-6s %-10s %-12s %s\n",
			ts,
			string(d.PhaseID),
			string(d.AgentID),
			action,
			rationale,
		)
	}
}

func printDecisionsJSON(decisions []types.Decision) error {
	if decisions == nil {
		decisions = []types.Decision{}
	}
	data, err := json.MarshalIndent(decisions, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal decisions: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func printDecision(d types.Decision) {
	fmt.Printf("ID:        %s\n", d.ID)
	fmt.Printf("Timestamp: %s\n", d.Timestamp.Format(time.RFC3339))
	fmt.Printf("Phase:     %s\n", d.PhaseID)
	fmt.Printf("Prompt:    %d\n", d.PromptNum)
	fmt.Printf("Agent:     %s\n", d.AgentID)
	fmt.Printf("Context:   %s\n", d.Context)
	fmt.Printf("Rationale: %s\n", d.Rationale)
	fmt.Printf("Action:    %s\n", d.Action)
	if d.Outcome != "" {
		fmt.Printf("Outcome:   %s\n", d.Outcome)
	}
}
