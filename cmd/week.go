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

// weekCmd is kept for backward compatibility but hidden — use: btrack h -w
var weekCmd = &cobra.Command{
	Use:     "week",
	Aliases: []string{"wk"},
	Hidden:  true,
	Short:   "Show this week's sessions (use: btrack h -w)",
	RunE:    runWeek,
}

func runWeek(cmd *cobra.Command, args []string) error {
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
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	monday := now.AddDate(0, 0, -(weekday - 1))
	y, m, d := monday.Local().Date()
	weekStart := time.Date(y, m, d, 0, 0, 0, 0, time.Local)

	sep := ui.StyleDimmed.Render(strings.Repeat("─", 60))
	fmt.Println()
	fmt.Printf("  %s  %s\n", ui.StyleTitle.Render("btrack"), ui.StyleHighlight.Render("week of "+weekStart.Format("January 02")))
	fmt.Println("  " + sep)

	var weekTotal time.Duration
	var weekSessions, activeDays int

	for i := 0; i < 7; i++ {
		day := weekStart.AddDate(0, 0, i)
		if day.After(now) {
			break
		}

		sessions, _ := store.GetSessionsForDate(day)

		isToday := sameDay(day, now)
		label := day.Format("Mon Jan 02")
		todayMark := ""
		if isToday {
			todayMark = ui.StyleDimmed.Render("  ← today")
		}

		if len(sessions) == 0 {
			fmt.Printf("\n  %s%s\n", ui.StyleDimmed.Render(label), todayMark)
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

		fmt.Printf("\n  %s  %s%s\n",
			ui.StyleHighlight.Render(label),
			ui.StyleElapsed.Render(formatDur(dayTotal)),
			todayMark,
		)

		for j, sess := range sessions {
			isLast := j == len(sessions)-1
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
			if len(taskStr) > 30 {
				taskStr = taskStr[:27] + "..."
			}
			fmt.Printf("%s %s  %s  %s\n",
				ui.StyleDimmed.Render(branch),
				ui.StyleHighlight.Render(fmt.Sprintf("%-31s", taskStr)),
				ui.StyleDimmed.Render(fmt.Sprintf("%s–%s", startStr, endStr)),
				ui.StyleElapsed.Render(formatDur(sess.Duration())),
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
		}
		fmt.Println()
		fmt.Println("  " + ui.RenderProgressBar(dayTotal, dailyHours))
	}

	fmt.Println()
	fmt.Println("  " + sep)

	pct := 0
	if target := time.Duration(activeDays) * time.Duration(dailyHours) * time.Hour; target > 0 {
		pct = int(weekTotal * 100 / target)
		if pct > 100 {
			pct = 100
		}
	}
	fmt.Printf("  %s  %s total  ·  %d sessions  ·  %d active days  ·  %d%% of target\n\n",
		ui.StyleDimmed.Render("week"),
		ui.StyleElapsed.Render(formatDur(weekTotal)),
		weekSessions, activeDays, pct,
	)
	return nil
}

func init() {
	rootCmd.AddCommand(weekCmd)
}
