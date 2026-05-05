package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tolgazorlu/btrack/internal/daemon"
)

type StatusModel struct {
	status      *daemon.StatusData
	startTime   time.Time
	frame       int
	err         error
	quitting    bool
	client      *daemon.Client
	dailyHours  int
	idleMinutes int
	lastKey     time.Time
}

type tickMsg time.Time
type statusMsg *daemon.StatusData
type errMsg error

func NewStatusModel(client *daemon.Client, dailyHours int, idleMinutes int) *StatusModel {
	if dailyHours <= 0 {
		dailyHours = 8
	}
	return &StatusModel{
		client:      client,
		dailyHours:  dailyHours,
		idleMinutes: idleMinutes,
		lastKey:     time.Now(),
	}
}

func (m StatusModel) Init() tea.Cmd {
	return tea.Batch(fetchStatus(m.client), tickCmd())
}

func (m StatusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.lastKey = time.Now()
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		}
	case tickMsg:
		m.frame = (m.frame + 1) % len(PulseFrames)
		return m, tea.Batch(tickCmd(), fetchStatus(m.client))
	case statusMsg:
		m.status = msg
		if msg != nil && msg.Active && msg.Session != nil {
			m.startTime, _ = time.Parse(time.RFC3339, msg.Session.StartTime)
		}
	case errMsg:
		m.err = msg
	}
	return m, nil
}

func (m StatusModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(StyleTitle.Render("  btrack"))
	b.WriteString(StyleDimmed.Render(" — time tracker\n\n"))

	if m.err != nil {
		b.WriteString(StyleError.Render("  error: "+m.err.Error()) + "\n")
		b.WriteString(StyleDimmed.Render("  press q to quit\n"))
		return b.String()
	}

	if m.status == nil {
		b.WriteString(StyleSubtle.Render("  loading...\n"))
		return b.String()
	}

	if !m.status.Active || m.status.Session == nil {
		b.WriteString(StyleSubtle.Render("  no active session\n\n"))
		b.WriteString(StyleDimmed.Render("  start one with: ") + StyleHighlight.Render("btrack start <task>\n"))
		b.WriteString(StyleDimmed.Render("\n  press q to quit\n"))
		return b.String()
	}

	sess := m.status.Session
	pulse := StyleSuccess.Render(PulseFrames[m.frame])
	elapsed := time.Since(m.startTime)

	b.WriteString(fmt.Sprintf("  %s  %s\n", pulse, StyleTitle.Render(sess.TaskName)))
	b.WriteString(fmt.Sprintf("     %s\n\n", FormatDuration(elapsed)))

	if sess.Project != "" {
		b.WriteString(fmt.Sprintf("  %s\n\n", StyleHighlight.Render("@"+sess.Project)))
	}

	if len(sess.Tags) > 0 {
		b.WriteString("  ")
		for _, tag := range sess.Tags {
			b.WriteString(StyleTag.Render(tag) + "  ")
		}
		b.WriteString("\n\n")
	}

	if sess.GitBranch != "" {
		b.WriteString(StyleSubtle.Render(fmt.Sprintf("  ⎇  %s", sess.GitBranch)))
		if sess.GitRepo != "" {
			b.WriteString(StyleDimmed.Render(fmt.Sprintf("  (%s)", sess.GitRepo)))
		}
		b.WriteString("\n\n")
	}

	b.WriteString(renderProgressBar(elapsed, m.dailyHours) + "\n")
	b.WriteString(fmt.Sprintf("  %s\n\n", StyleDimmed.Render(fmt.Sprintf("%dh daily target", m.dailyHours))))

	// Idle warning when within last 20% of threshold.
	if m.idleMinutes > 0 {
		idleThreshold := time.Duration(m.idleMinutes) * time.Minute
		idleElapsed := time.Since(m.lastKey)
		if idleElapsed > idleThreshold*8/10 {
			remaining := idleThreshold - idleElapsed
			if remaining < 0 {
				remaining = 0
			}
			mins := int(remaining.Minutes()) + 1
			b.WriteString(StyleWarning.Render(fmt.Sprintf("  ⚠  idle — auto-stop in ~%dm\n\n", mins)))
		}
	}

	if len(m.status.RecentLog) > 0 {
		b.WriteString(StyleHighlight.Render("  recent notes") + "\n")
		for i := len(m.status.RecentLog) - 1; i >= 0; i-- {
			entry := m.status.RecentLog[i]
			ts, _ := time.Parse(time.RFC3339, entry.Timestamp)
			timeStr := StyleDimmed.Render(ts.Format("15:04"))
			b.WriteString(fmt.Sprintf("%s  %s %s\n",
				timeStr, StyleDimmed.Render("·"), StyleLogEntry.Render(entry.Note),
			))
		}
		b.WriteString("\n")
	}

	b.WriteString(StyleDimmed.Render("  q quit  ·  btrack log \"note\"  ·  btrack stop -m \"msg\"\n"))
	return b.String()
}

func RenderProgressBar(d time.Duration, dailyHours int) string {
	return renderProgressBar(d, dailyHours)
}

func renderProgressBar(d time.Duration, dailyHours int) string {
	const width = 40
	pct := d.Hours() / float64(dailyHours)
	if pct > 1 {
		pct = 1
	}
	filled := int(float64(width) * pct)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	color := colorSuccess
	if pct > 0.75 {
		color = colorWarning
	}
	if pct > 0.95 {
		color = colorError
	}
	return "  " + lipgloss.NewStyle().Foreground(color).Render(bar)
}

func tickCmd() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func fetchStatus(client *daemon.Client) tea.Cmd {
	return func() tea.Msg {
		resp, err := client.Send(daemon.ActionStatus, nil)
		if err != nil {
			return errMsg(err)
		}
		if !resp.Success {
			return errMsg(fmt.Errorf("%s", resp.Error))
		}
		var s daemon.StatusData
		if err := json.Unmarshal(resp.Data, &s); err != nil {
			return errMsg(err)
		}
		return statusMsg(&s)
	}
}
