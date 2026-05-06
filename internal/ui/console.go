package ui

import (
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Suggestion is one row in the @-autocomplete dropdown.
type Suggestion struct {
	Trigger string // e.g. "@create-session"
	Hint    string // e.g. "→ btrack start"
}

// SortSuggestions returns s sorted alphabetically by Trigger (stable).
func SortSuggestions(s []Suggestion) []Suggestion {
	out := make([]Suggestion, len(s))
	copy(out, s)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Trigger < out[j].Trigger })
	return out
}

// ─── console (interactive REPL prompt) ───────────────────────────────────────

// ConsoleModel renders the welcome banner (optionally) and a rounded
// input box, mirroring the Claude Code / Gemini CLI splash. Each
// `bubbletea` run captures one line of input then exits — the caller
// loops to keep the REPL going.
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
	suggestions []Suggestion // full alias list, alphabetised
	suggCursor  int          // index inside the *filtered* list
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

// WithSuggestions wires the @-autocomplete list into the model.
func (m ConsoleModel) WithSuggestions(s []Suggestion) ConsoleModel {
	m.suggestions = SortSuggestions(s)
	return m
}

// filteredSuggestions returns the @-actions matching the current input.
// Returns nil when the input doesn't start with "@".
func (m ConsoleModel) filteredSuggestions() []Suggestion {
	v := m.input.Value()
	if !strings.HasPrefix(v, "@") {
		return nil
	}
	q := strings.ToLower(strings.TrimPrefix(v, "@"))
	// Only filter on the first whitespace-delimited token — once the user
	// types "@create-session ", suggestions should disappear.
	if i := strings.IndexAny(q, " \t"); i >= 0 {
		return nil
	}

	out := make([]Suggestion, 0, len(m.suggestions))
	for _, s := range m.suggestions {
		needle := strings.TrimPrefix(strings.ToLower(s.Trigger), "@")
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
			// Tab fills the input with the highlighted suggestion.
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
	// Reset highlight whenever the visible suggestion list shrinks.
	if n := len(m.filteredSuggestions()); m.suggCursor >= n {
		m.suggCursor = 0
	}
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
	// textinput.View() in text mode renders prompt(2) + Width + cursor(1)
	// chars wide, so reserve `Width + 3` for content (+ 2 for padding).
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(0, 1).
		Width(m.input.Width + 5).
		Render(m.input.View())
	sb.WriteString(Indent + box + "\n")

	// @-autocomplete dropdown.
	if matches := m.filteredSuggestions(); len(matches) > 0 {
		sb.WriteString(m.renderSuggestions(matches) + "\n")
	} else {
		sb.WriteString("\n")
	}

	// Footer hints — swap in tab/↑↓ when the dropdown is showing.
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
			StyleHighlight.Render("@") + " quick actions",
			StyleHighlight.Render("/help") + " commands",
			StyleHighlight.Render("ctrl+c") + " exit",
		}
	}
	sb.WriteString(Indent + StyleDimmed.Render(strings.Join(hints, "  ·  ")) + "\n")

	return sb.String()
}

// renderSuggestions draws the autocomplete dropdown under the input box.
func (m ConsoleModel) renderSuggestions(matches []Suggestion) string {
	const maxVisible = 6

	// Window the list around the cursor so it stays visible.
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

	// Pad triggers to a fixed column for alignment.
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

	// Footer line if the list is truncated.
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
		Render(body)

	return Indent + box
}

