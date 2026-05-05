package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/db"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var historyCmd = &cobra.Command{
	Use:     "history [today|yesterday|YYYY-MM-DD]",
	Aliases: []string{"h", "hist", "log", "l"},
	Short:   "View sessions — daily, weekly, monthly, yearly, or table",
	Long: `One command to view all your work across any time window.

Usage:
  btrack h                     today as a tree (default)
  btrack h -d                  today as a tree + GitHub commits for the day
  btrack h -w                  this week
  btrack h -m                  this month
  btrack h -y                  this year
  btrack h -n 20               last 20 sessions as a table
  btrack h -n 20 -v            with notes
  btrack h -l 5                last 5 hours
  btrack h yesterday           yesterday's tree
  btrack h 2026-05-01          specific date

Flags:
  -d, --day      day tree (default)
  -w, --week     week tree
  -m, --month    monthly summary
  -y, --year     yearly summary
  -n, --limit    last N sessions (table view)
  -l, --last     last N hours (tree view)
  -v, --notes    show checkpoint notes (table view)

The date argument accepts: today, yesterday, or YYYY-MM-DD`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		weekly, _ := cmd.Flags().GetBool("week")
		monthly, _ := cmd.Flags().GetBool("month")
		yearly, _ := cmd.Flags().GetBool("year")
		daily, _ := cmd.Flags().GetBool("day")
		limit, _ := cmd.Flags().GetInt("limit")
		lastHours, _ := cmd.Flags().GetFloat64("last")
		verbose, _ := cmd.Flags().GetBool("notes")
		project, _ := cmd.Flags().GetString("project")

		switch {
		case weekly:
			return runWeek(cmd, args)
		case monthly:
			return runMonth()
		case yearly:
			return runYear()
		case limit > 0:
			return runTable(limit, verbose, project)
		case lastHours > 0:
			return runLastHours(lastHours)
		case daily:
			// Explicit -d: day tree including GitHub commits for that day
			return runDay(cmd, args)
		default:
			// Default: day tree for today (or date arg)
			return runDay(cmd, args)
		}
	},
}

// ─── table view ──────────────────────────────────────────────────────────────

