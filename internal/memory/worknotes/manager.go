// Package worknotes manages per-phase work notes in markdown format.
// It provides structured parsing, rendering, and incremental updates
// for tracking decisions, progress, and session activity.
package worknotes

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// WorkNotes represents the structured content of a phase work-notes file.
type WorkNotes struct {
	// PhaseID is the phase identifier.
	PhaseID string
	// PhaseName is the human-readable phase name.
	PhaseName string
	// Status is the current phase status (Not started, In progress, Blocked, Complete).
	Status string
	// Decisions records key decisions and their rationale.
	Decisions []DecisionEntry
	// Assumptions lists assumptions made during the phase.
	Assumptions []string
	// OpenQuestions tracks questions that need answers.
	OpenQuestions []QuestionEntry
	// SessionLog records per-session activity.
	SessionLog []SessionLogEntry
	// PromptProgress tracks completion of individual prompts.
	PromptProgress []PromptStatus
	// Commits lists git commits associated with this phase.
	Commits []CommitEntry
}

// DecisionEntry pairs a decision with its rationale.
type DecisionEntry struct {
	// Decision is the choice made.
	Decision string
	// Rationale explains why.
	Rationale string
}

// QuestionEntry tracks an open question and its resolution status.
type QuestionEntry struct {
	// Question is the text of the question.
	Question string
	// Resolved indicates whether the question has been answered.
	Resolved bool
}

// SessionLogEntry records activity from a single work session.
type SessionLogEntry struct {
	// Timestamp is the session date/time in "YYYY-MM-DD HH:MM" format.
	Timestamp string
	// Changed describes what was modified.
	Changed string
	// Why explains the reason for changes.
	Why string
	// Blockers lists any blocking issues.
	Blockers string
	// Next describes planned next steps.
	Next string
}

// PromptStatus tracks the completion state of a numbered prompt.
type PromptStatus struct {
	// Number is the prompt number (1-based).
	Number int
	// Complete indicates whether the prompt is done.
	Complete bool
}

// CommitEntry records a git commit hash and message.
type CommitEntry struct {
	// Hash is the commit hash (short or full).
	Hash string
	// Message is the commit message.
	Message string
}

// Manager handles reading, writing, and updating work-notes files.
type Manager struct {
	notesDir string
	logger   *slog.Logger
}

// NewManager creates a Manager that stores work-notes files in the given directory.
func NewManager(notesDir string, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{notesDir: notesDir, logger: logger}
}

// Init creates a work-notes file for the given phase from the template.
// It is idempotent — if the file already exists, it does nothing.
func (m *Manager) Init(phaseID, phaseName string) error {
	path := m.filePath(phaseID)
	if _, err := os.Stat(path); err == nil {
		return nil // already exists
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating notes dir: %w", err)
	}

	notes := &WorkNotes{
		PhaseID:   phaseID,
		PhaseName: phaseName,
		Status:    "Not started",
	}
	return m.write(path, notes)
}

// Read parses the work-notes markdown file for the given phase.
func (m *Manager) Read(phaseID string) (*WorkNotes, error) {
	path := m.filePath(phaseID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading work notes: %w", err)
	}
	return parse(string(data))
}

// AppendDecision adds a decision entry to the phase work-notes.
func (m *Manager) AppendDecision(phaseID, decision, rationale string) error {
	notes, err := m.Read(phaseID)
	if err != nil {
		return err
	}
	notes.Decisions = append(notes.Decisions, DecisionEntry{
		Decision:  decision,
		Rationale: rationale,
	})
	return m.write(m.filePath(phaseID), notes)
}

// AppendSession adds a session log entry to the phase work-notes.
func (m *Manager) AppendSession(phaseID string, entry SessionLogEntry) error {
	notes, err := m.Read(phaseID)
	if err != nil {
		return err
	}
	notes.SessionLog = append(notes.SessionLog, entry)
	return m.write(m.filePath(phaseID), notes)
}

// UpdatePromptProgress sets the completion status of a specific prompt.
func (m *Manager) UpdatePromptProgress(phaseID string, promptNum int, complete bool) error {
	notes, err := m.Read(phaseID)
	if err != nil {
		return err
	}

	found := false
	for i := range notes.PromptProgress {
		if notes.PromptProgress[i].Number == promptNum {
			notes.PromptProgress[i].Complete = complete
			found = true
			break
		}
	}
	if !found {
		notes.PromptProgress = append(notes.PromptProgress, PromptStatus{
			Number:   promptNum,
			Complete: complete,
		})
	}
	return m.write(m.filePath(phaseID), notes)
}

// UpdateStatus sets the phase status, checking the matching box and unchecking others.
func (m *Manager) UpdateStatus(phaseID string, status string) error {
	notes, err := m.Read(phaseID)
	if err != nil {
		return err
	}
	notes.Status = status
	return m.write(m.filePath(phaseID), notes)
}

// Render converts a WorkNotes struct back into the markdown template format.
func (m *Manager) Render(notes *WorkNotes) string {
	return render(notes)
}

