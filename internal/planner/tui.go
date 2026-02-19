package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/roygabriel/crux/internal/ui/chrome"
)

// TUI styles.
var (
	userStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("141"))
	toolStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	inputBarStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("248"))
)

// Minimum debounce interval for glamour re-renders during streaming.
const glamourDebounce = 200 * time.Millisecond

// maxAutoContinues is the maximum number of transparent auto-continue attempts
// per user-initiated turn when the default max_tokens limit truncates a response.
const maxAutoContinues = 3

// chatMessage represents a single message in the conversation display.
type chatMessage struct {
	role     string // "user", "assistant", "tool"
	content  string // raw content
	rendered string // glamour-rendered content (assistant only)
}

// --- tea.Msg types ---

// StreamDoneMsg signals that a streaming response has completed.
type StreamDoneMsg struct{}

// StreamTruncatedMsg signals that the response was truncated by the default
// max_tokens limit and may be auto-continued.
type StreamTruncatedMsg struct{}

// StreamErrMsg signals a streaming error.
type StreamErrMsg struct {
	Err error
}

// ToolUseMsg signals the model has invoked a tool.
type ToolUseMsg struct {
	Chunk ToolUseChunk
}

// ToolResultMsg carries the result of a tool execution back to the model.
type ToolResultMsg struct {
	ToolID  string
	Result  string
	IsError bool
}

// initialSendMsg is dispatched by Init when an initial message is configured.
type initialSendMsg struct {
	text string
}

type planPanel int

const (
	panelTimeline planPanel = iota
	panelChat
	panelStatus
	panelInput
)

// TUIModel is the bubbletea model for the interactive planning conversation.
type TUIModel struct {
	agent       *Agent
	projectRoot string
	theme       chrome.Theme

	messages      []chatMessage
	streaming     bool
	streamBuf     *strings.Builder
	phaseCount    int
	continueCount int
	initialMsg    string

	generating      bool   // true while generate_single_phase calls are in flight
	genCompleted    int    // phases successfully generated so far
	genCurrentPhase string // phase ID currently being generated (e.g. "1A")
	genInFlight     bool   // true between a generate_single_phase ToolUseMsg and its ToolResultMsg

	viewport    viewport.Model // center chat viewport
	input       textarea.Model
	spinner     spinner.Model
	activePanel planPanel
	leftScroll  int
	rightScroll int
	showHelp    bool
	width       int
	height      int
	ready       bool
	err         error

	renderer   *glamour.TermRenderer
	lastRender time.Time
}

// NewTUIModel creates a new planning TUI model.
func NewTUIModel(agent *Agent, projectRoot string) TUIModel {
	ta := textarea.New()
	ta.Placeholder = "Describe your project or ask a question..."
	ta.Prompt = "┃ "
	ta.ShowLineNumbers = false
	ta.SetHeight(3)
	ta.CharLimit = 4096
	ta.Focus()

	vp := viewport.New(80, 20)

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithWordWrap(76),
		glamour.WithStylePath("dark"),
	)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	return TUIModel{
		agent:       agent,
		projectRoot: projectRoot,
		theme:       chrome.NewTheme(),
		streamBuf:   &strings.Builder{},
		viewport:    vp,
		input:       ta,
		spinner:     sp,
		renderer:    renderer,
		activePanel: panelInput,
	}
}

// SetInitialMessage configures a message to be sent automatically when the
// TUI starts. This is used by --from-description to seed the conversation.
func (m *TUIModel) SetInitialMessage(msg string) {
	m.initialMsg = msg
}

// Init implements tea.Model.
func (m TUIModel) Init() tea.Cmd {
	cmds := []tea.Cmd{textarea.Blink}
	if m.initialMsg != "" {
		text := m.initialMsg
		cmds = append(cmds, func() tea.Msg {
			return initialSendMsg{text: text}
		})
	}
	return tea.Batch(cmds...)
}

