package planner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// TUI styles.
var (
	userStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("141"))
	toolStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	statusStyle   = lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("252"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	inputBarStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
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

// TUIModel is the bubbletea model for the interactive planning conversation.
type TUIModel struct {
	agent       *Agent
	projectRoot string

	messages      []chatMessage
	streaming     bool
	streamBuf     *strings.Builder
	phaseCount    int
	continueCount int
	initialMsg    string

	viewport viewport.Model
	input    textarea.Model
	spinner  spinner.Model
	width    int
	height   int
	ready    bool
	err      error

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
		streamBuf:   &strings.Builder{},
		viewport:    vp,
		input:       ta,
		spinner:     sp,
		renderer:    renderer,
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
		m.messages = append(m.messages, chatMessage{
			role:    "tool",
			content: fmt.Sprintf("[tool: %s]", msg.Chunk.Name),
		})
		m.refreshViewport()
		// Execute the tool and send the result back.
		cmds = append(cmds, m.executeToolCmd(msg.Chunk))

	case ToolResultMsg:
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
	case "ctrl+c", "esc":
		return m, tea.Quit

	case "ctrl+r":
		m.agent.Reset()
		m.messages = nil
		m.streaming = false
		m.streamBuf.Reset()
		m.phaseCount = 0
		m.continueCount = 0
		m.err = nil
		m.refreshViewport()
		return m, m.input.Focus()

	case "ctrl+a":
		if !m.streaming {
			return m, m.sendMessageCmd("The plan is approved. Please generate all phase files now using the generate_single_phase tool, one phase at a time.")
		}
		return m, nil

	case "enter":
		if m.streaming {
			return m, nil
		}
		val := strings.TrimSpace(m.input.Value())
		if val == "" {
			return m, nil
		}
		m.input.Reset()
		return m, m.sendMessageCmd(val)
	}

	// Pass other keys to textarea when not streaming.
	if !m.streaming {
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
		return "Initializing planner..."
	}

	var b strings.Builder
	b.WriteString(m.viewport.View())
	b.WriteByte('\n')
	b.WriteString(m.inputView())
	b.WriteByte('\n')
	b.WriteString(m.statusBar())
	return b.String()
}

func (m TUIModel) inputView() string {
	if m.streaming {
		return inputBarStyle.Render(fmt.Sprintf("  %s Thinking...", m.spinner.View()))
	}
	if m.err != nil {
		return m.input.View() + "\n" + errorStyle.Render("  ⚠ Error occurred — see above. Type to continue.")
	}
	return m.input.View()
}

func (m TUIModel) statusBar() string {
	left := " Planning"
	if m.phaseCount > 0 {
		left += fmt.Sprintf(" | Phases: %d", m.phaseCount)
	}

	right := "Ctrl+A: generate | Ctrl+R: reset | Esc: quit "

	leftWidth := m.width / 2
	rightWidth := m.width - leftWidth

	bar := padRight(left, leftWidth) + padLeft(right, rightWidth)
	return statusStyle.Render(bar)
}

// recalcSizes adjusts component dimensions after a window resize.
func (m *TUIModel) recalcSizes() {
	inputHeight := 5 // textarea lines + border
	statusHeight := 1
	vpHeight := m.height - inputHeight - statusHeight
	if vpHeight < 3 {
		vpHeight = 3
	}
	m.viewport.Width = m.width
	m.viewport.Height = vpHeight
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
	m.viewport.GotoBottom()
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

// padRight pads a string with spaces to fill width.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

// padLeft pads a string with spaces on the left to fill width.
func padLeft(s string, width int) string {
	if len(s) >= width {
		return s[len(s)-width:]
	}
	return strings.Repeat(" ", width-len(s)) + s
}
