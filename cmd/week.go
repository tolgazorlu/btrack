package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/db"
	gh "github.com/tolgazorlu/btrack/internal/github"
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

	// One GitHub fetch for the whole week.
	var weekGH map[string]*gh.Activity
	if ghClient := ghClientFromConfig(cfg); ghClient != nil {
		if act, err := ghClient.GetActivity(weekStart.UTC(), now.UTC()); err == nil {
			weekGH = gh.SplitByDay(act)
		}
	}

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

		var dayGH *gh.Activity
		if weekGH != nil {
			dayGH = weekGH[day.Format("2006-01-02")]
		}

		sessions, _ := store.GetSessionsForDate(day)

		isToday := sameDay(day, now)
		label := day.Format("Mon Jan 02")
		todayMark := ""
		if isToday {
			todayMark = ui.StyleDimmed.Render("  ← today")
		}

		if len(sessions) == 0 {
			ghBadge := ghDayBadge(dayGH)
			fmt.Printf("\n  %s%s%s\n", ui.StyleDimmed.Render(label), todayMark, ghBadge)
			if dayGH == nil || dayGH.IsEmpty() {
				fmt.Printf("  %s\n", ui.StyleDimmed.Render("  (no sessions)"))
			}
			continue
		}

		activeDays++
		var dayTotal time.Duration
		for _, sess := range sessions {
			dayTotal += sess.Duration()
		}
		weekTotal += dayTotal
		weekSessions += len(sessions)

		ghBadge := ghDayBadge(dayGH)
		fmt.Printf("\n  %s  %s%s%s\n",
			ui.StyleHighlight.Render(label),
			ui.StyleElapsed.Render(formatDur(dayTotal)),
			todayMark,
			ghBadge,
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

// ghDayBadge returns a compact inline GitHub summary, e.g. "  ·  3 commits  1 PRs"
func ghDayBadge(act *gh.Activity) string {
	if act == nil || act.IsEmpty() {
		return ""
	}
	var parts []string
	if n := len(act.Commits); n > 0 {
		parts = append(parts, fmt.Sprintf("%d commits", n))
	}
	prs, reviews := 0, 0
	for _, pr := range act.PullRequests {
		if pr.Action == "opened" || pr.Action == "merged" {
			prs++
		} else if pr.Action == "reviewed" {
			reviews++
		}
	}
	if prs > 0 {
		parts = append(parts, fmt.Sprintf("%d PRs", prs))
	}
	if reviews > 0 {
		parts = append(parts, fmt.Sprintf("%d reviews", reviews))
	}
	if len(parts) == 0 {
		return ""
	}
	return ui.StyleDimmed.Render("  ·  " + strings.Join(parts, "  "))
}

func init() {
	rootCmd.AddCommand(weekCmd)
}
