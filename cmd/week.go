package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/db"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var weekCmd = &cobra.Command{
	Use:     "week",
	Aliases: []string{"wk"},
	Short:   "Show this week's sessions day by day as a tree",
	Long: `Show all sessions for the current week, grouped by day.

Usage:
  btrack week
  btrack wk     (short alias)

What you'll see:
  · Each day as a section with all its sessions
  · Notes indented under each session
  · Daily progress bar toward your work target
  · Weekly summary at the bottom

Tips:
  · Set your daily target with: btrack config hours 8
  · Use btrack d <date> to drill into a specific day`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		// Find Monday of current week.
		now := time.Now()
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7 // Sunday → 7
		}
		monday := now.AddDate(0, 0, -(weekday - 1))
		y, m, d := monday.Local().Date()
		weekStart := time.Date(y, m, d, 0, 0, 0, 0, time.Local)

		sep := ui.StyleDimmed.Render(strings.Repeat("─", 60))

		fmt.Println()
		weekLabel := fmt.Sprintf("week of %s", weekStart.Format("January 02"))
		fmt.Printf("  %s  %s\n", ui.StyleTitle.Render("btrack"), ui.StyleHighlight.Render(weekLabel))
		fmt.Println("  " + sep)

		var weekTotal time.Duration
		var weekSessions int
		activeDays := 0

		for i := 0; i < 7; i++ {
			day := weekStart.AddDate(0, 0, i)
			if day.After(now) {
				break
			}

			sessions, err := store.GetSessionsForDate(day)
			if err != nil || len(sessions) == 0 {
				// Show empty day briefly
				isToday := sameDay(day, now)
				label := day.Format("Mon Jan 02")
				todayMark := ""
				if isToday {
					todayMark = ui.StyleDimmed.Render("  ← today")
				}
				fmt.Printf("\n  %s%s\n",
					ui.StyleDimmed.Render(label),
					todayMark,
				)
				fmt.Printf("  %s\n", ui.StyleDimmed.Render("  (no sessions)"))
				continue
			}

			activeDays++
			var dayTotal time.Duration
			for _, sess := range sessions {
				dayTotal += sess.Duration()
			}
			weekTotal += dayTotal
			weekSessions += len(sessions)

			isToday := sameDay(day, now)
			label := day.Format("Mon Jan 02")
			todayMark := ""
			if isToday {
				todayMark = ui.StyleDimmed.Render("  ← today")
			}

			fmt.Printf("\n  %s  %s%s\n",
				ui.StyleHighlight.Render(label),
				ui.StyleElapsed.Render(formatDur(dayTotal)),
				todayMark,
			)

			for j, sess := range sessions {
				isLast := j == len(sessions)-1
				branch := "  ├─"
				childPfx := "  │  "
				if isLast {
					branch = "  └─"
					childPfx = "     "
				}

				startStr := sess.StartTime.Local().Format("15:04")
				endStr := "…"
				if sess.EndTime != nil {
					endStr = sess.EndTime.Local().Format("15:04")
				}
				timeRange := ui.StyleDimmed.Render(fmt.Sprintf("%s–%s", startStr, endStr))

				taskStr := sess.TaskName
				if len(taskStr) > 30 {
					taskStr = taskStr[:27] + "..."
				}

				fmt.Printf("%s %s  %s  %s\n",
					ui.StyleDimmed.Render(branch),
					ui.StyleHighlight.Render(fmt.Sprintf("%-31s", taskStr)),
					timeRange,
					ui.StyleElapsed.Render(formatDur(sess.Duration())),
				)

				logs, err := store.GetAllLogs(sess.ID)
				if err == nil {
					for k, log := range logs {
						isLastLog := k == len(logs)-1
						logBranch := childPfx + "├─"
						if isLastLog {
							logBranch = childPfx + "└─"
						}
						fmt.Printf("%s %s  %s\n",
							ui.StyleDimmed.Render(logBranch),
							ui.StyleDimmed.Render(log.Timestamp.Local().Format("15:04")),
							ui.StyleLogEntry.Render(log.Note),
						)
					}
				}
			}

			// Daily progress bar
			fmt.Println()
			fmt.Println("  " + ui.RenderProgressBar(dayTotal, dailyHours))
		}

		fmt.Println()
		fmt.Println("  " + sep)

		pct := 0
		targetTotal := time.Duration(activeDays) * time.Duration(dailyHours) * time.Hour
		if targetTotal > 0 {
			pct = int(weekTotal * 100 / targetTotal)
			if pct > 100 {
				pct = 100
			}
		}
		fmt.Printf("  %s  %s total  ·  %d sessions  ·  %d active days  ·  %d%% of weekly target\n\n",
			ui.StyleDimmed.Render("week"),
			ui.StyleElapsed.Render(formatDur(weekTotal)),
			weekSessions,
			activeDays,
			pct,
		)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(weekCmd)
}
