package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/roygabriel/crux/internal/config"
	"github.com/roygabriel/crux/internal/memory/session"
	"github.com/roygabriel/crux/internal/memory/store"
	"github.com/spf13/cobra"
)

var replayEventsFlag bool

var replayCmd = &cobra.Command{
	Use:   "replay [session-id]",
	Short: "List sessions or replay a session",
	Long:  "Without arguments, lists all sessions. With a session ID, shows session detail and events.",
	RunE:  runReplay,
}

func init() {
	replayCmd.Flags().BoolVar(&replayEventsFlag, "events", false, "Show session events")
}

func runReplay(cmd *cobra.Command, args []string) error {
	log := setupLogger()

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	sessDir := filepath.Join(cfg.Project.StateDir, "sessions")
	sessionMgr := session.NewManager(sessDir, nil, log)

	if len(args) == 0 {
		return runReplayList(sessionMgr)
	}

	return runReplayShow(cfg, sessionMgr, args[0])
}

func runReplayList(sessionMgr *session.Manager) error {
	sessions, err := sessionMgr.List()
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found.")
		return nil
	}

	fmt.Printf("%-10s %-22s %-8s %s\n", "ID", "Started", "Phase", "Prompt")
	fmt.Println(strings.Repeat("\u2500", 56))

	for _, sc := range sessions {
		ts := sc.StartedAt.Format(time.DateTime)
		phase := sc.CurrentPhase
		if phase == "" {
			phase = "\u2014"
		}
		fmt.Printf("%-10s %-22s %-8s %d\n", sc.ID, ts, phase, sc.PromptProgress)
	}

	return nil
}

func runReplayShow(cfg *config.Config, sessionMgr *session.Manager, sessionID string) error {
	sc, err := sessionMgr.Resume(sessionID)
	if err != nil {
		return fmt.Errorf("load session: %w", err)
	}

	fmt.Printf("Session:  %s\n", sc.ID)
	fmt.Printf("Started:  %s\n", sc.StartedAt.Format(time.RFC3339))
	fmt.Printf("Phase:    %s\n", sc.CurrentPhase)
	fmt.Printf("Progress: Prompt %d\n", sc.PromptProgress)

	if len(sc.Agents) > 0 {
		fmt.Println()
		fmt.Println("Agents:")
		for id, agent := range sc.Agents {
			fmt.Printf("  %-14s %-10s %s\n", id, agent.Status, agent.CurrentTask)
		}
	}

	if replayEventsFlag {
		return printSessionEvents(cfg, sc)
	}

	return nil
}

func printSessionEvents(cfg *config.Config, sc *session.SessionContext) error {
	st, err := store.NewStore(cfg.Memory.SQLitePath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	if err := st.Migrate(); err != nil {
		return fmt.Errorf("migrate store: %w", err)
	}

	ctx := context.Background()
	events, err := st.QueryEvents(ctx, store.EventFilter{Since: sc.StartedAt})
	if err != nil {
		return fmt.Errorf("query events: %w", err)
	}

	if len(events) == 0 {
		fmt.Println()
		fmt.Println("No events found for this session.")
		return nil
	}

	fmt.Println()
	fmt.Printf("%-22s %-12s %-16s %s\n", "Timestamp", "Agent", "Type", "Data")
	fmt.Println(strings.Repeat("\u2500", 70))

	for _, e := range events {
		ts := e.Timestamp.Format(time.DateTime)
		data := e.Data
		if len(data) > 40 {
			data = data[:37] + "..."
		}
		fmt.Printf("%-22s %-12s %-16s %s\n", ts, e.AgentID, e.Type, data)
	}

	return nil
}
