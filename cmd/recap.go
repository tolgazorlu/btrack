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

var recapCmd = &cobra.Command{
	Use:     "recap [today|yesterday|YYYY-MM-DD]",
	Aliases: []string{"rc"},
	Short:   "Standup-ready summary of your day",
	Long: `Format your sessions as a clean standup bullet list.
Defaults to yesterday — perfect for morning standups.

Examples:
  btrack recap              (yesterday — default)
  btrack recap today
  btrack recap 2026-05-01`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRecap,
}

func runRecap(cmd *cobra.Command, args []string) error {
	target := time.Now().AddDate(0, 0, -1) // default: yesterday

	if len(args) == 1 {
		switch args[0] {
		case "today":
			target = time.Now()
		case "yesterday":
			// keep default
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

	dayLabel := dateLabel
	if sameDay(target, time.Now()) {
		dayLabel = "Today"
	} else if sameDay(target, time.Now().AddDate(0, 0, -1)) {
		dayLabel = "Yesterday"
	}

	sep := ui.StyleDimmed.Render(strings.Repeat("─", 58))

	fmt.Println()
	fmt.Printf("  %s  %s  %s\n",
		ui.StyleTitle.Render("btrack"),
		ui.StyleHighlight.Render("recap"),
		ui.StyleDimmed.Render(dateLabel),
	)
	fmt.Println("  " + sep)

	if len(sessions) == 0 {
		fmt.Println(ui.StyleSubtle.Render("\n  no sessions recorded for this day\n"))
		return nil
	}

	fmt.Printf("\n  %s\n\n", ui.StyleHighlight.Render(dayLabel+":"))

	var totalDur time.Duration
	for _, sess := range sessions {
		dur := sess.Duration()
		totalDur += dur

		line := fmt.Sprintf("  %s %s", ui.StyleDimmed.Render("·"), ui.StyleHighlight.Render(sess.TaskName))
		if sess.Project != "" {
			line += "  " + ui.StyleTag.Render("@"+sess.Project)
		}
		line += "  " + ui.StyleElapsed.Render(formatDur(dur))
		fmt.Println(line)

		// Show top-level notes indented under the task
		logs, err := store.GetAllLogs(sess.ID)
		if err == nil {
			for _, log := range logs {
				if log.ParentID == nil {
					fmt.Printf("    %s %s\n",
						ui.StyleDimmed.Render("↳"),
						ui.StyleLogEntry.Render(log.Note),
					)
				}
			}
		}
	}

	fmt.Println()
	fmt.Println("  " + sep)
	fmt.Printf("  %s total  ·  %d sessions\n\n",
		ui.StyleElapsed.Render(formatDur(totalDur)),
		len(sessions),
	)

	return nil
}

func init() {
	rootCmd.AddCommand(recapCmd)
}
