package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/roygabriel/crux/pkg/types"
)

// ContentTab identifies which tab is active in the content panel.
type ContentTab int

const (
	// TabOutput shows live tmux pane content.
	TabOutput ContentTab = iota
	// TabDetails shows agent metadata.
	TabDetails
	// TabDecisions shows recent decisions and work notes.
	TabDecisions
)

// Content panel styles.
var (
	tabActiveStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("141")).Underline(true)
	tabInactiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	contentLabel     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	contentTitle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("141"))
	contentInput     = lipgloss.NewStyle().Underline(true)
)

// ContentPanel renders a tabbed view of agent output, details, and decisions
// with a scrollable viewport and message input.
type ContentPanel struct {
	agent       *AgentSnapshot
	activeTab   ContentTab
	focused     bool
	width       int
	height      int
	scrollPos   int
	inputMode   bool
	inputBuffer string
	sentMsg     string
}

// NewContentPanel creates an empty content panel.
func NewContentPanel() ContentPanel {
	return ContentPanel{}
}

// SetAgent updates the agent displayed in the content panel and resets scroll.
func (p *ContentPanel) SetAgent(snap *AgentSnapshot) {
	changed := p.agent == nil || snap == nil || (p.agent.ID != snap.ID)
	p.agent = snap
	if changed {
		p.scrollPos = 0
		p.inputMode = false
		p.inputBuffer = ""
		p.sentMsg = ""
	}
}

// SetSize sets the panel rendering dimensions.
func (p *ContentPanel) SetSize(w, h int) {
	p.width = w
	p.height = h
}

// SetFocused sets whether this panel is visually focused.
func (p *ContentPanel) SetFocused(focused bool) {
	p.focused = focused
}

// ActiveTab returns the currently selected tab.
func (p *ContentPanel) ActiveTab() ContentTab {
	return p.activeTab
}

// IsInputMode returns whether the panel is accepting text input.
func (p *ContentPanel) IsInputMode() bool {
	return p.inputMode
}

// HandleKey processes a key press when the content panel is focused. It returns
// whether the key was handled and an optional command to send.
func (p *ContentPanel) HandleKey(key string) (handled bool, cmd *Command) {
	if p.inputMode {
		return p.handleInputKey(key)
	}
	return p.handleNormalKey(key)
}

func (p *ContentPanel) handleNormalKey(key string) (bool, *Command) {
	switch key {
	case "1":
		p.activeTab = TabOutput
		p.scrollPos = 0
		return true, nil
	case "2":
		p.activeTab = TabDetails
		p.scrollPos = 0
		return true, nil
	case "3":
		p.activeTab = TabDecisions
		p.scrollPos = 0
		return true, nil
	case "j", "down":
		p.scrollDown(1)
		return true, nil
	case "k", "up":
		p.scrollUp(1)
		return true, nil
	case "pgdown":
		p.scrollDown(10)
		return true, nil
	case "pgup":
		p.scrollUp(10)
		return true, nil
	case "m":
		if p.agent != nil {
			p.inputMode = true
			p.sentMsg = ""
		}
		return true, nil
	default:
		return false, nil
	}
}

func (p *ContentPanel) handleInputKey(key string) (bool, *Command) {
	switch key {
	case "enter":
		if p.inputBuffer == "" {
			return true, nil
		}
		cmd := &Command{
			Type:    CmdSendMessage,
			AgentID: p.agent.ID,
			Text:    p.inputBuffer,
		}
		p.inputBuffer = ""
		p.sentMsg = "Sent"
		p.inputMode = false
		return true, cmd
	case "esc":
		p.inputBuffer = ""
		p.inputMode = false
		return true, nil
	case "backspace":
		if len(p.inputBuffer) > 0 {
			_, size := utf8.DecodeLastRuneInString(p.inputBuffer)
			p.inputBuffer = p.inputBuffer[:len(p.inputBuffer)-size]
		}
		return true, nil
	default:
		if utf8.RuneCountInString(key) == 1 {
			p.inputBuffer += key
		}
		return true, nil
	}
}

func (p *ContentPanel) scrollDown(n int) {
	p.scrollPos += n
	maxScroll := p.contentLineCount() - p.viewportHeight()
	if maxScroll < 0 {
		maxScroll = 0
	}
	if p.scrollPos > maxScroll {
		p.scrollPos = maxScroll
	}
}

func (p *ContentPanel) scrollUp(n int) {
	p.scrollPos -= n
	if p.scrollPos < 0 {
		p.scrollPos = 0
	}
}

func (p *ContentPanel) viewportHeight() int {
	h := p.height - 3 // tab bar + input line + border padding
	if h < 1 {
		return 1
	}
	return h
}

func (p *ContentPanel) contentLineCount() int {
	lines := p.contentLines()
	return len(lines)
}