func (m *Manager) filePath(phaseID string) string {
	return filepath.Join(m.notesDir, fmt.Sprintf("PHASE%s.md", phaseID))
}

func (m *Manager) write(path string, notes *WorkNotes) error {
	content := render(notes)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing work notes: %w", err)
	}
	return nil
}

// render produces the markdown representation of a WorkNotes struct.
func render(notes *WorkNotes) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Phase %s - %s\n", notes.PhaseID, notes.PhaseName))

	// Status section.
	b.WriteString("\n## Status\n")
	statuses := []string{"Not started", "In progress", "Blocked", "Complete"}
	for _, s := range statuses {
		if s == notes.Status {
			b.WriteString(fmt.Sprintf("- [x] %s\n", s))
		} else {
			b.WriteString(fmt.Sprintf("- [ ] %s\n", s))
		}
	}

	// Decisions section.
	b.WriteString("\n## Decisions\n")
	if len(notes.Decisions) == 0 {
		b.WriteString("- <Decision>: <Rationale>\n")
	} else {
		for _, d := range notes.Decisions {
			b.WriteString(fmt.Sprintf("- %s: %s\n", d.Decision, d.Rationale))
		}
	}

	// Assumptions section.
	b.WriteString("\n## Assumptions\n")
	if len(notes.Assumptions) == 0 {
		b.WriteString("- <Assumption>\n")
	} else {
		for _, a := range notes.Assumptions {
			b.WriteString(fmt.Sprintf("- %s\n", a))
		}
	}

	// Open Questions section.
	b.WriteString("\n## Open Questions\n")
	if len(notes.OpenQuestions) == 0 {
		b.WriteString("- [ ] <Question>\n")
	} else {
		for _, q := range notes.OpenQuestions {
			if q.Resolved {
				b.WriteString(fmt.Sprintf("- [x] %s\n", q.Question))
			} else {
				b.WriteString(fmt.Sprintf("- [ ] %s\n", q.Question))
			}
		}
	}

	// Session Log section.
	b.WriteString("\n## Session Log\n")
	if len(notes.SessionLog) == 0 {
		b.WriteString("\n### <YYYY-MM-DD HH:MM>\n")
		b.WriteString("- What changed:\n")
		b.WriteString("- Why:\n")
		b.WriteString("- Blockers:\n")
		b.WriteString("- Next:\n")
	} else {
		for _, s := range notes.SessionLog {
			b.WriteString(fmt.Sprintf("\n### %s\n", s.Timestamp))
			b.WriteString(fmt.Sprintf("- What changed: %s\n", s.Changed))
			b.WriteString(fmt.Sprintf("- Why: %s\n", s.Why))
			b.WriteString(fmt.Sprintf("- Blockers: %s\n", s.Blockers))
			b.WriteString(fmt.Sprintf("- Next: %s\n", s.Next))
		}
	}

	// Prompt Progress section.
	b.WriteString("\n## Prompt Progress\n")
	if len(notes.PromptProgress) == 0 {
		b.WriteString("- [ ] Prompt 1\n")
		b.WriteString("- [ ] Prompt 2\n")
	} else {
		for _, p := range notes.PromptProgress {
			if p.Complete {
				b.WriteString(fmt.Sprintf("- [x] Prompt %d\n", p.Number))
			} else {
				b.WriteString(fmt.Sprintf("- [ ] Prompt %d\n", p.Number))
			}
		}
	}

	// Commits section.
	b.WriteString("\n## Commits\n")
	if len(notes.Commits) == 0 {
		b.WriteString("- <hash> - <message>\n")
	} else {
		for _, c := range notes.Commits {
			b.WriteString(fmt.Sprintf("- %s - %s\n", c.Hash, c.Message))
		}
	}

	return b.String()
}

// Section names for the state machine parser.
const (
	sectionNone           = ""
	sectionStatus         = "Status"
	sectionDecisions      = "Decisions"
	sectionAssumptions    = "Assumptions"
	sectionOpenQuestions   = "Open Questions"
	sectionSessionLog     = "Session Log"
	sectionPromptProgress = "Prompt Progress"
	sectionCommits        = "Commits"
)

