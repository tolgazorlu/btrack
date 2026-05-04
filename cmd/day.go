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

var dayCmd = &cobra.Command{
	Use:   "day [today|yesterday|YYYY-MM-DD]",
	Short: "Show all sessions for a day in a tree view",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runDay,
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

	fmt.Println()
	fmt.Printf("  %s  %s%s\n", ui.StyleTitle.Render("btrack"), ui.StyleHighlight.Render(dateLabel), suffix)
	fmt.Println("  " + ui.StyleDimmed.Render(strings.Repeat("─", 58)))

	if len(sessions) == 0 {
		fmt.Println(ui.StyleSubtle.Render("\n  no sessions recorded for this day\n"))
		return nil
	}

	var totalDur time.Duration
	for i, sess := range sessions {
		d := sess.Duration()
		totalDur += d

		isLast := i == len(sessions)-1
		prefix := "  ├─"
		childPrefix := "  │  "
		if isLast {
			prefix = "  └─"
			childPrefix = "     "
		}

		// Session header line
		startStr := sess.StartTime.Local().Format("15:04")
		endStr := "…"
		if sess.EndTime != nil {
			endStr = sess.EndTime.Local().Format("15:04")
		}
		timeRange := ui.StyleDimmed.Render(fmt.Sprintf("%s–%s", startStr, endStr))

		taskStyle := ui.StyleHighlight
		durStyle := ui.StyleElapsed

		taskStr := sess.TaskName
		if len(taskStr) > 32 {
			taskStr = taskStr[:29] + "..."
		}

		fmt.Printf("%s %s  %s  %s\n",
			ui.StyleDimmed.Render(prefix),
			taskStyle.Render(fmt.Sprintf("%-33s", taskStr)),
			timeRange,
			durStyle.Render(formatDur(d)),
		)

		// Tags
		if len(sess.Tags) > 0 {
			var tags string
			for _, t := range sess.Tags {
				tags += ui.StyleTag.Render(t) + " "
			}
			fmt.Printf("%s %s\n", ui.StyleDimmed.Render(childPrefix), strings.TrimSpace(tags))
		}

		// Log entries
		logs, err := store.GetAllLogs(sess.ID)
		if err == nil && len(logs) > 0 {
			for j, log := range logs {
				isLastLog := j == len(logs)-1
				logPrefix := childPrefix + "├─"
				if isLastLog {
					logPrefix = childPrefix + "└─"
				}
				ts := log.Timestamp.Local().Format("15:04")
				fmt.Printf("%s %s  %s\n",
					ui.StyleDimmed.Render(logPrefix),
					ui.StyleDimmed.Render(ts),
					ui.StyleLogEntry.Render(log.Note),
				)
			}
		}

		if !isLast {
			fmt.Printf("%s\n", ui.StyleDimmed.Render("  │"))
		}
	}

	fmt.Println("  " + ui.StyleDimmed.Render(strings.Repeat("─", 58)))

	pct := int(totalDur.Hours() / float64(dailyHours) * 100)
	if pct > 100 {
		pct = 100
	}
	fmt.Printf("  %s  %s total  ·  %d sessions  ·  %s target (%d%%)\n\n",
		ui.StyleDimmed.Render("summary"),
		ui.StyleElapsed.Render(formatDur(totalDur)),
		len(sessions),
		ui.StyleDimmed.Render(fmt.Sprintf("%dh", dailyHours)),
		pct,
	)

	return nil
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Local().Date()
	by, bm, bd := b.Local().Date()
	return ay == by && am == bm && ad == bd
}

func init() {
	rootCmd.AddCommand(dayCmd)
}
