package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/roygabriel/crux/pkg/types"
)

// Detail panel styles.
var (
	detailTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("141"))
	detailLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	detailInputStyle = lipgloss.NewStyle().Underline(true)
)

// DetailPanel renders an expanded view of a single agent with message input.
type DetailPanel struct {
	agentID     types.AgentID
	snapshot    *AgentSnapshot
	visible     bool
	inputMode   bool
	inputBuffer string
	sentMsg     string
	width       int
	height      int
}

// Open displays the detail panel for the given agent snapshot.
func (d *DetailPanel) Open(snap *AgentSnapshot) {
	if snap == nil {
		return
	}
	d.agentID = snap.ID
	d.snapshot = snap
	d.visible = true
	d.inputMode = false
	d.inputBuffer = ""
	d.sentMsg = ""
}

// Close hides the detail panel and clears all state.
func (d *DetailPanel) Close() {
	d.visible = false
	d.agentID = ""
	d.snapshot = nil
	d.inputMode = false
	d.inputBuffer = ""
	d.sentMsg = ""
}

// SetSize sets the panel rendering dimensions.
func (d *DetailPanel) SetSize(w, h int) {
	d.width = w
	d.height = h
}

// IsVisible returns whether the detail panel is currently shown.
func (d *DetailPanel) IsVisible() bool {
	return d.visible
}

// IsInputMode returns whether the panel is accepting text input.
func (d *DetailPanel) IsInputMode() bool {
	return d.inputMode
}

// Update refreshes the snapshot data. If snap is nil (agent was killed), the
// panel closes automatically.
func (d *DetailPanel) Update(snap *AgentSnapshot) {
	if snap == nil {
		d.Close()
		return
	}
	d.snapshot = snap
}

// HandleKey processes a key press in the detail panel. It returns whether the
// key was handled and an optional command to send.
func (d *DetailPanel) HandleKey(key string) (handled bool, cmd *Command) {
	if d.inputMode {
		return d.handleInputKey(key)
	}
	return d.handleNormalKey(key)
}

func (d *DetailPanel) handleNormalKey(key string) (bool, *Command) {
	switch key {
	case "esc":
		d.Close()
		return true, nil
	case "m", "/":
		d.inputMode = true
		d.sentMsg = ""
		return true, nil
	default:
		return true, nil
	}
}

func (d *DetailPanel) handleInputKey(key string) (bool, *Command) {
	switch key {
	case "enter":
		if d.inputBuffer == "" {
			return true, nil
		}
		cmd := &Command{
			Type:    CmdSendMessage,
			AgentID: d.agentID,
			Text:    d.inputBuffer,
		}
		d.inputBuffer = ""
		d.sentMsg = "Sent"
		d.inputMode = false
		return true, cmd
	case "esc":
		d.inputBuffer = ""
		d.inputMode = false
		return true, nil
	case "backspace":
		if len(d.inputBuffer) > 0 {
			_, size := utf8.DecodeLastRuneInString(d.inputBuffer)
			d.inputBuffer = d.inputBuffer[:len(d.inputBuffer)-size]
		}
		return true, nil
	default:
		if utf8.RuneCountInString(key) == 1 {
			d.inputBuffer += key
		}
		return true, nil
	}
}

// View renders the full detail overlay.
func (d *DetailPanel) View() string {
	if d.snapshot == nil {
		return ""
	}

	s := d.snapshot
	var b strings.Builder

	// Title bar.
	title := fmt.Sprintf("═══ %s (%s) ", s.ID, s.Plugin)
	remaining := d.width - utf8.RuneCountInString(title)
	if remaining > 0 {
		title += strings.Repeat("═", remaining)
	}
	b.WriteString(detailTitleStyle.Render(title))
	b.WriteByte('\n')

	// Info rows.
	perm := s.Permission
	if perm == "" {
		perm = "—"
	}
	prompt := s.PromptDisplay
	if prompt == "" {
		prompt = "—"
	}
	task := s.Task
	if task == "" {
		task = "—"
	}

	b.WriteString(fmt.Sprintf("%s %s            %s %s\n",
		detailLabelStyle.Render("Role:"), string(s.Role),
		detailLabelStyle.Render("Permission:"), perm))
	b.WriteString(fmt.Sprintf("%s %s %s      %s %s\n",
		detailLabelStyle.Render("Status:"), styledStatusDot(s.Status), string(s.Status),
		detailLabelStyle.Render("Prompt:"), prompt))
	b.WriteString(fmt.Sprintf("%s %s\n",
		detailLabelStyle.Render("Task:"), task))
	b.WriteString(fmt.Sprintf("%s %d       %s %d\n",
		detailLabelStyle.Render("Commands/min:"), s.CommandsPerMin,
		detailLabelStyle.Render("Files/session:"), s.FilesSession))

	b.WriteByte('\n')

	// Decisions section.
	b.WriteString(detailTitleStyle.Render("Recent Decisions:"))
	b.WriteByte('\n')
	if len(s.Decisions) == 0 {
		b.WriteString(detailLabelStyle.Render("  (none)"))
		b.WriteByte('\n')
	} else {
		for _, dec := range s.Decisions {
			b.WriteString(fmt.Sprintf("  • %s\n", dec))
		}
	}

	b.WriteByte('\n')

	// Work notes section.
	b.WriteString(detailTitleStyle.Render("Work Notes:"))
	b.WriteByte('\n')
	if s.WorkNotesInfo == "" {
		b.WriteString(detailLabelStyle.Render("  (none)"))
		b.WriteByte('\n')
	} else {
		for _, line := range strings.Split(s.WorkNotesInfo, "\n") {
			b.WriteString(fmt.Sprintf("  %s\n", line))
		}
	}

	b.WriteByte('\n')

	// Input section.
	inputTitle := "─── Send Message "
	inputRemaining := d.width - utf8.RuneCountInString(inputTitle)
	if inputRemaining > 0 {
		inputTitle += strings.Repeat("─", inputRemaining)
	}
	b.WriteString(detailLabelStyle.Render(inputTitle))
	b.WriteByte('\n')

	inputLine := "> "
	if d.inputMode {
		inputLine += detailInputStyle.Render(d.inputBuffer + "_")
	} else {
		inputLine += detailLabelStyle.Render("(press m to type)")
	}
	if d.sentMsg != "" {
		inputLine += "  " + d.sentMsg
	}
	b.WriteString(inputLine)

	return b.String()
}
