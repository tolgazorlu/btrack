package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── console (interactive REPL prompt) ───────────────────────────────────────

// ConsoleModel renders the welcome banner (optionally) and a rounded
// input box, mirroring the Claude Code / Gemini CLI splash. Each
// `bubbletea` run captures one line of input then exits — the caller
// loops to keep the REPL going.
type ConsoleModel struct {
	input      textinput.Model
	tagline    string
	version    string
	hint       string
	width      int
	value      string
	quit       bool
	showHelp   bool
	showBanner bool
}

// NewConsoleModel returns a fresh console capturing one input.
//   - tagline:    shown under the banner (e.g. "time tracker for developers")
//   - version:    optional, shown next to tagline
//   - hint:       optional one-line hint above the input
//   - showBanner: render the big banner + tips block on top
func NewConsoleModel(tagline, version, hint string, showBanner bool) ConsoleModel {
	ti := textinput.New()
	ti.Placeholder = "type a command, @action, or ask anything…"
	ti.Prompt = "> "
	ti.PromptStyle = StyleSuccess
	ti.CharLimit = 500
	ti.Width = 56
	ti.Focus()

	if tagline == "" {
		tagline = "time tracker for developers"
	}

	return ConsoleModel{
		input:      ti,
		tagline:    tagline,
		version:    version,
		hint:       hint,
		width:      80,
		showBanner: showBanner,
	}
}

func (m ConsoleModel) Init() tea.Cmd { return textinput.Blink }

func (m ConsoleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		w := msg.Width - 8
		if w > 80 {
			w = 80
		}
		if w < 20 {
			w = 20
		}
		m.input.Width = w

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quit = true
			return m, tea.Quit
		case tea.KeyEnter:
			m.value = strings.TrimSpace(m.input.Value())
			return m, tea.Quit
		case tea.KeyCtrlH:
			m.showHelp = !m.showHelp
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// Value returns the trimmed input the user submitted.
// Empty when the user pressed esc / ctrl-c.
func (m ConsoleModel) Value() string { return m.value }

// Aborted reports whether the user dismissed the prompt.
func (m ConsoleModel) Aborted() bool { return m.quit }

func (m ConsoleModel) View() string {
	if m.quit && m.value == "" {
		return ""
	}

	var sb strings.Builder

	if m.showBanner {
		// Banner
		sb.WriteString("\n")
		sb.WriteString(Banner())
		sb.WriteString("\n")

		// Tagline + version
		taglineLine := Indent + StyleDimmed.Render(m.tagline)
		if m.version != "" {
			taglineLine += "  " + StyleDimmed.Render("·") + "  " + StyleHighlight.Render(m.version)
		}
		sb.WriteString(taglineLine + "\n\n")

		// Tips block
		sb.WriteString(Indent + StyleHighlight.Render("Tips for getting started:") + "\n")
		sb.WriteString(Indent + StyleDimmed.Render("1. ") +
			"Run any command — e.g. " + StyleHighlight.Render(`s "fix bug" -p myapp`) + "\n")
		sb.WriteString(Indent + StyleDimmed.Render("2. ") +
			"Use " + StyleHighlight.Render("@") + " for quick actions, e.g. " +
			StyleHighlight.Render("@create-session") + "\n")
		sb.WriteString(Indent + StyleDimmed.Render("3. ") +
			"Ask anything — free text goes to " + StyleHighlight.Render("btrack ai") + " chat\n")
		sb.WriteString(Indent + StyleDimmed.Render("4. ") +
			StyleHighlight.Render("/help") + " for the full reference, " +
			StyleHighlight.Render("/exit") + " to quit\n\n")
	} else {
		sb.WriteString("\n")
	}

	if m.hint != "" {
		sb.WriteString(Indent + StyleWarning.Render(m.hint) + "\n\n")
	}

	// Input box — rounded border, full-width relative to terminal.
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(0, 1).
		Width(m.input.Width + 4).
		Render(m.input.View())
	sb.WriteString(Indent + box + "\n\n")

	// Footer hints
	hints := []string{
		StyleHighlight.Render("enter") + " run",
		StyleHighlight.Render("@") + " quick actions",
		StyleHighlight.Render("/help") + " commands",
		StyleHighlight.Render("ctrl+c") + " exit",
	}
	sb.WriteString(Indent + StyleDimmed.Render(strings.Join(hints, "  ·  ")) + "\n")

	return sb.String()
}
