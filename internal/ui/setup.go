package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	aiPkg "github.com/tolgazorlu/btrack/internal/ai"
)

type setupStep int

const (
	stepPickProvider setupStep = iota
	stepEnterKey
	stepTesting
	stepResult
	stepAskAnother
	stepDone
)

type ProviderChoice struct {
	ID    string
	Label string
	Model string
	URL   string
}

var Providers = []ProviderChoice{
	{ID: "openai", Label: "OpenAI  (GPT-4o)", Model: "gpt-4o", URL: "platform.openai.com/api-keys"},
	{ID: "claude", Label: "Anthropic  (Claude Sonnet)", Model: "claude-sonnet-4-6", URL: "console.anthropic.com/settings/keys"},
	{ID: "gemini", Label: "Google  (Gemini 2.0 Flash)", Model: "gemini-2.0-flash", URL: "aistudio.google.com/apikey"},
}

type SetupModel struct {
	step      setupStep
	cursor    int
	input     textinput.Model
	chosen    ProviderChoice
	frame     int
	testErr   error
	saved     []string // providers configured so far
	quitting  bool
	testDone  chan testResult
	lastResult testResult
}

type testResult struct {
	ok  bool
	err error
}

type testDoneMsg testResult
type savedMsg struct{}

func NewSetupModel() SetupModel {
	ti := textinput.New()
	ti.Placeholder = "paste your API key here"
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'
	ti.CharLimit = 256
	ti.Width = 52

	return SetupModel{
		step:     stepPickProvider,
		input:    ti,
		testDone: make(chan testResult, 1),
	}
}

func (m SetupModel) Init() tea.Cmd {
	return nil
}

func (m SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.step {
		case stepPickProvider:
			return m.updatePick(msg)
		case stepEnterKey:
			return m.updateKey(msg)
		case stepTesting:
			// ignore keys while testing
		case stepResult:
			return m.updateResult(msg)
		case stepAskAnother:
			return m.updateAskAnother(msg)
		}
	case tickMsg:
		if m.step == stepTesting {
			m.frame = (m.frame + 1) % len(PulseFrames)
			return m, tickCmd()
		}
	case testDoneMsg:
		m.lastResult = testResult(msg)
		m.step = stepResult
		if msg.ok {
			return m, saveKeyCmd(m.chosen.ID, m.input.Value())
		}
		m.testErr = msg.err
	case savedMsg:
		m.saved = append(m.saved, m.chosen.Label)
		m.step = stepAskAnother
	}
	return m, nil
}

func (m SetupModel) updatePick(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(Providers)-1 {
			m.cursor++
		}
	case "enter", " ":
		m.chosen = Providers[m.cursor]
		m.input.Focus()
		m.input.Placeholder = "paste your " + m.chosen.ID + " key"
		m.step = stepEnterKey
		return m, textinput.Blink
	case "q", "ctrl+c", "esc":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

func (m SetupModel) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if strings.TrimSpace(m.input.Value()) == "" {
			return m, nil
		}
		m.step = stepTesting
		m.frame = 0
		return m, tea.Batch(tickCmd(), testKeyCmd(m.chosen.ID, m.input.Value()))
	case "esc":
		m.step = stepPickProvider
		m.input.Blur()
		m.input.SetValue("")
		return m, nil
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m SetupModel) updateResult(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", " ", "q", "esc", "ctrl+c":
		if m.lastResult.ok {
			m.step = stepAskAnother
		} else {
			// Retry: go back to key entry
			m.step = stepEnterKey
			m.input.SetValue("")
			m.input.Focus()
			return m, textinput.Blink
		}
	}
	return m, nil
}