// Update implements tea.Model.
func (m TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalcSizes()
		m.ready = true
		m.refreshViewport()

	case streamBatchMsg:
		m.streamBuf.WriteString(msg.text)
		if time.Since(m.lastRender) > glamourDebounce {
			m.refreshViewport()
			m.lastRender = time.Now()
		}
		ch := msg.ch
		cmds = append(cmds, func() tea.Msg {
			return readStreamMsg(ch)
		})

	case StreamDoneMsg:
		m.streaming = false
		if m.generating {
			m.generating = false
			m.phaseCount = m.genCompleted
		}
		// Finalize the assistant message with glamour rendering.
		raw := m.streamBuf.String()
		m.streamBuf.Reset()
		if raw != "" {
			rendered := m.renderMarkdown(raw)
			m.messages = append(m.messages, chatMessage{
				role:     "assistant",
				content:  raw,
				rendered: rendered,
			})
		}
		m.refreshViewport()
		cmds = append(cmds, m.input.Focus())

	case StreamTruncatedMsg:
		// Finalize partial response as an assistant message.
		raw := m.streamBuf.String()
		m.streamBuf.Reset()
		if raw != "" {
			rendered := m.renderMarkdown(raw)
			m.messages = append(m.messages, chatMessage{
				role:     "assistant",
				content:  raw,
				rendered: rendered,
			})
		}

		if m.continueCount < maxAutoContinues {
			m.continueCount++
			m.refreshViewport()
			cmds = append(cmds, m.autoContinueCmd())
		} else {
			m.streaming = false
			m.messages = append(m.messages, chatMessage{
				role:    "assistant",
				content: "\n\n---\n*[Response reached token limit. Send a follow-up message to continue.]*\n",
			})
			m.refreshViewport()
			cmds = append(cmds, m.input.Focus())
		}

	case StreamErrMsg:
		m.streaming = false
		m.streamBuf.Reset()
		m.err = msg.Err
		m.messages = append(m.messages, chatMessage{
			role:    "error",
			content: formatAPIError(msg.Err),
		})
		m.refreshViewport()
		cmds = append(cmds, m.input.Focus())

	case ToolUseMsg:
		if msg.Chunk.Name == "generate_single_phase" {
			var input struct {
				ID string `json:"id"`
			}
			_ = json.Unmarshal(msg.Chunk.Input, &input)
			m.generating = true
			m.genInFlight = true
			m.genCurrentPhase = input.ID
			m.messages = append(m.messages, chatMessage{
				role:    "tool",
				content: fmt.Sprintf("[generating phase %s... (%d completed)]", input.ID, m.genCompleted),
			})
		} else {
			m.messages = append(m.messages, chatMessage{
				role:    "tool",
				content: fmt.Sprintf("[tool: %s]", msg.Chunk.Name),
			})
		}
		m.refreshViewport()
		// Execute the tool and send the result back.
		cmds = append(cmds, m.executeToolCmd(msg.Chunk))

	case ToolResultMsg:
		if m.generating && m.genInFlight {
			if !msg.IsError {
				m.genCompleted++
			} else {
				m.messages = append(m.messages, chatMessage{
					role:    "error",
					content: fmt.Sprintf("Phase %s generation failed.", m.genCurrentPhase),
				})
			}
			m.genInFlight = false
		}
		cmds = append(cmds, m.handleToolResultCmd(msg))

	case spinner.TickMsg:
		if m.streaming {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case initialSendMsg:
		return m, m.sendMessageCmd(msg.text)
	}

	// Update sub-models.
	if !m.streaming {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	if vpCmd != nil {
		cmds = append(cmds, vpCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m TUIModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "q", "ctrl+c", "esc":
		return m, tea.Quit

	case "?":
		m.showHelp = !m.showHelp
		return m, nil

	case "tab":
		if !m.streaming {
			m.activePanel = (m.activePanel + 1) % 4
			if m.activePanel == panelInput {
				return m, m.input.Focus()
			}
			m.input.Blur()
		}
		return m, nil

	case "shift+tab", "backtab":
		if !m.streaming {
			m.activePanel--
			if m.activePanel < panelTimeline {
				m.activePanel = panelInput
			}
			if m.activePanel == panelInput {
				return m, m.input.Focus()
			}
			m.input.Blur()
		}
		return m, nil

	case "ctrl+n":
		m.agent.Reset()
		m.messages = nil
		m.streaming = false
		m.streamBuf.Reset()
		m.phaseCount = 0
		m.continueCount = 0
		m.generating = false
		m.genCompleted = 0
		m.genCurrentPhase = ""
		m.genInFlight = false
		m.err = nil
		m.leftScroll = 0
		m.rightScroll = 0
		m.refreshViewport()
		return m, m.input.Focus()

	case "ctrl+g":
		if !m.streaming {
			return m, m.sendMessageCmd("The plan is approved. Please generate all phase files now using the generate_single_phase tool, one phase at a time.")
		}
		return m, nil

	case "j", "down":
		m.scrollActive(1)
		return m, nil

	case "k", "up":
		m.scrollActive(-1)
		return m, nil

	case "pgdown":
		m.scrollActive(8)
		return m, nil

	case "pgup":
		m.scrollActive(-8)
		return m, nil

	case "enter":
		if m.streaming || m.activePanel != panelInput {
			return m, nil
		}
		val := strings.TrimSpace(m.input.Value())
		if val == "" {
			return m, nil
		}
		m.input.Reset()
		return m, m.sendMessageCmd(val)
	}

	// Pass other keys to textarea when the input dock is focused.
	if !m.streaming && m.activePanel == panelInput {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

// sendMessageCmd creates a tea.Cmd that sends a user message to the agent
// and pipes streaming chunks back as tea.Msg values.
func (m *TUIModel) sendMessageCmd(text string) tea.Cmd {
	m.messages = append(m.messages, chatMessage{role: "user", content: text})
	m.streaming = true
	m.streamBuf.Reset()
	m.continueCount = 0
	m.err = nil
	m.input.Blur()
	m.refreshViewport()

	agent := m.agent
	sendCmd := func() tea.Msg {
		ch, err := agent.SendMessage(context.Background(), text)
		if err != nil {
			return StreamErrMsg{Err: err}
		}
		return readStreamMsg(ch)
	}
	return tea.Batch(sendCmd, m.spinner.Tick)
}

// readStreamMsg reads one chunk from the stream channel and returns the
// appropriate tea.Msg. It returns a tea.Cmd to continue reading if not done.
func readStreamMsg(ch <-chan StreamChunk) tea.Msg {
	chunk, ok := <-ch
	if !ok {
		return StreamDoneMsg{}
	}
	if chunk.Err != nil {
		return StreamErrMsg{Err: chunk.Err}
	}
	if chunk.Truncated {
		return StreamTruncatedMsg{}
	}
	if chunk.ToolUse != nil {
		return ToolUseMsg{Chunk: *chunk.ToolUse}
	}
	if chunk.Done {
		return StreamDoneMsg{}
	}
	return streamBatchMsg{text: chunk.Text, ch: ch}
}

// streamBatchMsg carries a text delta plus the channel for the next read.
type streamBatchMsg struct {
	text string
	ch   <-chan StreamChunk
}

// executeToolCmd executes a tool and returns a ToolResultMsg.
func (m *TUIModel) executeToolCmd(chunk ToolUseChunk) tea.Cmd {
	root := m.projectRoot
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), streamTimeout)
		defer cancel()
		result, isError := ExecuteTool(ctx, root, chunk.Name, chunk.Input)
		return ToolResultMsg{
			ToolID:  chunk.ID,
			Result:  result,
			IsError: isError,
		}
	}
}

// handleToolResultCmd sends the tool result to the agent and starts a new stream.
func (m *TUIModel) handleToolResultCmd(msg ToolResultMsg) tea.Cmd {
	m.streaming = true
	m.streamBuf.Reset()
	m.continueCount = 0
	agent := m.agent
	sendCmd := func() tea.Msg {
		ch, err := agent.HandleToolResult(context.Background(), msg.ToolID, msg.Result, msg.IsError)
		if err != nil {
			return StreamErrMsg{Err: err}
		}
		return readStreamMsg(ch)
	}
	return tea.Batch(sendCmd, m.spinner.Tick)
}

// autoContinueCmd sends an invisible follow-up message to resume a truncated
// response. It does not append a visible user message to m.messages.
func (m *TUIModel) autoContinueCmd() tea.Cmd {
	m.streaming = true
	m.streamBuf.Reset()
	agent := m.agent
	sendCmd := func() tea.Msg {
		ch, err := agent.SendMessage(context.Background(), "Your previous response was cut short by the token limit. If you were about to call a tool, please call it now. If you were generating phase docs, use the generate_single_phase tool to generate one phase at a time.")
		if err != nil {
			return StreamErrMsg{Err: err}
		}
		return readStreamMsg(ch)
	}
	return tea.Batch(sendCmd, m.spinner.Tick)
}

// View implements tea.Model.
func (m TUIModel) View() string {
	if !m.ready {
		return "Initializing planning board..."
	}
	if m.showHelp {
		return m.helpView()
	}

	bodyHeight := m.bodyHeight()
	leftW, centerW, rightW := m.columnWidths()
	leftInnerW := maxInt(0, leftW-4)
	centerInnerW := maxInt(0, centerW-4)
	rightInnerW := maxInt(0, rightW-4)
	panelInnerH := maxInt(0, bodyHeight-4)

	leftView := m.renderPanel("Timeline", m.panelMeta(panelTimeline), m.timelineView(panelInnerH), leftInnerW, panelInnerH, m.activePanel == panelTimeline)
	centerView := m.renderPanel("Conversation", m.panelMeta(panelChat), m.viewport.View(), centerInnerW, panelInnerH, m.activePanel == panelChat)
	rightView := m.renderPanel("Run Status", m.panelMeta(panelStatus), m.statusPaneView(panelInnerH), rightInnerW, panelInnerH, m.activePanel == panelStatus)
	boards := lipgloss.JoinHorizontal(lipgloss.Top, leftView, centerView, rightView)

	var b strings.Builder
	b.WriteString(m.statusBar())
	b.WriteByte('\n')
	b.WriteString(boards)
	b.WriteByte('\n')
	b.WriteString(m.inputView())
	b.WriteByte('\n')
	b.WriteString(m.commandLegend())
	return b.String()
}

func (m TUIModel) inputView() string {
	if m.streaming && m.generating {
		return inputBarStyle.Render(fmt.Sprintf(" %s generating phases (%d complete, writing %s)", m.spinner.View(), m.genCompleted, m.genCurrentPhase))
	}
	if m.streaming {
		return inputBarStyle.Render(fmt.Sprintf(" %s thinking...", m.spinner.View()))
	}
	if m.err != nil {
		return m.input.View() + "\n" + errorStyle.Render("  ⚠ Error occurred — see above. Type to continue.")
	}
	prefix := "input"
	if m.activePanel == panelInput {
		prefix = "input*"
	}
	return inputBarStyle.Render(" "+prefix) + "\n" + m.input.View()
}

func (m TUIModel) statusBar() string {
	left := "Planning"
	if m.generating {
		left += fmt.Sprintf(" | generating %d complete (%s)", m.genCompleted, m.genCurrentPhase)
	} else if m.phaseCount > 0 {
		left += fmt.Sprintf(" | phases %d", m.phaseCount)
	}

	center := fmt.Sprintf("messages %d", len(m.messages))
	if m.streaming {
		center = "streaming"
	}
	right := "q quit"
	return m.theme.RenderHeaderBar(m.width, left, center, right)
}

func (m TUIModel) commandLegend() string {
	items := []chrome.LegendItem{
		{Key: "q", Action: "quit"},
		{Key: "?", Action: "help"},
		{Key: "tab", Action: "focus"},
		{Key: "ctrl+g", Action: "generate"},
		{Key: "ctrl+n", Action: "reset"},
	}
	switch m.activePanel {
	case panelTimeline, panelChat, panelStatus:
		items = append(items,
			chrome.LegendItem{Key: "j/k", Action: "scroll"},
			chrome.LegendItem{Key: "pgup/pgdn", Action: "page"},
		)
	case panelInput:
		items = append(items, chrome.LegendItem{Key: "enter", Action: "send"})
	}
	return m.theme.RenderLegend(m.width, m.panelMeta(m.activePanel), items)
}

// recalcSizes adjusts component dimensions after a window resize.
func (m *TUIModel) recalcSizes() {
	inputHeight := 4 // input label + textarea
	statusHeight := 1
	legendHeight := 1
	vpHeight := m.height - inputHeight - statusHeight - legendHeight
	if vpHeight < 3 {
		vpHeight = 3
	}

	_, centerW, _ := m.columnWidths()
	innerW := centerW - 6
	if innerW < 20 {
		innerW = 20
	}
	m.viewport.Width = innerW
	m.viewport.Height = maxInt(1, vpHeight-2)
	m.input.SetWidth(m.width - 2)

	// Recreate the renderer with the new width.
	wordWrap := m.width - 4
	if wordWrap < 20 {
		wordWrap = 20
	}
	m.renderer, _ = glamour.NewTermRenderer(
		glamour.WithWordWrap(wordWrap),
		glamour.WithStylePath("dark"),
	)
}

// refreshViewport rebuilds the viewport content from messages and streaming buffer.
func (m *TUIModel) refreshViewport() {
	var b strings.Builder

	for _, msg := range m.messages {
		switch msg.role {
		case "user":
			b.WriteString(userStyle.Render("You: "))
			b.WriteString(msg.content)
			b.WriteString("\n\n")
		case "assistant":
			if msg.rendered != "" {
				b.WriteString(msg.rendered)
			} else {
				b.WriteString(msg.content)
			}
			b.WriteByte('\n')
		case "tool":
			b.WriteString(toolStyle.Render(msg.content))
			b.WriteString("\n\n")
		case "error":
			b.WriteString(errorStyle.Render(msg.content))
			b.WriteString("\n\n")
		}
	}

	// Show streaming buffer if active.
	if m.streaming && m.streamBuf.Len() > 0 {
		b.WriteString(m.streamBuf.String())
	}

	m.viewport.SetContent(b.String())
	if m.activePanel == panelChat {
		m.viewport.GotoBottom()
	}
}

// renderMarkdown renders markdown text using glamour.
func (m *TUIModel) renderMarkdown(text string) string {
	if m.renderer == nil || text == "" {
		return text
	}
	rendered, err := m.renderer.Render(text)
	if err != nil {
		return text
	}
	return strings.TrimSpace(rendered)
}

// formatAPIError returns a user-friendly error message with the raw error details.
func formatAPIError(err error) string {
	msg := err.Error()
	var friendly string

	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "context canceled"):
		friendly = "Request was interrupted."
	case strings.Contains(lower, "context deadline exceeded"):
		friendly = "Request timed out — the API did not respond in time."
	case strings.Contains(lower, "401") || strings.Contains(lower, "unauthorized"):
		friendly = "Authentication failed — check your API key."
	case strings.Contains(lower, "rate limit") || strings.Contains(lower, "429"):
		friendly = "Rate limited — too many requests. Wait a moment and try again."
	case strings.Contains(lower, "overloaded") || strings.Contains(lower, "529"):
		friendly = "API is overloaded — try again shortly."
	case strings.Contains(lower, "connection refused") || strings.Contains(lower, "no such host"):
		friendly = "Cannot reach the API — check your network connection."
	case strings.Contains(lower, "500") || strings.Contains(lower, "internal server error"):
		friendly = "API server error — this is not your fault. Try again shortly."
	default:
		friendly = "An API error occurred."
	}

	return fmt.Sprintf("API Error: %s\n\nDetails: %s", friendly, msg)
}

