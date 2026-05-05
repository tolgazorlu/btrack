package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// ─── types ────────────────────────────────────────────────────────────────────

type chatMsg struct {
	role string // "you" | "ai"
	text string
}

type chatResponseMsg struct {
	text string
	err  error
}

// SendFn is the AI completion function injected from cmd layer.
type SendFn func(ctx context.Context, prompt string) (string, error)

// ─── model ───────────────────────────────────────────────────────────────────

type ChatModel struct {
	msgs      []chatMsg
	input     textinput.Model
	loading   bool
	frame     int
	sendFn    SendFn
	sysPrompt string // btrack context injected at startup
	quitting  bool
	width     int
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func NewChatModel(sysPrompt string, fn SendFn) ChatModel {
	ti := textinput.New()
	ti.Placeholder = "ask anything about your work..."
	ti.Focus()
	ti.CharLimit = 500
	ti.Width = 56

	return ChatModel{
		input:     ti,
		sendFn:    fn,
		sysPrompt: sysPrompt,
		width:     80,
		msgs: []chatMsg{
			{
				role: "ai",
				text: "Hi! I have context about your sessions. Ask me anything — standups, productivity patterns, or what you worked on.",
			},
		},
	}
}

// ─── tea interface ───────────────────────────────────────────────────────────

func (m ChatModel) Init() tea.Cmd {
	return textinput.Blink
}

type chatTickMsg struct{}

func (m ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.input.Width = msg.Width - 10

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit

		case tea.KeyEnter:
			if m.loading {
				return m, nil
			}
			text := strings.TrimSpace(m.input.Value())
			if text == "" {
				return m, nil
			}
			m.msgs = append(m.msgs, chatMsg{role: "you", text: text})
			m.input.SetValue("")
			m.loading = true
			m.frame = 0
			return m, tea.Batch(m.doSend(text), chatTickCmd())
		}

	case chatTickMsg:
		if m.loading {
			m.frame = (m.frame + 1) % len(spinnerFrames)
			return m, chatTickCmd()
		}

	case chatResponseMsg:
		m.loading = false
		if msg.err != nil {
			m.msgs = append(m.msgs, chatMsg{role: "ai", text: "error: " + msg.err.Error()})
		} else {
			m.msgs = append(m.msgs, chatMsg{role: "ai", text: msg.text})
		}
	}

	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func chatTickCmd() tea.Cmd {
	return func() tea.Msg {
		// Small sleep via tea.Tick would be cleaner, but a no-op tick is fine
		// for spinner — each keypress or response will also redraw.
		return chatTickMsg{}
	}
}

func (m ChatModel) doSend(userText string) tea.Cmd {
	return func() tea.Msg {
		var history strings.Builder
		for _, msg := range m.msgs {
			if msg.role == "you" {
				history.WriteString("User: " + msg.text + "\n")
			} else {
				history.WriteString("Assistant: " + msg.text + "\n")
			}
		}

		prompt := fmt.Sprintf(
			"%s\n\nConversation so far:\n%s\nUser: %s\n\nRespond concisely (under 150 words). Use bullet points when listing items.",
			m.sysPrompt,
			history.String(),
			userText,
		)

		resp, err := m.sendFn(context.Background(), prompt)
		return chatResponseMsg{text: resp, err: err}
	}
}

// ─── view ────────────────────────────────────────────────────────────────────

func (m ChatModel) View() string {
	if m.quitting {
		return ""
	}

	sep := StyleDimmed.Render(strings.Repeat("─", 54))
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString("  " + StyleTitle.Render("btrack ai") + "  " + StyleDimmed.Render("chat  ·  ctrl+c to exit") + "\n")
	sb.WriteString("  " + sep + "\n\n")

	// Show messages — cap at last 12 to avoid overflow.
	msgs := m.msgs
	if len(msgs) > 12 {
		msgs = msgs[len(msgs)-12:]
	}

	for _, msg := range msgs {
		switch msg.role {
		case "you":
			sb.WriteString("  " + StyleHighlight.Render("you") + "  " + msg.text + "\n\n")
		case "ai":
			lines := strings.Split(strings.TrimSpace(msg.text), "\n")
			sb.WriteString("  " + StyleSuccess.Render(" ai") + "  " + lines[0] + "\n")
			for _, line := range lines[1:] {
				line = strings.TrimSpace(line)
				if line != "" {
					sb.WriteString("       " + line + "\n")
				}
			}
			sb.WriteString("\n")
		}
	}

	if m.loading {
		spinner := spinnerFrames[m.frame%len(spinnerFrames)]
		sb.WriteString("  " + StyleSuccess.Render(" ai") + "  " + StyleDimmed.Render(spinner+" thinking...") + "\n\n")
	}

	sb.WriteString("  " + sep + "\n")
	sb.WriteString("  " + StyleHighlight.Render(">") + " " + m.input.View() + "\n\n")

	hints := []string{"enter to send", "ctrl+c to exit"}
	sb.WriteString("  " + StyleDimmed.Render(strings.Join(hints, "  ·  ")) + "\n")

	return sb.String()
}
