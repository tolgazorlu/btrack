package ui

import (
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Suggestion struct {
	Trigger string
	Hint    string
}

func SortSuggestions(s []Suggestion) []Suggestion {
	out := make([]Suggestion, len(s))
	copy(out, s)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Trigger < out[j].Trigger })
	return out
}

type ConsoleModel struct {
	input       textinput.Model
	tagline     string
	version     string
	hint        string
	width       int
	value       string
	quit        bool
	showHelp    bool
	showBanner  bool
	suggestions []Suggestion
	suggCursor  int
}

func NewConsoleModel(tagline, version, hint string, showBanner bool) ConsoleModel {
	ti := textinput.New()
	ti.Placeholder = "type a command, /action, or ask anything…"
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

func (m ConsoleModel) WithSuggestions(s []Suggestion) ConsoleModel {
	m.suggestions = SortSuggestions(s)
	return m
}

func (m ConsoleModel) filteredSuggestions() []Suggestion {
	v := m.input.Value()
	if !strings.HasPrefix(v, "/") {
		return nil
	}
	q := strings.ToLower(strings.TrimPrefix(v, "/"))
	if i := strings.IndexAny(q, " \t"); i >= 0 {
		return nil
	}

	out := make([]Suggestion, 0, len(m.suggestions))
	for _, s := range m.suggestions {
		needle := strings.TrimPrefix(strings.ToLower(s.Trigger), "/")
		if strings.HasPrefix(needle, q) {
			out = append(out, s)
		}
	}
	return out
}

func (m ConsoleModel) Init() tea.Cmd { return textinput.Blink }

func (m ConsoleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		w := msg.Width - 9
		if w > 80 {
			w = 80
		}
		if w < 20 {
			w = 20
		}
		m.input.Width = w

	case tea.KeyMsg:
		matches := m.filteredSuggestions()

		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quit = true
			return m, tea.Quit

		case tea.KeyEnter:
			m.value = strings.TrimSpace(m.input.Value())
			return m, tea.Quit

		case tea.KeyTab:
			if len(matches) > 0 {
				if m.suggCursor >= len(matches) {
					m.suggCursor = 0
				}
				m.input.SetValue(matches[m.suggCursor].Trigger + " ")
				m.input.CursorEnd()
				m.suggCursor = 0
				return m, nil
			}

		case tea.KeyDown:
			if len(matches) > 0 {
				m.suggCursor = (m.suggCursor + 1) % len(matches)
				return m, nil
			}

		case tea.KeyUp:
			if len(matches) > 0 {
				m.suggCursor--
				if m.suggCursor < 0 {
					m.suggCursor = len(matches) - 1
				}
				return m, nil
			}

		case tea.KeyCtrlH:
			m.showHelp = !m.showHelp
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if n := len(m.filteredSuggestions()); m.suggCursor >= n {
		m.suggCursor = 0
	}
	return m, cmd
}

func (m ConsoleModel) Value() string { return m.value }

func (m ConsoleModel) Aborted() bool { return m.quit }

func (m ConsoleModel) View() string {
	if m.quit && m.value == "" {
		return ""
	}

	var sb strings.Builder

	if m.showBanner {
		sb.WriteString("\n")
		sb.WriteString(Banner())
		sb.WriteString("\n")

		taglineLine := Indent + StyleDimmed.Render(m.tagline)
		if m.version != "" {
			taglineLine += "  " + StyleDimmed.Render("·") + "  " + StyleHighlight.Render(m.version)
		}
		sb.WriteString(taglineLine + "\n\n")

		sb.WriteString(Indent + StyleHighlight.Render("Tips for getting started:") + "\n")
		sb.WriteString(Indent + StyleDimmed.Render("1. ") +
			"Run any command — e.g. " + StyleHighlight.Render(`s "fix bug" -p myapp`) + "\n")
		sb.WriteString(Indent + StyleDimmed.Render("2. ") +
			"Use " + StyleHighlight.Render("/") + " for quick actions, e.g. " +
			StyleHighlight.Render("/start") + "\n")
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

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(0, 1).
		MarginLeft(len(Indent)).
		Width(m.input.Width + 5).
		Render(m.input.View())
	sb.WriteString(box + "\n")

	if matches := m.filteredSuggestions(); len(matches) > 0 {
		sb.WriteString(m.renderSuggestions(matches) + "\n")
	} else {
		sb.WriteString("\n")
	}

	var hints []string
	if len(m.filteredSuggestions()) > 0 {
		hints = []string{
			StyleHighlight.Render("tab") + " complete",
			StyleHighlight.Render("↑↓") + " navigate",
			StyleHighlight.Render("enter") + " run",
			StyleHighlight.Render("esc") + " exit",
		}
	} else {
		hints = []string{
			StyleHighlight.Render("enter") + " run",
			StyleHighlight.Render("/") + " commands",
			StyleHighlight.Render("/help") + " full reference",
			StyleHighlight.Render("ctrl+c") + " exit",
		}
	}
	sb.WriteString(Indent + StyleDimmed.Render(strings.Join(hints, "  ·  ")) + "\n")

	return sb.String()
}

func (m ConsoleModel) renderSuggestions(matches []Suggestion) string {
	const maxVisible = 6

	start := 0
	if len(matches) > maxVisible {
		start = m.suggCursor - maxVisible/2
		if start < 0 {
			start = 0
		}
		if start+maxVisible > len(matches) {
			start = len(matches) - maxVisible
		}
	}
	end := start + maxVisible
	if end > len(matches) {
		end = len(matches)
	}

	maxTrig := 0
	for _, s := range matches[start:end] {
		if len(s.Trigger) > maxTrig {
			maxTrig = len(s.Trigger)
		}
	}

	var rows []string
	for i, s := range matches[start:end] {
		idx := start + i
		trigger := s.Trigger + strings.Repeat(" ", maxTrig-len(s.Trigger))
		var row string
		if idx == m.suggCursor {
			row = StyleSuccess.Render("▸ ") +
				StyleHighlight.Render(trigger) +
				"   " + StyleSubtle.Render(s.Hint)
		} else {
			row = "  " + StyleDimmed.Render(trigger) +
				"   " + StyleDimmed.Render(s.Hint)
		}
		rows = append(rows, row)
	}

	if len(matches) > maxVisible {
		rows = append(rows,
			StyleDimmed.Render("  … "+strconv.Itoa(len(matches)-maxVisible)+" more"),
		)
	}

	body := strings.Join(rows, "\n")
	box := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(colorBorder).
		PaddingLeft(1).
		MarginLeft(len(Indent)).
		Render(body)

	return box
}