func (m TUIModel) columnWidths() (int, int, int) {
	left := m.width * 24 / 100
	center := m.width * 52 / 100
	right := m.width - left - center
	if left < 24 {
		left = 24
	}
	if right < 24 {
		right = 24
	}
	if center < 40 {
		center = 40
	}
	total := left + center + right
	if total > m.width {
		diff := total - m.width
		center -= diff
		if center < 20 {
			center = 20
		}
	}
	if left+center+right < m.width {
		right += m.width - (left + center + right)
	}
	return left, center, right
}

func (m TUIModel) bodyHeight() int {
	h := m.height - 6
	if h < 3 {
		h = 3
	}
	return h
}

func (m TUIModel) renderPanel(title, meta, body string, innerW, innerH int, focused bool) string {
	header := m.theme.PanelTitle.Render(title)
	if meta != "" {
		header += " " + m.theme.PanelMeta.Render("["+meta+"]")
	}
	content := header + "\n" + body
	return m.theme.PanelStyle(focused).Width(innerW).Height(innerH).Render(content)
}

func (m TUIModel) panelMeta(p planPanel) string {
	switch p {
	case panelTimeline:
		return "timeline"
	case panelChat:
		return "chat"
	case panelStatus:
		return "status"
	case panelInput:
		return "input"
	default:
		return ""
	}
}