func (m SetupModel) updateAskAnother(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.step = stepPickProvider
		m.cursor = 0
		m.input.SetValue("")
	case "n", "N", "enter", "esc", "ctrl+c", "q":
		m.step = stepDone
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

func (m SetupModel) IsDone() bool { return m.step == stepDone || m.quitting }

func (m SetupModel) View() string {
	if m.quitting && m.step != stepDone {
		return "\n"
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(StyleTitle.Render("  btrack AI Setup") + "\n\n")

	switch m.step {
	case stepPickProvider:
		b.WriteString(StyleHighlight.Render("  Select an AI provider:") + "\n\n")
		for i, p := range Providers {
			cursor := "  "
			label := StyleSubtle.Render("  " + p.Label)
			if i == m.cursor {
				cursor = StyleSuccess.Render("▶ ")
				label = StyleHighlight.Render("  " + p.Label)
			}
			// Mark already-configured providers
			tick := ""
			for _, s := range m.saved {
				if s == p.Label {
					tick = " " + StyleSuccess.Render("✓")
				}
			}
			b.WriteString(fmt.Sprintf("  %s%s%s\n", cursor, label, tick))
		}
		b.WriteString("\n")
		b.WriteString(StyleDimmed.Render("  ↑↓ navigate  ·  enter select  ·  q quit") + "\n")

	case stepEnterKey:
		b.WriteString(fmt.Sprintf("  %s\n\n", StyleHighlight.Render(m.chosen.Label)))
		b.WriteString(StyleSubtle.Render("  Paste your API key:") + "\n")
		b.WriteString("  " + m.input.View() + "\n\n")
		b.WriteString(StyleDimmed.Render(fmt.Sprintf("  Get your key → %s", m.chosen.URL)) + "\n\n")
		b.WriteString(StyleDimmed.Render("  enter confirm  ·  esc back") + "\n")

	case stepTesting:
		spinner := StyleWarning.Render(PulseFrames[m.frame])
		b.WriteString(fmt.Sprintf("  %s  Connecting to %s...\n", spinner, m.chosen.Label))

	case stepResult:
		if m.lastResult.ok {
			b.WriteString(StyleSuccess.Render("  ✓  Connected!") + "\n\n")
			b.WriteString(fmt.Sprintf("  %s  %s\n",
				StyleDimmed.Render("provider"),
				StyleHighlight.Render(m.chosen.Label),
			))
			b.WriteString(fmt.Sprintf("  %s  %s\n",
				StyleDimmed.Render("model   "),
				StyleSubtle.Render(m.chosen.Model),
			))
			b.WriteString("\n" + StyleDimmed.Render("  press enter to continue") + "\n")
		} else {
			b.WriteString(StyleError.Render("  ✗  Connection failed") + "\n\n")
			b.WriteString("  " + StyleWarning.Render(m.testErr.Error()) + "\n\n")
			b.WriteString(StyleDimmed.Render("  press enter to retry  ·  esc to pick another") + "\n")
		}

	case stepAskAnother:
		b.WriteString(StyleSuccess.Render("  ✓  Setup complete!") + "\n\n")
		if len(m.saved) > 0 {
			b.WriteString(StyleSubtle.Render("  Configured:") + "\n")
			for _, s := range m.saved {
				b.WriteString(StyleDimmed.Render("    · ") + StyleHighlight.Render(s) + "\n")
			}
		}
		b.WriteString("\n")
		b.WriteString(StyleHighlight.Render("  Add another provider?") +
			StyleDimmed.Render("  [y/N]") + "  ")
	}

	return StyleBorder.Render(b.String())
}

// SavedProviders returns the list of configured provider labels.
func (m SetupModel) SavedProviders() []string { return m.saved }

func testKeyCmd(provider, key string) tea.Cmd {
	return func() tea.Msg {
		p, err := aiPkg.NewProviderFor(provider, key)
		if err != nil {
			return testDoneMsg{ok: false, err: err}
		}
		if err := aiPkg.ValidateKey(context.Background(), p); err != nil {
			return testDoneMsg{ok: false, err: err}
		}
		return testDoneMsg{ok: true}
	}
}

func saveKeyCmd(provider, key string) tea.Cmd {
	return func() tea.Msg {
		_ = saveKey(provider, key) // errors handled silently; user sees success already
		return savedMsg{}
	}
}

// saveKey is set by cmd layer to avoid import cycle.
var saveKey func(provider, key string) error = func(_, _ string) error { return nil }

// SetSaveKeyFunc lets cmd/ai_setup.go inject the real save function.
func SetSaveKeyFunc(fn func(provider, key string) error) {
	saveKey = fn
}

// ─── standalone box rendering helpers used by insights ───────────────────────

func RenderBox(title, body string) string {
	content := StyleTitle.Render(title) + "\n\n" + body
	return StyleBorder.Render(content)
}

func RenderStat(label string, value string) string {
	lbl := lipgloss.NewStyle().Width(22).Foreground(ColorMuted).Render(label)
	return fmt.Sprintf("  %s  %s", lbl, value)
}

func RenderBar(label string, val, max float64, width int) string {
	pct := 0.0
	if max > 0 {
		pct = val / max
	}
	if pct > 1 {
		pct = 1
	}
	filled := int(float64(width) * pct)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	color := colorSuccess
	if pct > 0.75 {
		color = colorWarning
	}
	barStr := lipgloss.NewStyle().Foreground(color).Render(bar)
	lbl := lipgloss.NewStyle().Width(24).Foreground(ColorMuted).Render(label)
	return fmt.Sprintf("  %s %s  %.0f%%", lbl, barStr, pct*100)
}
