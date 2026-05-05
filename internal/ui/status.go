package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tolgazorlu/btrack/internal/daemon"
	"github.com/tolgazorlu/btrack/internal/db"
)

type inputMode int

const (
	modeNormal inputMode = iota
	modeNote
	modeStop
)

type StatusModel struct {
	status       *daemon.StatusData
	startTime    time.Time
	frame        int
	err          error
	quitting     bool
	client       *daemon.Client
	dailyHours   int
	idleMinutes  int
	lastKey      time.Time
	store        db.Store
	sessions     []*db.Session
	mode         inputMode
	inputText    string
	updateAvail  string
	version      string
	actionResult string
	actionIsErr  bool
}

// ── message types ─────────────────────────────────────────────────────────────
type tickMsg time.Time
type statusMsg *daemon.StatusData
type errMsg error
type sessionsMsg []*db.Session
type sessionsTickMsg struct{}
type updateCheckMsg struct{ newVersion string }
type actionResultMsg struct {
	text    string
	isError bool
}
type clearResultMsg struct{}

// ─────────────────────────────────────────────────────────────────────────────

func NewStatusModel(client *daemon.Client, dailyHours int, idleMinutes int, store db.Store, version string) *StatusModel {
	if dailyHours <= 0 {
		dailyHours = 8
	}
	return &StatusModel{
		client:      client,
		dailyHours:  dailyHours,
		idleMinutes: idleMinutes,
		lastKey:     time.Now(),
		store:       store,
		version:     version,
	}
}

func (m StatusModel) Init() tea.Cmd {
	return tea.Batch(
		fetchStatus(m.client),
		tickCmd(),
		fetchTodaySessions(m.store),
		sessionsTickCmd(),
		checkForUpdateCmd(m.version),
	)
}

func (m StatusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// ── Input mode ────────────────────────────────────────────
		if m.mode != modeNormal {
			switch msg.Type {
			case tea.KeyEsc:
				m.mode = modeNormal
				m.inputText = ""
			case tea.KeyEnter:
				mode := m.mode
				text := strings.TrimSpace(m.inputText)
				m.mode = modeNormal
				m.inputText = ""
				if mode == modeNote {
					return m, sendNoteCmd(m.client, text)
				}
				return m, sendStopCmd(m.client, text)
			case tea.KeyBackspace, tea.KeyDelete:
				if len(m.inputText) > 0 {
					m.inputText = m.inputText[:len(m.inputText)-1]
				}
			default:
				if msg.Type == tea.KeyRunes {
					m.inputText += string(msg.Runes)
				}
			}
			return m, nil
		}

		// ── Normal mode ───────────────────────────────────────────
		m.lastKey = time.Now()
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "n":
			if m.status != nil && m.status.Active {
				m.mode = modeNote
				m.inputText = ""
				m.actionResult = ""
			}
		case "x":
			if m.status != nil && m.status.Active {
				m.mode = modeStop
				m.inputText = ""
				m.actionResult = ""
			}
		}

	case tickMsg:
		m.frame = (m.frame + 1) % len(PulseFrames)
		return m, tea.Batch(tickCmd(), fetchStatus(m.client))

	case sessionsTickMsg:
		return m, tea.Batch(fetchTodaySessions(m.store), sessionsTickCmd())

	case statusMsg:
		m.status = msg
		if msg != nil && msg.Active && msg.Session != nil {
			t, _ := time.Parse(time.RFC3339, msg.Session.StartTime)
			m.startTime = t.Local()
		}

	case errMsg:
		m.err = msg

	case sessionsMsg:
		m.sessions = []*db.Session(msg)

	case updateCheckMsg:
		m.updateAvail = msg.newVersion

	case actionResultMsg:
		m.actionResult = msg.text
		m.actionIsErr = msg.isError
		return m, clearResultAfter(3 * time.Second)

	case clearResultMsg:
		m.actionResult = ""
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

	// ── Today's completed sessions ────────────────────────────────────────────
	var completed []*db.Session
	for _, s := range m.sessions {
		if s.EndTime != nil {
			completed = append(completed, s)
		}
	}

	if len(completed) > 0 {
		var todayTotal time.Duration
		for _, s := range completed {
			todayTotal += s.EndTime.Sub(s.StartTime)
		}

		sep := StyleDimmed.Render(strings.Repeat("─", 52))
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			StyleHighlight.Render("today"),
			StyleDimmed.Render(compactDur(todayTotal)+" tracked"),
		))
		b.WriteString("  " + sep + "\n")

		// Show last 5 completed sessions (oldest first in today's batch)
		shown := completed
		if len(shown) > 5 {
			shown = shown[len(shown)-5:]
		}
		for _, s := range shown {
			startStr := s.StartTime.Local().Format("15:04")
			endStr := s.EndTime.Local().Format("15:04")
			d := s.EndTime.Sub(s.StartTime)
			timeRange := fmt.Sprintf("%s–%s", startStr, endStr)
			taskStr := s.TaskName
			if len(taskStr) > 30 {
				taskStr = taskStr[:27] + "..."
			}
			b.WriteString(fmt.Sprintf("  %s  %-30s  %s\n",
				StyleDimmed.Render(fmt.Sprintf("%-11s", timeRange)),
				StyleSubtle.Render(taskStr),
				StyleElapsed.Render(compactDur(d)),
			))
		}
		b.WriteString("  " + sep + "\n\n")
	}

	// ── Active session ────────────────────────────────────────────────────────
	if !m.status.Active || m.status.Session == nil {
		b.WriteString(StyleSubtle.Render("  no active session\n\n"))
		b.WriteString(StyleDimmed.Render("  start one with: ") + StyleHighlight.Render("btrack s \"task\"\n"))
		b.WriteString(StyleDimmed.Render("\n  q quit\n"))
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

	// Idle warning when within last 20% of threshold
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

	// Recent notes
	if len(m.status.RecentLog) > 0 {
		b.WriteString(StyleHighlight.Render("  recent notes") + "\n")
		for i := len(m.status.RecentLog) - 1; i >= 0; i-- {
			entry := m.status.RecentLog[i]
			ts, _ := time.Parse(time.RFC3339, entry.Timestamp)
			timeStr := StyleDimmed.Render(ts.Local().Format("15:04"))
			b.WriteString(fmt.Sprintf("%s  %s %s\n",
				timeStr, StyleDimmed.Render("·"), StyleLogEntry.Render(entry.Note),
			))
		}
		b.WriteString("\n")
	}

	// Action result feedback (note added / session stopped)
	if m.actionResult != "" {
		if m.actionIsErr {
			b.WriteString(StyleError.Render("  ✗  "+m.actionResult) + "\n\n")
		} else {
			b.WriteString(StyleSuccess.Render("  ✓  "+m.actionResult) + "\n\n")
		}
	}

	// ── Input overlay / key hints ─────────────────────────────────────────────
	switch m.mode {
	case modeNote:
		b.WriteString(StyleDimmed.Render("  ─────────────────────────────────────\n"))
		b.WriteString(fmt.Sprintf("  %s %s_\n",
			StyleHighlight.Render("note:"),
			StyleTitle.Render(m.inputText),
		))
		b.WriteString(StyleDimmed.Render("  enter to save  ·  esc to cancel\n"))

	case modeStop:
		b.WriteString(StyleDimmed.Render("  ─────────────────────────────────────\n"))
		b.WriteString(fmt.Sprintf("  %s %s_\n",
			StyleWarning.Render("stop:"),
			StyleTitle.Render(m.inputText),
		))
		b.WriteString(StyleDimmed.Render("  enter to stop  ·  esc to cancel\n"))

	default:
		if m.updateAvail != "" {
			b.WriteString(StyleWarning.Render(
				"  ↑ "+m.updateAvail+" available  ·  go install github.com/tolgazorlu/btrack@latest\n",
			))
		}
		b.WriteString(StyleDimmed.Render("  n note  ·  x stop  ·  q quit\n"))
	}

	return b.String()
}