func runTable(limit int, showNotes bool, project string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	store, err := db.Open(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	var sessions []*db.Session
	if project != "" {
		sessions, err = store.GetSessionsByProject(project, limit)
	} else {
		sessions, err = store.GetRecentSessions(limit)
	}
	if err != nil {
		return fmt.Errorf("load sessions: %w", err)
	}
	if len(sessions) == 0 {
		fmt.Println(ui.StyleSubtle.Render("\n  no sessions recorded yet\n"))
		fmt.Println(ui.StyleDimmed.Render("  start one with: btrack s \"task\"\n"))
		return nil
	}

	fmt.Println()
	printTableHeader()

	var total time.Duration
	for _, s := range sessions {
		d := s.Duration()
		total += d
		printSessionRow(s, d)
		if showNotes {
			logs, err := store.GetAllLogs(s.ID)
			if err == nil {
				for _, l := range logs {
					fmt.Printf("  %s %s %s\n",
						ui.StyleDimmed.Render("  "+l.Timestamp.Local().Format("15:04")),
						ui.StyleDimmed.Render("·"),
						ui.StyleLogEntry.Render(l.Note),
					)
				}
			}
		}
	}

	fmt.Println()
	printTotals(len(sessions), total)
	fmt.Println()
	return nil
}

// ─── last N hours ────────────────────────────────────────────────────────────

func runLastHours(hours float64) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	store, err := db.Open(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	since := time.Now().Add(-time.Duration(hours * float64(time.Hour)))
	all, err := store.GetRecentSessions(500)
	if err != nil {
		return err
	}

	var sessions []*db.Session
	for _, s := range all {
		if s.StartTime.After(since) {
			sessions = append(sessions, s)
		}
	}

	label := fmt.Sprintf("last %.0fh", hours)
	if hours != float64(int(hours)) {
		label = fmt.Sprintf("last %.1fh", hours)
	}

	sep := ui.StyleDimmed.Render(strings.Repeat("─", 58))
	fmt.Println()
	fmt.Printf("  %s  %s\n", ui.StyleTitle.Render("btrack"), ui.StyleHighlight.Render(label))
	fmt.Println("  " + sep)

	if len(sessions) == 0 {
		fmt.Println(ui.StyleSubtle.Render("\n  no sessions in this window\n"))
		return nil
	}

	var total time.Duration
	for i, sess := range sessions {
		d := sess.Duration()
		total += d
		isLast := i == len(sessions)-1
		branch, childPfx := "  ├─", "  │  "
		if isLast {
			branch, childPfx = "  └─", "     "
		}
		startStr := sess.StartTime.Local().Format("15:04")
		endStr := "…"
		if sess.EndTime != nil {
			endStr = sess.EndTime.Local().Format("15:04")
		}
		taskStr := sess.TaskName
		if len(taskStr) > 32 {
			taskStr = taskStr[:29] + "..."
		}
		fmt.Printf("%s %s  %s  %s\n",
			ui.StyleDimmed.Render(branch),
			ui.StyleHighlight.Render(fmt.Sprintf("%-33s", taskStr)),
			ui.StyleDimmed.Render(fmt.Sprintf("%s–%s", startStr, endStr)),
			ui.StyleElapsed.Render(formatDur(d)),
		)
		logs, _ := store.GetAllLogs(sess.ID)
		for k, log := range logs {
			logBranch := childPfx + "├─"
			if k == len(logs)-1 {
				logBranch = childPfx + "└─"
			}
			fmt.Printf("%s %s  %s\n",
				ui.StyleDimmed.Render(logBranch),
				ui.StyleDimmed.Render(log.Timestamp.Local().Format("15:04")),
				ui.StyleLogEntry.Render(log.Note),
			)
		}
		if !isLast {
			fmt.Printf("%s\n", ui.StyleDimmed.Render("  │"))
		}
	}
	fmt.Println()
	fmt.Println("  " + sep)
	fmt.Printf("  %s  %s total  ·  %d sessions\n\n",
		ui.StyleDimmed.Render("summary"),
		ui.StyleElapsed.Render(formatDur(total)),
		len(sessions),
	)
	return nil
}

// ─── monthly view ────────────────────────────────────────────────────────────

func runMonth() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	dailyHours := 8
	if cfg.Work.DailyHours > 0 {
		dailyHours = cfg.Work.DailyHours
	}

	store, err := db.Open(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)

	// Fetch all sessions for the month.
	all, err := store.GetRecentSessions(2000)
	if err != nil {
		return err
	}

	// Group by week.
	type weekStat struct {
		label    string
		start    time.Time
		end      time.Time
		dur      time.Duration
		sessions int
		days     int
	}

	// Build weeks: each Monday-Sunday that overlaps the month.
	var weeks []weekStat
	weekday := int(monthStart.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	firstMonday := monthStart.AddDate(0, 0, -(weekday - 1))
	if firstMonday.Before(monthStart) && firstMonday.AddDate(0, 0, 6).Before(monthStart) {
		firstMonday = firstMonday.AddDate(0, 0, 7)
	}

	// Simpler: just iterate weeks from month start
	wStart := monthStart
	for wStart.Month() == now.Month() || wStart.Before(now) {
		wd := int(wStart.Weekday())
		if wd == 0 {
			wd = 7
		}
		mon := wStart.AddDate(0, 0, -(wd - 1))
		sun := mon.AddDate(0, 0, 6)
		_ = firstMonday

		ws := weekStat{
			label: fmt.Sprintf("%s – %s", mon.Format("Jan 02"), sun.Format("Jan 02")),
			start: mon,
			end:   sun.Add(24 * time.Hour),
		}
		activeDaySet := map[string]bool{}
		for _, s := range all {
			st := s.StartTime.Local()
			if !st.Before(mon) && st.Before(sun.Add(24*time.Hour)) {
				d := s.Duration()
				ws.dur += d
				ws.sessions++
				activeDaySet[st.Format("2006-01-02")] = true
			}
		}
		ws.days = len(activeDaySet)
		weeks = append(weeks, ws)
		wStart = wStart.AddDate(0, 0, 7-int(wStart.Weekday())+1)
		if wStart.Month() != now.Month() && wStart.After(now) {
			break
		}
	}

	// Remove duplicate weeks
	seen := map[string]bool{}
	var uniqueWeeks []weekStat
	for _, w := range weeks {
		k := w.start.Format("2006-01-02")
		if !seen[k] {
			seen[k] = true
			uniqueWeeks = append(uniqueWeeks, w)
		}
	}
	weeks = uniqueWeeks

	// Find max week duration for bar scaling.
	var maxDur time.Duration
	for _, w := range weeks {
		if w.dur > maxDur {
			maxDur = w.dur
		}
	}
	if maxDur == 0 {
		maxDur = time.Duration(dailyHours) * 5 * time.Hour
	}

	sep := ui.StyleDimmed.Render(strings.Repeat("─", 62))
	fmt.Println()
	fmt.Printf("  %s  %s\n", ui.StyleTitle.Render("btrack"), ui.StyleHighlight.Render(now.Format("January 2006")))
	fmt.Println("  " + sep)
	fmt.Println()

	const barWidth = 24
	for _, w := range weeks {
		filled := 0
		if maxDur > 0 {
			filled = int(float64(barWidth) * w.dur.Hours() / maxDur.Hours())
		}
		if filled > barWidth {
			filled = barWidth
		}
		bar := ui.StyleSuccess.Render(strings.Repeat("█", filled)) +
			ui.StyleDimmed.Render(strings.Repeat("░", barWidth-filled))

		dstr := ""
		if w.dur > 0 {
			dstr = formatDur(w.dur)
		} else {
			dstr = "—"
		}
		fmt.Printf("  %s  %s  %s  %s\n",
			ui.StyleDimmed.Render(fmt.Sprintf("%-19s", w.label)),
			bar,
			ui.StyleElapsed.Render(fmt.Sprintf("%-8s", dstr)),
			ui.StyleDimmed.Render(fmt.Sprintf("%d days", w.days)),
		)
	}

	// Monthly totals.
	var monthTotal time.Duration
	monthSessions := 0
	activeDaySet := map[string]bool{}
	for _, s := range all {
		st := s.StartTime.Local()
		if st.Month() == now.Month() && st.Year() == now.Year() {
			monthTotal += s.Duration()
			monthSessions++
			activeDaySet[st.Format("2006-01-02")] = true
		}
	}

	fmt.Println()
	fmt.Println("  " + sep)
	fmt.Printf("  %s  %s total  ·  %d sessions  ·  %d active days\n\n",
		ui.StyleDimmed.Render(now.Format("January")),
		ui.StyleElapsed.Render(formatDur(monthTotal)),
		monthSessions,
		len(activeDaySet),
	)
	return nil
}

// ─── yearly view ─────────────────────────────────────────────────────────────

func runYear() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	store, err := db.Open(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	now := time.Now()
	all, err := store.GetRecentSessions(5000)
	if err != nil {
		return err
	}

	type monthStat struct {
		month    time.Month
		dur      time.Duration
		sessions int
		days     int
	}

	months := make([]monthStat, 12)
	for i := range months {
		months[i].month = time.Month(i + 1)
	}

	var yearTotal time.Duration
	yearSessions := 0
	yearActiveDays := map[string]bool{}

	for _, s := range all {
		if s.StartTime.Year() != now.Year() {
			continue
		}
		mi := int(s.StartTime.Month()) - 1
		d := s.Duration()
		months[mi].dur += d
		months[mi].sessions++
		months[mi].days++ // rough, will dedup below
		yearTotal += d
		yearSessions++
		yearActiveDays[s.StartTime.Local().Format("2006-01-02")] = true
	}

	// Find max month duration for scaling.
	var maxDur time.Duration
	for _, m := range months {
		if m.dur > maxDur {
			maxDur = m.dur
		}
	}
	if maxDur == 0 {
		maxDur = 160 * time.Hour // fallback
	}

	sep := ui.StyleDimmed.Render(strings.Repeat("─", 62))
	fmt.Println()
	fmt.Printf("  %s  %s\n", ui.StyleTitle.Render("btrack"), ui.StyleHighlight.Render(fmt.Sprintf("%d", now.Year())))
	fmt.Println("  " + sep)
	fmt.Println()

	const barWidth = 28
	for _, m := range months {
		if m.month > now.Month() {
			break // don't show future months
		}
		filled := 0
		if maxDur > 0 {
			filled = int(float64(barWidth) * m.dur.Hours() / maxDur.Hours())
		}
		if filled > barWidth {
			filled = barWidth
		}
		bar := ui.StyleSuccess.Render(strings.Repeat("█", filled)) +
			ui.StyleDimmed.Render(strings.Repeat("░", barWidth-filled))

		dstr := "—"
		if m.dur > 0 {
			dstr = formatDur(m.dur)
		}
		fmt.Printf("  %s  %s  %s  %s\n",
			ui.StyleDimmed.Render(fmt.Sprintf("%-4s", m.month.String()[:3])),
			bar,
			ui.StyleElapsed.Render(fmt.Sprintf("%-9s", dstr)),
			ui.StyleDimmed.Render(fmt.Sprintf("%d sessions", m.sessions)),
		)
	}

	fmt.Println()
	fmt.Println("  " + sep)
	fmt.Printf("  %s  %s total  ·  %d sessions  ·  %d active days\n\n",
		ui.StyleDimmed.Render(fmt.Sprintf("%d", now.Year())),
		ui.StyleElapsed.Render(formatDur(yearTotal)),
		yearSessions,
		len(yearActiveDays),
	)
	return nil
}

// ─── table rendering helpers ─────────────────────────────────────────────────

var (
	colDate     = lipgloss.NewStyle().Width(11).Foreground(ui.ColorMuted)
	colTime     = lipgloss.NewStyle().Width(6).Foreground(ui.ColorMuted)
	colDuration = lipgloss.NewStyle().Width(10).Foreground(ui.ColorPrimary).Bold(true)
	colTask     = lipgloss.NewStyle().Width(30).Foreground(ui.ColorSecondary)
	colMessage  = lipgloss.NewStyle().Foreground(ui.ColorMuted)
)

func printTableHeader() {
	sep := ui.StyleDimmed.Render(strings.Repeat("─", 80))
	header := fmt.Sprintf("  %s  %s  %s  %s",
		colDate.Render("date"),
		colTime.Render("time"),
		colDuration.Render("duration"),
		colTask.Render("task"),
	)
	fmt.Println(sep)
	fmt.Println(ui.StyleDimmed.Render(header))
	fmt.Println(sep)
}

func printSessionRow(s *db.Session, d time.Duration) {
	date := s.StartTime.Local().Format("Mon Jan 02")
	startClock := s.StartTime.Local().Format("15:04")
	status := "■"
	if s.EndTime == nil {
		status = ui.StyleSuccess.Render("▶")
	}
	taskTrunc := s.TaskName
	if len(taskTrunc) > 28 {
		taskTrunc = taskTrunc[:25] + "..."
	}
	line := fmt.Sprintf("  %s  %s  %s  %s  %s  %s",
		colDate.Render(date),
		colTime.Render(startClock),
		colDuration.Render(formatDur(d)),
		colTask.Render(taskTrunc),
		status,
		colMessage.Render(truncate(s.Message, 25)),
	)
	fmt.Println(line)
	var badges string
	if s.Project != "" {
		badges += ui.StyleHighlight.Render("@"+s.Project) + " "
	}
	for _, t := range s.Tags {
		badges += ui.StyleTag.Render(t) + " "
	}
	if badges != "" {
		fmt.Printf("  %s\n", strings.TrimSpace(badges))
	}
}

func printTotals(count int, total time.Duration) {
	fmt.Printf("  %s  %s sessions  ·  %s total\n",
		ui.StyleDimmed.Render("total"),
		ui.StyleHighlight.Render(fmt.Sprintf("%d", count)),
		ui.StyleElapsed.Render(formatDur(total)),
	)
}

func formatDur(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func init() {
	historyCmd.Flags().BoolP("week", "w", false, "show this week")
	historyCmd.Flags().BoolP("month", "m", false, "show this month")
	historyCmd.Flags().BoolP("year", "y", false, "show this year")
	historyCmd.Flags().BoolP("day", "d", false, "show today (default)")
	historyCmd.Flags().IntP("limit", "n", 0, "show last N sessions as a table")
	historyCmd.Flags().Float64P("last", "l", 0, "show last N hours")
	historyCmd.Flags().BoolP("notes", "v", false, "show checkpoint notes (table view)")
	historyCmd.Flags().StringP("project", "p", "", "filter by project")
	rootCmd.AddCommand(historyCmd)
}