// View renders the tabbed content panel.
func (p *ContentPanel) View() string {
	var b strings.Builder

	// Tab bar.
	b.WriteString(p.renderTabBar())
	b.WriteByte('\n')

	if p.agent == nil {
		b.WriteString(contentLabel.Render("Select an agent in the sidebar"))
		return b.String()
	}

	// Scrollable content area.
	lines := p.contentLines()
	viewport := p.viewportHeight()

	start := p.scrollPos
	if start > len(lines) {
		start = len(lines)
	}
	end := start + viewport
	if end > len(lines) {
		end = len(lines)
	}

	for i := start; i < end; i++ {
		if p.width > 0 {
			runes := []rune(lines[i])
			if len(runes) > p.width {
				lines[i] = string(runes[:p.width])
			}
		}
		b.WriteString(lines[i])
		if i < end-1 {
			b.WriteByte('\n')
		}
	}

	// Pad remaining viewport lines.
	rendered := end - start
	for i := rendered; i < viewport; i++ {
		b.WriteByte('\n')
	}

	// Scroll indicator.
	if len(lines) > viewport {
		pct := 0
		if len(lines)-viewport > 0 {
			pct = p.scrollPos * 100 / (len(lines) - viewport)
		}
		b.WriteByte('\n')
		b.WriteString(contentLabel.Render(fmt.Sprintf("─── %d%% ───", pct)))
	}

	// Input line.
	b.WriteByte('\n')
	b.WriteString(p.renderInputLine())

	return b.String()
}

func (p *ContentPanel) renderTabBar() string {
	tabs := []struct {
		label string
		tab   ContentTab
	}{
		{"Output", TabOutput},
		{"Details", TabDetails},
		{"Decisions", TabDecisions},
	}

	var parts []string
	for _, t := range tabs {
		label := fmt.Sprintf("[%d] %s", int(t.tab)+1, t.label)
		if t.tab == p.activeTab {
			parts = append(parts, tabActiveStyle.Render(label))
		} else {
			parts = append(parts, tabInactiveStyle.Render(label))
		}
	}
	return strings.Join(parts, "  ")
}

func (p *ContentPanel) renderInputLine() string {
	line := "> "
	if p.inputMode {
		line += contentInput.Render(p.inputBuffer + "_")
	} else {
		line += contentLabel.Render("(press m to type)")
	}
	if p.sentMsg != "" {
		line += "  " + p.sentMsg
	}
	return line
}

func (p *ContentPanel) contentLines() []string {
	if p.agent == nil {
		return nil
	}

	switch p.activeTab {
	case TabOutput:
		return p.outputLines()
	case TabDetails:
		return p.detailLines()
	case TabDecisions:
		return p.decisionLines()
	default:
		return nil
	}
}

func (p *ContentPanel) outputLines() []string {
	if p.agent.PaneContent == "" {
		return []string{"Waiting for output..."}
	}
	return strings.Split(p.agent.PaneContent, "\n")
}

func (p *ContentPanel) detailLines() []string {
	s := p.agent
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

	lines := []string{
		fmt.Sprintf("%s (%s)", s.ID, s.Plugin),
		"",
		fmt.Sprintf("%s %s", contentLabel.Render("Role:"), string(s.Role)),
		fmt.Sprintf("%s %s", contentLabel.Render("Permission:"), perm),
		fmt.Sprintf("%s %s %s", contentLabel.Render("Status:"), styledStatusDot(s.Status), string(s.Status)),
		fmt.Sprintf("%s %s", contentLabel.Render("Prompt:"), prompt),
		fmt.Sprintf("%s %s", contentLabel.Render("Task:"), task),
		fmt.Sprintf("%s %d", contentLabel.Render("Commands/min:"), s.CommandsPerMin),
		fmt.Sprintf("%s %d", contentLabel.Render("Files/session:"), s.FilesSession),
	}
	return lines
}

func (p *ContentPanel) decisionLines() []string {
	s := p.agent
	var lines []string

	lines = append(lines, contentTitle.Render("Recent Decisions"))
	if len(s.Decisions) == 0 {
		lines = append(lines, contentLabel.Render("  (none)"))
	} else {
		for _, d := range s.Decisions {
			lines = append(lines, fmt.Sprintf("  • %s", d))
		}
	}

	lines = append(lines, "")
	lines = append(lines, contentTitle.Render("Work Notes"))
	if s.WorkNotesInfo == "" {
		lines = append(lines, contentLabel.Render("  (none)"))
	} else {
		for _, line := range strings.Split(s.WorkNotesInfo, "\n") {
			lines = append(lines, fmt.Sprintf("  %s", line))
		}
	}

	return lines
}

// agentID returns the ID of the currently displayed agent, or empty if none.
func (p *ContentPanel) agentID() types.AgentID {
	if p.agent == nil {
		return ""
	}
	return p.agent.ID
}