func (m *TUIModel) scrollActive(delta int) {
	switch m.activePanel {
	case panelTimeline:
		lines := m.timelineLines()
		m.leftScroll = clampScroll(m.leftScroll+delta, len(lines), maxInt(1, m.bodyHeight()-4))
	case panelStatus:
		lines := m.statusPaneLines()
		m.rightScroll = clampScroll(m.rightScroll+delta, len(lines), maxInt(1, m.bodyHeight()-4))
	case panelChat:
		if delta > 0 {
			for i := 0; i < delta; i++ {
				m.viewport.LineDown(1)
			}
		} else {
			for i := 0; i < -delta; i++ {
				m.viewport.LineUp(1)
			}
		}
	}
}

func (m TUIModel) timelineView(height int) string {
	return m.renderScrolledLines(m.timelineLines(), m.leftScroll, height)
}

func (m TUIModel) statusPaneView(height int) string {
	return m.renderScrolledLines(m.statusPaneLines(), m.rightScroll, height)
}

func (m TUIModel) renderScrolledLines(lines []string, scroll int, height int) string {
	if height < 1 {
		height = 1
	}
	if len(lines) == 0 {
		return " "
	}
	start := clampScroll(scroll, len(lines), height)
	end := start + height
	if end > len(lines) {
		end = len(lines)
	}
	out := strings.Join(lines[start:end], "\n")
	if end-start < height {
		out += strings.Repeat("\n", height-(end-start))
	}
	return out
}

