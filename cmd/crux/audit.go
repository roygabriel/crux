package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/roygabriel/crux/internal/config"
	"github.com/roygabriel/crux/internal/security"
	"github.com/spf13/cobra"
)

var (
	auditLimit     int
	auditAgent     string
	auditDenied    bool
	auditJSONFlag  bool
	auditSinceFlag string
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "View and analyze audit log",
	Long:  "List audit entries with filters or compute statistics.",
}

var auditListCmd = &cobra.Command{
	Use:   "list",
	Short: "List audit log entries",
	RunE:  runAuditList,
}

var auditStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show audit statistics",
	RunE:  runAuditStats,
}

func init() {
	auditListCmd.Flags().IntVar(&auditLimit, "limit", 50, "Maximum entries")
	auditListCmd.Flags().StringVar(&auditAgent, "agent", "", "Filter by agent ID")
	auditListCmd.Flags().BoolVar(&auditDenied, "denied", false, "Show denied entries only")
	auditListCmd.Flags().BoolVar(&auditJSONFlag, "json", false, "Output as JSON")

	auditStatsCmd.Flags().StringVar(&auditSinceFlag, "since", "24h", "Time window (e.g. 1h, 24h, 7d)")

	auditCmd.AddCommand(auditListCmd)
	auditCmd.AddCommand(auditStatsCmd)
}

// AuditStats holds computed audit statistics.
type AuditStats struct {
	Total    int            `json:"total"`
	Allowed  int            `json:"allowed"`
	Denied   int            `json:"denied"`
	ByAgent  map[string]int `json:"by_agent"`
	ByAction map[string]int `json:"by_action"`
}

func runAuditList(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logPath := cfg.Security.AuditLog
	if logPath == "" {
		logPath = filepath.Join(cfg.Project.StateDir, "audit.log")
	}

	entries, err := parseAuditLog(logPath)
	if err != nil {
		return fmt.Errorf("parse audit log: %w", err)
	}

	entries = filterAuditEntries(entries, auditAgent, auditDenied)

	// Take last N entries.
	if auditLimit > 0 && len(entries) > auditLimit {
		entries = entries[len(entries)-auditLimit:]
	}

	if auditJSONFlag {
		return printAuditJSON(entries)
	}

	printAuditList(entries)
	return nil
}

func runAuditStats(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logPath := cfg.Security.AuditLog
	if logPath == "" {
		logPath = filepath.Join(cfg.Project.StateDir, "audit.log")
	}

	entries, err := parseAuditLog(logPath)
	if err != nil {
		return fmt.Errorf("parse audit log: %w", err)
	}

	// Filter by time window.
	dur, err := parseDuration(auditSinceFlag)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", auditSinceFlag, err)
	}
	cutoff := time.Now().Add(-dur)
	var filtered []security.AuditEntry
	for _, e := range entries {
		if e.Timestamp.After(cutoff) {
			filtered = append(filtered, e)
		}
	}

	stats := computeAuditStats(filtered)
	printAuditStats(stats, auditSinceFlag)
	return nil
}

func parseAuditLog(path string) ([]security.AuditEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open audit log: %w", err)
	}
	defer f.Close()

	var entries []security.AuditEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry security.AuditEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue // skip malformed lines
		}
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}

func filterAuditEntries(entries []security.AuditEntry, agent string, deniedOnly bool) []security.AuditEntry {
	if agent == "" && !deniedOnly {
		return entries
	}

	filtered := make([]security.AuditEntry, 0, len(entries))
	for _, e := range entries {
		if agent != "" && string(e.AgentID) != agent {
			continue
		}
		if deniedOnly && e.Allowed {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered
}

func computeAuditStats(entries []security.AuditEntry) AuditStats {
	stats := AuditStats{
		ByAgent:  make(map[string]int),
		ByAction: make(map[string]int),
	}
	for _, e := range entries {
		stats.Total++
		if e.Allowed {
			stats.Allowed++
		} else {
			stats.Denied++
		}
		stats.ByAgent[string(e.AgentID)]++
		stats.ByAction[string(e.Action)]++
	}
	return stats
}

// parseDuration extends time.ParseDuration with support for "Nd" (days).
func parseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		numStr := strings.TrimSuffix(s, "d")
		days, err := strconv.Atoi(numStr)
		if err != nil {
			return 0, fmt.Errorf("invalid day duration %q: %w", s, err)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func printAuditList(entries []security.AuditEntry) {
	if len(entries) == 0 {
		fmt.Println("No audit entries found.")
		return
	}

	fmt.Printf("%-22s %-12s %-14s %-20s %-8s %s\n", "Timestamp", "Agent", "Action", "Target", "Result", "Reason")
	fmt.Println(strings.Repeat("\u2500", 90))

	for _, e := range entries {
		ts := e.Timestamp.Format(time.DateTime)
		result := "allowed"
		if !e.Allowed {
			result = "denied"
		}
		target := e.Target
		if len(target) > 20 {
			target = target[:17] + "..."
		}
		reason := e.Reason
		if len(reason) > 30 {
			reason = reason[:27] + "..."
		}
		fmt.Printf("%-22s %-12s %-14s %-20s %-8s %s\n",
			ts,
			string(e.AgentID),
			string(e.Action),
			target,
			result,
			reason,
		)
	}
}

func printAuditJSON(entries []security.AuditEntry) error {
	if entries == nil {
		entries = []security.AuditEntry{}
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal audit entries: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func printAuditStats(stats AuditStats, since string) {
	fmt.Printf("Audit Statistics (last %s)\n", since)
	fmt.Println(strings.Repeat("\u2550", 40))

	fmt.Printf("Total:   %d\n", stats.Total)
	if stats.Total > 0 {
		allowedPct := float64(stats.Allowed) / float64(stats.Total) * 100
		deniedPct := float64(stats.Denied) / float64(stats.Total) * 100
		fmt.Printf("Allowed: %d (%.1f%%)\n", stats.Allowed, allowedPct)
		fmt.Printf("Denied:  %d (%.1f%%)\n", stats.Denied, deniedPct)
	} else {
		fmt.Printf("Allowed: 0\n")
		fmt.Printf("Denied:  0\n")
	}

	if len(stats.ByAgent) > 0 {
		fmt.Println()
		fmt.Println("By Agent:")
		for agent, count := range stats.ByAgent {
			fmt.Printf("  %-14s %d\n", agent, count)
		}
	}

	if len(stats.ByAction) > 0 {
		fmt.Println()
		fmt.Println("By Action:")
		for action, count := range stats.ByAction {
			fmt.Printf("  %-14s %d\n", action, count)
		}
	}
}
