package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/daemon"
	"github.com/tolgazorlu/btrack/internal/db"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the active session with live elapsed time",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
		dailyHours := 8
		if cfg != nil && cfg.Work.DailyHours > 0 {
			dailyHours = cfg.Work.DailyHours
		}

		client := daemon.NewClient()
		model := ui.NewStatusModel(client, dailyHours)

		p := tea.NewProgram(model, tea.WithAltScreen())
		_, err := p.Run()
		return err
	},
}

var statusDayCmd = &cobra.Command{
	Use:   "day",
	Short: "Show day-by-day session summary",
	RunE:  runStatusDay,
}

func runStatusDay(cmd *cobra.Command, args []string) error {
	days, _ := cmd.Flags().GetInt("days")

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

	sessions, err := store.GetRecentSessions(days * 20)
	if err != nil {
		return fmt.Errorf("load sessions: %w", err)
	}

	stats := db.ComputeStats(sessions, days)

	sep := ui.StyleDimmed.Render(strings.Repeat("─", 62))
	colDay := lipgloss.NewStyle().Width(13).Foreground(lipgloss.Color("#A9B1D6"))
	colDur := lipgloss.NewStyle().Width(8).Foreground(lipgloss.Color("#7AA2F7")).Bold(true)

	fmt.Println()
	fmt.Printf("  %s  %s\n", ui.StyleTitle.Render("btrack"), ui.StyleDimmed.Render("daily summary"))
	fmt.Println("  " + sep)

	const barWidth = 28
	for _, day := range stats.DailyBreakdown {
		label := day.Date.Format("Mon Jan 02")

		var bar string
		var durStr string
		var marker string

		if day.Sessions == 0 {
			bar = ui.StyleDimmed.Render(strings.Repeat("░", barWidth))
			durStr = ui.StyleDimmed.Render("  —   ")
		} else {
			pct := day.Duration.Hours() / float64(dailyHours)
			if pct > 1 {
				pct = 1
			}
			filled := int(float64(barWidth) * pct)
			barColor := lipgloss.Color("#9ECE6A") // green
			if day.Duration.Hours() >= float64(dailyHours) {
				barColor = lipgloss.Color("#7AA2F7") // blue = hit target
				marker = ui.StyleSuccess.Render(" ✓")
			} else if pct >= 0.75 {
				barColor = lipgloss.Color("#E0AF68") // yellow = close
			}
			bar = lipgloss.NewStyle().Foreground(barColor).Render(strings.Repeat("█", filled)) +
				ui.StyleDimmed.Render(strings.Repeat("░", barWidth-filled))
			durStr = colDur.Render(formatDur(day.Duration))
		}

		todayMark := ""
		if day.IsToday {
			todayMark = ui.StyleDimmed.Render(" ← today")
		}

		sessStr := fmt.Sprintf("%d session", day.Sessions)
		if day.Sessions != 1 {
			sessStr += "s"
		}

		fmt.Printf("  %s  %s  %s  %s%s%s\n",
			colDay.Render(label),
			bar,
			durStr,
			ui.StyleDimmed.Render(sessStr),
			marker,
			todayMark,
		)
	}

	fmt.Println("  " + sep)

	avgDur := time.Duration(0)
	if stats.TotalSessions > 0 {
		avgDur = stats.TotalDuration / time.Duration(stats.TotalSessions)
	}
	fmt.Printf("  %s  %s total  ·  %s avg/session  ·  %s daily target\n\n",
		ui.StyleDimmed.Render("summary"),
		ui.StyleHighlight.Render(formatDur(stats.TotalDuration)),
		ui.StyleDimmed.Render(formatDur(avgDur)),
		ui.StyleDimmed.Render(fmt.Sprintf("%dh", dailyHours)),
	)

	return nil
}

func init() {
	statusDayCmd.Flags().IntP("days", "n", 7, "number of days to show")
	statusCmd.AddCommand(statusDayCmd)
	rootCmd.AddCommand(statusCmd)
}