// parse converts markdown text into a WorkNotes struct using a line-by-line
// state machine with section headers as transitions.
func parse(text string) (*WorkNotes, error) {
	notes := &WorkNotes{}
	lines := strings.Split(text, "\n")
	currentSection := sectionNone
	var currentSession *SessionLogEntry

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Phase header.
		if strings.HasPrefix(trimmed, "# Phase ") {
			rest := strings.TrimPrefix(trimmed, "# Phase ")
			if idx := strings.Index(rest, " - "); idx >= 0 {
				notes.PhaseID = rest[:idx]
				notes.PhaseName = rest[idx+3:]
			}
			continue
		}

		// Section headers (## ).
		if strings.HasPrefix(trimmed, "## ") {
			// Flush any pending session log entry.
			if currentSession != nil {
				notes.SessionLog = append(notes.SessionLog, *currentSession)
				currentSession = nil
			}
			section := strings.TrimPrefix(trimmed, "## ")
			currentSection = section
			continue
		}

		// Session log sub-headers (### ).
		if strings.HasPrefix(trimmed, "### ") && currentSection == sectionSessionLog {
			if currentSession != nil {
				notes.SessionLog = append(notes.SessionLog, *currentSession)
			}
			ts := strings.TrimPrefix(trimmed, "### ")
			if isPlaceholder(ts) {
				currentSession = nil
				continue
			}
			currentSession = &SessionLogEntry{Timestamp: ts}
			continue
		}

		// Parse content lines based on current section.
		switch currentSection {
		case sectionStatus:
			parseStatusLine(trimmed, notes)
		case sectionDecisions:
			parseDecisionLine(trimmed, notes)
		case sectionAssumptions:
			parseAssumptionLine(trimmed, notes)
		case sectionOpenQuestions:
			parseQuestionLine(trimmed, notes)
		case sectionSessionLog:
			if currentSession != nil {
				parseSessionField(trimmed, currentSession)
			}
		case sectionPromptProgress:
			parsePromptLine(trimmed, notes)
		case sectionCommits:
			parseCommitLine(trimmed, notes)
		}
	}

	// Flush final session entry.
	if currentSession != nil {
		notes.SessionLog = append(notes.SessionLog, *currentSession)
	}

	return notes, nil
}

func parseStatusLine(line string, notes *WorkNotes) {
	if strings.HasPrefix(line, "- [x] ") {
		notes.Status = strings.TrimPrefix(line, "- [x] ")
	}
}

func parseDecisionLine(line string, notes *WorkNotes) {
	if !strings.HasPrefix(line, "- ") {
		return
	}
	content := strings.TrimPrefix(line, "- ")
	if isPlaceholder(content) {
		return
	}
	if idx := strings.Index(content, ": "); idx >= 0 {
		notes.Decisions = append(notes.Decisions, DecisionEntry{
			Decision:  content[:idx],
			Rationale: content[idx+2:],
		})
	}
}

func parseAssumptionLine(line string, notes *WorkNotes) {
	if !strings.HasPrefix(line, "- ") {
		return
	}
	content := strings.TrimPrefix(line, "- ")
	if isPlaceholder(content) {
		return
	}
	notes.Assumptions = append(notes.Assumptions, content)
}

func parseQuestionLine(line string, notes *WorkNotes) {
	if strings.HasPrefix(line, "- [x] ") {
		q := strings.TrimPrefix(line, "- [x] ")
		if !isPlaceholder(q) {
			notes.OpenQuestions = append(notes.OpenQuestions, QuestionEntry{Question: q, Resolved: true})
		}
	} else if strings.HasPrefix(line, "- [ ] ") {
		q := strings.TrimPrefix(line, "- [ ] ")
		if !isPlaceholder(q) {
			notes.OpenQuestions = append(notes.OpenQuestions, QuestionEntry{Question: q, Resolved: false})
		}
	}
}

func parseSessionField(line string, entry *SessionLogEntry) {
	if strings.HasPrefix(line, "- What changed:") {
		entry.Changed = strings.TrimSpace(strings.TrimPrefix(line, "- What changed:"))
	} else if strings.HasPrefix(line, "- Why:") {
		entry.Why = strings.TrimSpace(strings.TrimPrefix(line, "- Why:"))
	} else if strings.HasPrefix(line, "- Blockers:") {
		entry.Blockers = strings.TrimSpace(strings.TrimPrefix(line, "- Blockers:"))
	} else if strings.HasPrefix(line, "- Next:") {
		entry.Next = strings.TrimSpace(strings.TrimPrefix(line, "- Next:"))
	}
}

func parsePromptLine(line string, notes *WorkNotes) {
	if strings.HasPrefix(line, "- [x] Prompt ") {
		numStr := strings.TrimPrefix(line, "- [x] Prompt ")
		num := parsePromptNum(numStr)
		if num > 0 {
			notes.PromptProgress = append(notes.PromptProgress, PromptStatus{Number: num, Complete: true})
		}
	} else if strings.HasPrefix(line, "- [ ] Prompt ") {
		numStr := strings.TrimPrefix(line, "- [ ] Prompt ")
		num := parsePromptNum(numStr)
		if num > 0 {
			notes.PromptProgress = append(notes.PromptProgress, PromptStatus{Number: num, Complete: false})
		}
	}
}

func parseCommitLine(line string, notes *WorkNotes) {
	if !strings.HasPrefix(line, "- ") {
		return
	}
	content := strings.TrimPrefix(line, "- ")
	if isPlaceholder(content) {
		return
	}
	if idx := strings.Index(content, " - "); idx >= 0 {
		notes.Commits = append(notes.Commits, CommitEntry{
			Hash:    content[:idx],
			Message: content[idx+3:],
		})
	}
}

func parsePromptNum(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	return n
}

func isPlaceholder(s string) bool {
	return strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">")
}
