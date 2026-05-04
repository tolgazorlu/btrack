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
	Use:     "history",
	Aliases: []string{"h", "hist"},
	Short:   "Show past sessions in a table",
	Long: `List your past tracking sessions in a table view.

Usage:
  btrack history
  btrack h         (short alias)

Examples:
  btrack h
  btrack h -n 50
  btrack h -v        (includes notes under each session)

Flags:
  -n, --limit   Number of sessions to show (default 20)
  -v, --notes   Also show checkpoint notes under each session

Tips:
  · For a tree view of a single day, use: btrack d
  · For AI analysis of your patterns, use: btrack ai insights`,
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		showNotes, _ := cmd.Flags().GetBool("notes")

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		store, err := db.Open(cfg)
		if err != nil {
			return err
		}
		defer store.Close()

		sessions, err := store.GetRecentSessions(limit)
		if err != nil {
			return fmt.Errorf("load sessions: %w", err)
		}
		if len(sessions) == 0 {
			fmt.Println(ui.StyleSubtle.Render("\n  no sessions recorded yet\n"))
			fmt.Println(ui.StyleDimmed.Render("  start one with: btrack start <task>\n"))
			return nil
		}

		fmt.Println()
		printHeader()

		var totalDuration time.Duration
		for _, s := range sessions {
			d := s.Duration()
			totalDuration += d
			printSession(s, d)

			if showNotes {
				logs, err := store.GetAllLogs(s.ID)
				if err == nil && len(logs) > 0 {
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
		printTotals(len(sessions), totalDuration)
		fmt.Println()
		return nil
	},
}

func init() {
	historyCmd.Flags().IntP("limit", "n", 20, "number of sessions to show")
	historyCmd.Flags().BoolP("notes", "v", false, "also show checkpoint notes")
	rootCmd.AddCommand(historyCmd)
}

var (
	colDate     = lipgloss.NewStyle().Width(11).Foreground(lipgloss.Color("#565F89"))
	colTime     = lipgloss.NewStyle().Width(6).Foreground(lipgloss.Color("#565F89"))
	colDuration = lipgloss.NewStyle().Width(10).Foreground(lipgloss.Color("#7AA2F7")).Bold(true)
	colTask     = lipgloss.NewStyle().Width(30).Foreground(lipgloss.Color("#A9B1D6"))
	colMessage  = lipgloss.NewStyle().Foreground(lipgloss.Color("#565F89"))
)

func printHeader() {
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

func printSession(s *db.Session, d time.Duration) {
	date := s.StartTime.Local().Format("Mon Jan 02")
	startClock := s.StartTime.Local().Format("15:04")

	status := "■"
	if s.EndTime == nil {
		status = ui.StyleSuccess.Render("▶") // still running
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

	if len(s.Tags) > 0 {
		tags := ""
		for _, t := range s.Tags {
			tags += ui.StyleTag.Render(t) + " "
		}
		fmt.Printf("  %s\n", strings.TrimSpace(tags))
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
