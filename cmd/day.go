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

var dayCmd = &cobra.Command{
	Use:     "day [today|yesterday|YYYY-MM-DD]",
	Aliases: []string{"d"},
	Short:   "Show all sessions for a day as a tree",
	Long: `Show all sessions for a day in a tree view with notes and progress.

Usage:
  btrack day
  btrack d       (short alias)

Examples:
  btrack d               (today)
  btrack d yesterday
  btrack d 2026-05-01

What you'll see:
  · Each session as a branch with time range and duration
  · Notes indented under their session
  · Progress bar toward your daily hour target
  · Summary: total hours, sessions, target %

Tips:
  · Set your daily target with: btrack config hours 8
  · Add notes while working with: btrack n "what you found"`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDay,
}

func runDay(cmd *cobra.Command, args []string) error {
	target := time.Now()

	if len(args) == 1 {
		switch args[0] {
		case "today", "":
			// default
		case "yesterday":
			target = time.Now().AddDate(0, 0, -1)
		default:
			t, err := time.ParseInLocation("2006-01-02", args[0], time.Local)
			if err != nil {
				return fmt.Errorf("invalid date %q — use YYYY-MM-DD, today, or yesterday", args[0])
			}
			target = t
		}
	}

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

	// Fetch GitHub activity for the day if connected (non-blocking: ignore errors).
	var ghActivity *gh.Activity
	if ghClient := ghClientFromConfig(cfg); ghClient != nil {
		dayStart := time.Date(target.Year(), target.Month(), target.Day(), 0, 0, 0, 0, time.Local).UTC()
		dayEnd := dayStart.Add(24 * time.Hour)
		if act, err := ghClient.GetActivity(dayStart, dayEnd); err == nil {
			ghActivity = act
		}
	}

	sessions, err := store.GetSessionsForDate(target)
	if err != nil {
		return fmt.Errorf("load sessions: %w", err)
	}

	y, m, d := target.Local().Date()
	dateLabel := time.Date(y, m, d, 0, 0, 0, 0, time.Local).Format("Monday, January 02 2006")

	isToday := sameDay(target, time.Now())
	isYesterday := sameDay(target, time.Now().AddDate(0, 0, -1))
	suffix := ""
	if isToday {
		suffix = ui.StyleDimmed.Render("  today")
	} else if isYesterday {
		suffix = ui.StyleDimmed.Render("  yesterday")
	}

	sep := ui.StyleDimmed.Render(strings.Repeat("─", 58))

	fmt.Println()
	fmt.Printf("  %s  %s%s\n", ui.StyleTitle.Render("btrack"), ui.StyleHighlight.Render(dateLabel), suffix)
	fmt.Println("  " + sep)

	if len(sessions) == 0 {
		fmt.Println(ui.StyleSubtle.Render("\n  no sessions recorded for this day\n"))
		return nil
	}

	var totalDur time.Duration
	for i, sess := range sessions {
		d := sess.Duration()
		totalDur += d

		isLast := i == len(sessions)-1
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
		if len(taskStr) > 32 {
			taskStr = taskStr[:29] + "..."
		}

		fmt.Printf("%s %s  %s  %s\n",
			ui.StyleDimmed.Render(branch),
			ui.StyleHighlight.Render(fmt.Sprintf("%-33s", taskStr)),
			timeRange,
			ui.StyleElapsed.Render(formatDur(d)),
		)

		if len(sess.Tags) > 0 {
			var tags string
			for _, t := range sess.Tags {
				tags += ui.StyleTag.Render(t) + " "
			}
			fmt.Printf("%s %s\n", ui.StyleDimmed.Render(childPfx), strings.TrimSpace(tags))
		}

		logs, err := store.GetAllLogs(sess.ID)
		if err == nil && len(logs) > 0 {
			for j, log := range logs {
				isLastLog := j == len(logs)-1
				logBranch := childPfx + "├─"
				if isLastLog {
					logBranch = childPfx + "└─"
				}
				ts := log.Timestamp.Local().Format("15:04")
				fmt.Printf("%s %s  %s\n",
					ui.StyleDimmed.Render(logBranch),
					ui.StyleDimmed.Render(ts),
					ui.StyleLogEntry.Render(log.Note),
				)
			}
		}

		if !isLast {
			fmt.Printf("%s\n", ui.StyleDimmed.Render("  │"))
		}
	}

	fmt.Println()
	fmt.Println("  " + ui.RenderProgressBar(totalDur, dailyHours))

	pct := int(totalDur.Hours() / float64(dailyHours) * 100)
	if pct > 100 {
		pct = 100
	}
	fmt.Println("  " + sep)
	fmt.Printf("  %s  %s total  ·  %d sessions  ·  %s target  ·  %d%%\n\n",
		ui.StyleDimmed.Render("summary"),
		ui.StyleElapsed.Render(formatDur(totalDur)),
		len(sessions),
		ui.StyleDimmed.Render(fmt.Sprintf("%dh", dailyHours)),
		pct,
	)

	// GitHub activity section
	if ghActivity != nil && !ghActivity.IsEmpty() {
		printDayGitHubSummary(ghActivity)
	}

	return nil
}