// ─── progress bar ─────────────────────────────────────────────────────────────

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

// ─── tea.Cmd helpers ──────────────────────────────────────────────────────────

func tickCmd() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func sessionsTickCmd() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return sessionsTickMsg{}
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

func fetchTodaySessions(store db.Store) tea.Cmd {
	if store == nil {
		return nil
	}
	return func() tea.Msg {
		sessions, err := store.GetSessionsForDate(time.Now())
		if err != nil {
			return nil
		}
		return sessionsMsg(sessions)
	}
}

func checkForUpdateCmd(currentVersion string) tea.Cmd {
	if currentVersion == "" || currentVersion == "dev" {
		return nil
	}
	return func() tea.Msg {
		c := &http.Client{Timeout: 4 * time.Second}
		resp, err := c.Get("https://api.github.com/repos/tolgazorlu/btrack/releases/latest")
		if err != nil {
			return updateCheckMsg{}
		}
		defer resp.Body.Close()
		var data struct {
			TagName string `json:"tag_name"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return updateCheckMsg{}
		}
		if data.TagName != "" && data.TagName != currentVersion {
			return updateCheckMsg{newVersion: data.TagName}
		}
		return updateCheckMsg{}
	}
}

func sendNoteCmd(client *daemon.Client, note string) tea.Cmd {
	return func() tea.Msg {
		if note == "" {
			return actionResultMsg{"nothing to add", false}
		}
		resp, err := client.Send(daemon.ActionLog, daemon.LogPayload{Note: note})
		if err != nil {
			return actionResultMsg{err.Error(), true}
		}
		if !resp.Success {
			return actionResultMsg{resp.Error, true}
		}
		return actionResultMsg{"note added", false}
	}
}

func sendStopCmd(client *daemon.Client, message string) tea.Cmd {
	return func() tea.Msg {
		resp, err := client.Send(daemon.ActionStop, daemon.StopPayload{Message: message})
		if err != nil {
			return actionResultMsg{err.Error(), true}
		}
		if !resp.Success {
			return actionResultMsg{resp.Error, true}
		}
		return actionResultMsg{"session stopped", false}
	}
}

func clearResultAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return clearResultMsg{}
	})
}

// ─── local formatting ─────────────────────────────────────────────────────────

func compactDur(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}
