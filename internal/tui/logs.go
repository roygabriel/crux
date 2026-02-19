package tui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// LogLevel classifies the severity of a log entry.
type LogLevel int

const (
	// LogInfo is an informational message.
	LogInfo LogLevel = iota
	// LogOK indicates a successful operation.
	LogOK
	// LogWarn indicates a warning.
	LogWarn
	// LogError indicates an error.
	LogError
)

// String returns the display label for a LogLevel.
func (l LogLevel) String() string {
	switch l {
	case LogInfo:
		return "INFO"
	case LogOK:
		return "OK"
	case LogWarn:
		return "WARN"
	case LogError:
		return "ERROR"
	default:
		return "INFO"
	}
}

// LogEntry is a single log record for display in the logs panel.
type LogEntry struct {
	Time    time.Time `json:"time"`
	Level   LogLevel  `json:"level"`
	Message string    `json:"message"`
	Source  string    `json:"source"`
}

// Log level display styles.
var (
	logStyleInfo  = lipgloss.NewStyle().Foreground(lipgloss.Color("15")) // white
	logStyleOK    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))  // green
	logStyleWarn  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))  // yellow
	logStyleError = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))  // red

	filterInputStyle = lipgloss.NewStyle().Underline(true)
	filterBadgeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Background(lipgloss.Color("236"))
)

// LogsPanel displays a scrollable circular buffer of log entries.
type LogsPanel struct {
	entries      []LogEntry
	capacity     int
	head         int
	count        int
	autoScroll   bool
	scrollPos    int
	focused      bool
	width        int
	height       int
	filterMode   bool
	filterInput  string
	filterActive string
}

// NewLogsPanel creates a logs panel with the given buffer capacity.
func NewLogsPanel(capacity int) LogsPanel {
	if capacity < 1 {
		capacity = 1
	}
	return LogsPanel{
		entries:    make([]LogEntry, capacity),
		capacity:   capacity,
		autoScroll: true,
	}
}

// Append adds a log entry to the circular buffer.
func (p *LogsPanel) Append(entry LogEntry) {
	p.entries[p.head] = entry
	p.head = (p.head + 1) % p.capacity
	if p.count < p.capacity {
		p.count++
	}
	if p.autoScroll {
		p.scrollPos = 0
	}
}

// ScrollUp moves the view up by n lines and disengages auto-scroll.
func (p *LogsPanel) ScrollUp(n int) {
	p.autoScroll = false
	p.scrollPos += n
	maxScroll := p.count - p.height
	if maxScroll < 0 {
		maxScroll = 0
	}
	if p.scrollPos > maxScroll {
		p.scrollPos = maxScroll
	}
}

// ScrollDown moves the view down by n lines, re-engaging auto-scroll at 0.
func (p *LogsPanel) ScrollDown(n int) {
	p.scrollPos -= n
	if p.scrollPos <= 0 {
		p.scrollPos = 0
		p.autoScroll = true
	}
}

// SetSize sets the panel rendering dimensions.
func (p *LogsPanel) SetSize(w, h int) {
	p.width = w
	p.height = h
}

// SetFocused sets whether this panel is visually focused.
func (p *LogsPanel) SetFocused(focused bool) {
	p.focused = focused
}

// IsFilterMode returns whether the filter input is currently active.
func (p *LogsPanel) IsFilterMode() bool {
	return p.filterMode
}

// ClearFilter clears all filter state.
func (p *LogsPanel) ClearFilter() {
	p.filterActive = ""
	p.filterInput = ""
	p.filterMode = false
}

// HandleKey processes a key press in the logs panel. It returns whether the key
// was handled and an optional command to send.
func (p *LogsPanel) HandleKey(key string) (handled bool, cmd *Command) {
	if p.filterMode {
		return p.handleFilterKey(key), nil
	}
	return p.handleNormalKey(key), nil
}

func (p *LogsPanel) handleNormalKey(key string) bool {
	switch key {
	case "/":
		p.filterMode = true
		p.filterInput = p.filterActive
		return true
	case "g":
		p.autoScroll = false
		p.scrollPos = p.maxScroll()
		return true
	case "G":
		p.scrollPos = 0
		p.autoScroll = true
		return true
	case "up", "k":
		p.ScrollUp(1)
		return true
	case "down", "j":
		p.ScrollDown(1)
		return true
	case "pgup":
		p.ScrollUp(10)
		return true
	case "pgdown":
		p.ScrollDown(10)
		return true
	default:
		return false
	}
}

func (p *LogsPanel) maxScroll() int {
	maxScroll := p.count - p.height
	if maxScroll < 0 {
		maxScroll = 0
	}
	return maxScroll
}