func (m TUIModel) timelineLines() []string {
	lines := []string{
		fmt.Sprintf("messages: %d", len(m.messages)),
		fmt.Sprintf("streaming: %t", m.streaming),
		fmt.Sprintf("truncation retries: %d/%d", m.continueCount, maxAutoContinues),
		"",
		"recent activity",
	}
	for i := len(m.messages) - 1; i >= 0 && len(lines) < 25; i-- {
		msg := m.messages[i]
		prefix := "A"
		switch msg.role {
		case "user":
			prefix = "U"
		case "tool":
			prefix = "T"
		case "error":
			prefix = "E"
		}
		text := strings.TrimSpace(msg.content)
		if text == "" {
			text = "(empty)"
		}
		runes := []rune(text)
		if len(runes) > 48 {
			text = string(runes[:48]) + "..."
		}
		lines = append(lines, fmt.Sprintf("%s | %s", prefix, text))
	}
	return lines
}

func (m TUIModel) statusPaneLines() []string {
	phase := m.genCurrentPhase
	if phase == "" {
		phase = "n/a"
	}
	lines := []string{
		fmt.Sprintf("mode: %s", m.modeLabel()),
		fmt.Sprintf("phase files generated: %d", m.phaseCount),
		fmt.Sprintf("current generation phase: %s", phase),
		fmt.Sprintf("generation complete count: %d", m.genCompleted),
		fmt.Sprintf("generation tool inflight: %t", m.genInFlight),
		fmt.Sprintf("auto-continue attempts: %d/%d", m.continueCount, maxAutoContinues),
	}
	if m.err != nil {
		lines = append(lines, "", "last error:", strings.TrimSpace(formatAPIError(m.err)))
	}
	return lines
}

