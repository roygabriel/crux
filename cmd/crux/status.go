package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/roygabriel/crux/internal/config"
	"github.com/roygabriel/crux/internal/memory/session"
	"github.com/roygabriel/crux/internal/phase"
	"github.com/roygabriel/crux/pkg/types"
	"github.com/spf13/cobra"
)

var jsonFlag bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show world state (agents, phases, progress)",
	Long:  "Displays the current or most recent session status including phase progress, agent states, and decision counts.",
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().BoolVar(&jsonFlag, "json", false, "Output as JSON")
}

// StatusOutput is the machine-readable status representation.
type StatusOutput struct {
	Session        SessionInfo                  `json:"session"`
	Phase          string                       `json:"phase"`
	PromptProgress int                          `json:"prompt_progress"`
	Agents         map[string]session.AgentState `json:"agents"`
	PhaseProgress  []PhaseProgressLine          `json:"phase_progress"`
}

// SessionInfo describes the active session.
type SessionInfo struct {
	ID        string    `json:"id"`
	StartedAt time.Time `json:"started_at"`
	Duration  string    `json:"duration"`
}

// PhaseProgressLine describes the progress of a single phase.
type PhaseProgressLine struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
}

func runStatus(cmd *cobra.Command, args []string) error {
	log := setupLogger()

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	sessDir := filepath.Join(cfg.Project.StateDir, "sessions")
	sessionMgr := session.NewManager(sessDir, nil, log)

	sc, err := sessionMgr.ResumeLatest()
	if err != nil {
		if errors.Is(err, session.ErrSessionNotFound) {
			fmt.Println("No sessions found.")
			return nil
		}
		return fmt.Errorf("resume latest session: %w", err)
	}

	var phaseLines []PhaseProgressLine
	var promptTotal int

	engine := loadPhaseEngine(cfg, log)
	if engine != nil {
		phaseLines = buildPhaseProgress(engine, sc)
		if prog, ok := engine.Progress()[types.PhaseID(sc.CurrentPhase)]; ok {
			promptTotal = len(prog.Prompts)
		}
	}

	if jsonFlag {
		return printStatusJSON(sc, phaseLines, promptTotal)
	}

	printStatusFormatted(sc, cfg, phaseLines, promptTotal)
	return nil
}

func loadPhaseEngine(cfg *config.Config, logger *slog.Logger) *phase.Engine {
	gateRunner := phase.NewGateRunner(cfg.Project.Root, 30*time.Second, logger)
	engine, err := phase.NewEngine(cfg.Phases.SpecDir, gateRunner, nil, logger)
	if err != nil {
		logger.Debug("could not create phase engine", "error", err)
		return nil
	}
	if err := engine.LoadAll(); err != nil {
		logger.Debug("could not load phases", "error", err)
		return nil
	}
	return engine
}

func buildPhaseProgress(engine *phase.Engine, sc *session.SessionContext) []PhaseProgressLine {
	order := engine.PhaseOrder()
	progress := engine.Progress()

	lines := make([]PhaseProgressLine, 0, len(order))
	for _, id := range order {
		prog, ok := progress[id]
		if !ok {
			continue
		}

		name := ""
		if prog.Spec != nil {
			name = prog.Spec.Name
		}
		total := len(prog.Prompts)

		var completed int
		var status string

		switch {
		case prog.Spec != nil && prog.Spec.Status == types.PhaseComplete:
			status = "complete"
			completed = total
		case string(id) == sc.CurrentPhase:
			status = "in-progress"
			completed = sc.PromptProgress
		default:
			status = "not-started"
			completed = 0
		}

		lines = append(lines, PhaseProgressLine{
			ID:        string(id),
			Name:      name,
			Status:    status,
			Completed: completed,
			Total:     total,
		})
	}

	return lines
}

func printStatusJSON(sc *session.SessionContext, phaseLines []PhaseProgressLine, promptTotal int) error {
	agents := sc.Agents
	if agents == nil {
		agents = make(map[string]session.AgentState)
	}
	if phaseLines == nil {
		phaseLines = []PhaseProgressLine{}
	}

	output := StatusOutput{
		Session: SessionInfo{
			ID:        sc.ID,
			StartedAt: sc.StartedAt,
			Duration:  formatDuration(time.Since(sc.StartedAt)),
		},
		Phase:          sc.CurrentPhase,
		PromptProgress: sc.PromptProgress,
		Agents:         agents,
		PhaseProgress:  phaseLines,
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal status: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func printStatusFormatted(sc *session.SessionContext, cfg *config.Config, phaseLines []PhaseProgressLine, promptTotal int) {
	fmt.Println("Crux Status")
	fmt.Println("\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550")
	fmt.Printf("Session:  %s (started %s)\n", sc.ID, formatDuration(time.Since(sc.StartedAt)))

	if sc.CurrentPhase != "" {
		phaseName := phaseNameFromLines(phaseLines, sc.CurrentPhase)
		if phaseName != "" {
			fmt.Printf("Phase:    %s \u2014 %s\n", sc.CurrentPhase, phaseName)
		} else {
			fmt.Printf("Phase:    %s\n", sc.CurrentPhase)
		}
	}

	if promptTotal > 0 {
		fmt.Printf("Progress: Prompt %d/%d\n", sc.PromptProgress, promptTotal)
	}

	if len(sc.Agents) > 0 {
		fmt.Println()
		fmt.Println("Agents:")
		agentIDs := sortedKeys(sc.Agents)
		for _, id := range agentIDs {
			agent := sc.Agents[id]
			plugin := ""
			if acfg, ok := cfg.Agents[id]; ok {
				plugin = acfg.Plugin
			}
			indicator := statusIndicator(agent.Status)
			line := fmt.Sprintf("  %-14s %-8s %s %-12s %s", id, plugin, indicator, agent.Status, agent.CurrentTask)
			fmt.Println(strings.TrimRight(line, " "))
		}
	}

	if len(phaseLines) > 0 {
		fmt.Println()
		fmt.Println("Phase Progress:")
		for _, pl := range phaseLines {
			indicator := phaseIndicator(pl.Status)
			fmt.Printf("  %s %-4s %-20s %d/%d\n", indicator, pl.ID, padOrTruncate(pl.Name, 20), pl.Completed, pl.Total)
		}
	}
}

func phaseNameFromLines(lines []PhaseProgressLine, phaseID string) string {
	for _, l := range lines {
		if l.ID == phaseID {
			return l.Name
		}
	}
	return ""
}

func sortedKeys(m map[string]session.AgentState) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func statusIndicator(status string) string {
	switch strings.ToLower(status) {
	case "running", "active", "busy":
		return "\u25cf"
	case "idle", "ready":
		return "\u25cb"
	case "stopped", "error", "failed":
		return "\u2715"
	default:
		return "\u25cb"
	}
}

func phaseIndicator(status string) string {
	switch status {
	case "complete":
		return "\u2713"
	case "in-progress":
		return "\u25c9"
	default:
		return "\u25cb"
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}

	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh ago", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh%dm ago", hours, minutes)
	}
	return fmt.Sprintf("%dm ago", minutes)
}

// padOrTruncate ensures s is exactly n characters wide, padding with spaces or truncating.
func padOrTruncate(s string, n int) string {
	if len(s) >= n {
		return s[:n]
	}
	return s + strings.Repeat(" ", n-len(s))
}