func printDayGitHubSummary(activity *gh.Activity) {
	sep := ui.StyleDimmed.Render(strings.Repeat("─", 58))
	fmt.Printf("  %s\n", ui.StyleDimmed.Render("github"))
	fmt.Println("  " + sep)

	if len(activity.Commits) > 0 {
		fmt.Printf("  %s\n", ui.StyleDimmed.Render(fmt.Sprintf("commits  (%d)", len(activity.Commits))))
		for _, c := range activity.Commits {
			msg := c.Message
			if len(msg) > 55 {
				msg = msg[:52] + "..."
			}
			repo := c.Repo
			if idx := strings.Index(repo, "/"); idx != -1 {
				repo = repo[idx+1:]
			}
			fmt.Printf("  %s  %s  %s\n",
				ui.StyleSubtle.Render(c.SHA),
				ui.StyleDimmed.Render(msg),
				ui.StyleSubtle.Render(repo),
			)
		}
		fmt.Println()
	}

	if len(activity.PullRequests) > 0 {
		fmt.Printf("  %s\n", ui.StyleDimmed.Render(fmt.Sprintf("pull requests  (%d)", len(activity.PullRequests))))
		for _, pr := range activity.PullRequests {
			t := pr.Title
			if len(t) > 52 {
				t = t[:49] + "..."
			}
			var badge string
			switch pr.Action {
			case "merged":
				badge = ui.StyleSuccess.Render("[merged]")
			case "opened":
				badge = ui.StyleHighlight.Render("[opened]")
			case "reviewed":
				badge = ui.StyleWarning.Render("[reviewed]")
			default:
				badge = ui.StyleDimmed.Render("[" + pr.Action + "]")
			}
			fmt.Printf("  %s  %s\n", badge, ui.StyleDimmed.Render(t))
		}
		fmt.Println()
	}

	if len(activity.Issues) > 0 {
		fmt.Printf("  %s\n", ui.StyleDimmed.Render(fmt.Sprintf("issues  (%d)", len(activity.Issues))))
		for _, iss := range activity.Issues {
			t := iss.Title
			if len(t) > 55 {
				t = t[:52] + "..."
			}
			fmt.Printf("  %s  %s\n",
				ui.StyleDimmed.Render(fmt.Sprintf("[%s]", iss.Action)),
				ui.StyleDimmed.Render(t),
			)
		}
		fmt.Println()
	}
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Local().Date()
	by, bm, bd := b.Local().Date()
	return ay == by && am == bm && ad == bd
}

func init() {
	rootCmd.AddCommand(dayCmd)
}
