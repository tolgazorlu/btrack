package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tolgazorlu/btrack/internal/daemon"
)

// ─── types ───────────────────────────────────────────────────────────────────

type PomoPhase int

const (
	PhaseFocus PomoPhase = iota
	PhaseBreak
	PhaseLongBreak
	PhaseDone
)

type pomoTickMsg struct{}
type pomoStartedMsg struct{}
type pomoStoppedMsg struct{}

// ─── model ───────────────────────────────────────────────────────────────────

type PomoModel struct {
	phase         PomoPhase
	remaining     time.Duration
	total         time.Duration // total duration for current phase
	pomoCount     int           // completed focus intervals
	rounds        int           // focus intervals before long break
	taskName      string
	workMins      int
	breakMins     int
	longBreakMins int
	client        *daemon.Client
	sessionActive bool
	quitting      bool
	width         int
}

func NewPomoModel(task string, client *daemon.Client, workMins, breakMins, longBreakMins, rounds int) PomoModel {
	work := time.Duration(workMins) * time.Minute
	return PomoModel{
		phase:         PhaseFocus,
		remaining:     work,
		total:         work,
		pomoCount:     0,
		rounds:        rounds,
		taskName:      task,
		workMins:      workMins,
		breakMins:     breakMins,
		longBreakMins: longBreakMins,
		client:        client,
		width:         80,
	}
}

// ─── tea interface ───────────────────────────────────────────────────────────

func (m PomoModel) Init() tea.Cmd {
	return tea.Batch(pomoTickEvery(), m.startSession())
}

func (m PomoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			if m.sessionActive {
				return m, tea.Batch(m.stopSession("stopped early #pomo"), tea.Quit)
			}
			return m, tea.Quit
		}

	case pomoStartedMsg:
		m.sessionActive = true

	case pomoStoppedMsg:
		m.sessionActive = false

	case pomoTickMsg:
		_ = msg
		if m.remaining > time.Second {
			m.remaining -= time.Second
			return m, pomoTickEvery()
		}
		// Phase complete
		return m.advancePhase()
	}
	return m, nil
}

func (m PomoModel) advancePhase() (PomoModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch m.phase {
	case PhaseFocus:
		m.pomoCount++
		cmds = append(cmds, m.stopSession("pomodoro complete #pomo"))
		m.sessionActive = false

		if m.pomoCount >= m.rounds {
			// Long break after completing all rounds
			dur := time.Duration(m.longBreakMins) * time.Minute
			m.phase = PhaseLongBreak
			m.remaining = dur
			m.total = dur
		} else {
			dur := time.Duration(m.breakMins) * time.Minute
			m.phase = PhaseBreak
			m.remaining = dur
			m.total = dur
		}

	case PhaseBreak, PhaseLongBreak:
		if m.phase == PhaseLongBreak {
			// All done after long break
			m.phase = PhaseDone
			return m, tea.Quit
		}
		// Start next focus round
		dur := time.Duration(m.workMins) * time.Minute
		m.phase = PhaseFocus
		m.remaining = dur
		m.total = dur
		cmds = append(cmds, m.startSession())
	}

	cmds = append(cmds, pomoTickEvery())
	return m, tea.Batch(cmds...)
}

// ─── daemon helpers ──────────────────────────────────────────────────────────

func (m PomoModel) startSession() tea.Cmd {
	return func() tea.Msg {
		payload := daemon.StartPayload{TaskName: m.taskName}
		resp, err := m.client.Send(daemon.ActionStart, payload)
		if err != nil || !resp.Success {
			// Ignore — session might already be active
		}
		return pomoStartedMsg{}
	}
}

func (m PomoModel) stopSession(msg string) tea.Cmd {
	return func() tea.Msg {
		payload := daemon.StopPayload{Message: msg}
		m.client.Send(daemon.ActionStop, payload)
		return pomoStoppedMsg{}
	}
}

// ─── view ────────────────────────────────────────────────────────────────────

func (m PomoModel) View() string {
	if m.quitting {
		return ""
	}
	if m.phase == PhaseDone {
		return fmt.Sprintf("\n  %s  %s  all %d pomodoros complete!\n\n",
			StyleSuccess.Render("✓"),
			StyleTitle.Render(m.taskName),
			m.pomoCount,
		)
	}

	var sb strings.Builder
	sep := StyleDimmed.Render(strings.Repeat("─", 50))

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("  %s  %s  %s\n",
		StyleTitle.Render("btrack"),
		StyleDimmed.Render("pomo"),
		StyleHighlight.Render(m.taskName),
	))
	sb.WriteString("  " + sep + "\n\n")

	// Phase label
	phaseLabel := ""
	switch m.phase {
	case PhaseFocus:
		phaseLabel = StyleSuccess.Render(fmt.Sprintf("  FOCUS  %d/%d", m.pomoCount+1, m.rounds))
	case PhaseBreak:
		phaseLabel = StyleHighlight.Render("  SHORT BREAK")
	case PhaseLongBreak:
		phaseLabel = StyleWarning.Render("  LONG BREAK")
	}
	sb.WriteString(phaseLabel + "\n\n")

	// Countdown
	mins := int(m.remaining.Minutes())
	secs := int(m.remaining.Seconds()) % 60
	countdown := StyleElapsed.Render(fmt.Sprintf("         %02d:%02d", mins, secs))
	sb.WriteString(countdown + "\n\n")

	// Progress bar
	const barWidth = 32
	elapsed := m.total - m.remaining
	pct := 0.0
	if m.total > 0 {
		pct = elapsed.Seconds() / m.total.Seconds()
	}
	if pct > 1 {
		pct = 1
	}
	filled := int(float64(barWidth) * pct)
	bar := StyleSuccess.Render(strings.Repeat("█", filled)) +
		StyleDimmed.Render(strings.Repeat("░", barWidth-filled))
	sb.WriteString(fmt.Sprintf("  %s  %d%%\n\n", bar, int(pct*100)))

	sb.WriteString("  " + sep + "\n")
	sb.WriteString(StyleDimmed.Render("  q to stop early\n"))

	return sb.String()
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func pomoTickEvery() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return pomoTickMsg{}
	})
}