func (m TUIModel) modeLabel() string {
	if m.streaming && m.generating {
		return "generating"
	}
	if m.streaming {
		return "streaming"
	}
	return "idle"
}

func (m TUIModel) helpView() string {
	help := []string{
		"Planner Keymap",
		"",
		"Global: q quit | ? help | tab focus | ctrl+g generate | ctrl+n reset",
		"Panels: timeline/chat/status use j/k and pgup/pgdn to scroll",
		"Input: focus input pane and press enter to send",
		"",
		"Press ? again to dismiss",
	}
	content := strings.Join(help, "\n")
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("81")).
		Background(lipgloss.Color("235")).
		Foreground(lipgloss.Color("252")).
		Padding(1, 2)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, style.Render(content))
}

func clampScroll(pos, total, viewport int) int {
	if pos < 0 {
		return 0
	}
	maxPos := total - viewport
	if maxPos < 0 {
		maxPos = 0
	}
	if pos > maxPos {
		return maxPos
	}
	return pos
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// padRight pads a string with spaces to fill width.
// Kept for compatibility with existing tests and helpers.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

// padLeft pads a string with spaces on the left to fill width.
// Kept for compatibility with existing tests and helpers.
func padLeft(s string, width int) string {
	if len(s) >= width {
		return s[len(s)-width:]
	}
	return strings.Repeat(" ", width-len(s)) + s
}