// ModeLabel returns the current interaction mode for panel chrome.
func (p *LogsPanel) ModeLabel() string {
	if p.filterMode {
		return "filtering"
	}
	if p.filterActive != "" {
		return "filtered"
	}
	if p.autoScroll {
		return "live"
	}
	return "scrolled"
}

func (p *LogsPanel) handleFilterKey(key string) bool {
	switch key {
	case "enter":
		p.filterActive = p.filterInput
		p.filterMode = false
		p.scrollPos = 0
		p.autoScroll = true
		return true
	case "esc":
		p.filterInput = ""
		p.filterActive = ""
		p.filterMode = false
		return true
	case "backspace":
		if len(p.filterInput) > 0 {
			_, size := utf8.DecodeLastRuneInString(p.filterInput)
			p.filterInput = p.filterInput[:len(p.filterInput)-size]
		}
		return true
	default:
		if utf8.RuneCountInString(key) == 1 {
			p.filterInput += key
		}
		return true
	}
}

// filteredEntries returns entries matching the filter via case-insensitive
// substring search against level, source, and message.
func filteredEntries(entries []LogEntry, filter string) []LogEntry {
	if filter == "" {
		return entries
	}
	lower := strings.ToLower(filter)
	var result []LogEntry
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.Level.String()), lower) ||
			strings.Contains(strings.ToLower(e.Source), lower) ||
			strings.Contains(strings.ToLower(e.Message), lower) {
			result = append(result, e)
		}
	}
	return result
}

// View renders the visible log entries.
func (p *LogsPanel) View() string {
	if p.count == 0 {
		return "Waiting for logs..."
	}

	ordered := p.orderedEntries()

	// Apply filter if active.
	if p.filterActive != "" {
		ordered = filteredEntries(ordered, p.filterActive)
	}

	// Determine visible window from the bottom (newest).
	visibleHeight := p.height
	if visibleHeight <= 0 {
		visibleHeight = 10
	}

	var b strings.Builder

	// Render filter bar if in filter mode or filter is active.
	if p.filterMode {
		b.WriteString(filterInputStyle.Render(fmt.Sprintf("Filter: %s_", p.filterInput)))
		b.WriteByte('\n')
		visibleHeight--
	} else if p.filterActive != "" {
		b.WriteString(filterBadgeStyle.Render(fmt.Sprintf(" filter: %s ", p.filterActive)))
		b.WriteByte('\n')
		visibleHeight--
	}

	if visibleHeight < 0 {
		visibleHeight = 0
	}

	end := len(ordered) - p.scrollPos
	if end < 0 {
		end = 0
	}
	start := end - visibleHeight
	if start < 0 {
		start = 0
	}

	for i := start; i < end; i++ {
		e := ordered[i]
		b.WriteString(formatLogEntry(e, p.width))
		if i < end-1 {
			b.WriteByte('\n')
		}
	}

	return b.String()
}

// orderedEntries returns entries in chronological order (oldest first).
func (p *LogsPanel) orderedEntries() []LogEntry {
	if p.count == 0 {
		return nil
	}

	result := make([]LogEntry, p.count)
	if p.count < p.capacity {
		// Buffer hasn't wrapped yet.
		copy(result, p.entries[:p.count])
	} else {
		// Buffer has wrapped: oldest is at head, newest is at head-1.
		copy(result, p.entries[p.head:p.capacity])
		copy(result[p.capacity-p.head:], p.entries[:p.head])
	}
	return result
}

// formatLogEntry renders a single log entry as a formatted string.
func formatLogEntry(e LogEntry, width int) string {
	timeStr := e.Time.Format("15:04:05")
	levelStr := styledLevel(e.Level)

	source := ""
	if e.Source != "" {
		source = fmt.Sprintf("(%s) ", e.Source)
	}

	line := fmt.Sprintf("%s %s %s%s", timeStr, levelStr, source, e.Message)

	// Truncate to width if needed.
	if width > 0 && ansi.StringWidthWc(line) > width {
		line = ansi.TruncateWc(line, width, "")
	}

	return line
}

// styledLevel returns a level label with color styling.
func styledLevel(level LogLevel) string {
	label := fmt.Sprintf("[%-5s]", level.String())
	switch level {
	case LogInfo:
		return logStyleInfo.Render(label)
	case LogOK:
		return logStyleOK.Render(label)
	case LogWarn:
		return logStyleWarn.Render(label)
	case LogError:
		return logStyleError.Render(label)
	default:
		return logStyleInfo.Render(label)
	}
}
