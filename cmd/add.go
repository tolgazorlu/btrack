package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/db"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var addCmd = &cobra.Command{
	Use:     "add <task>",
	Aliases: []string{"a"},
	Short:   "Record a completed session with explicit start and end times",
	Long: `Record one finished session in a single call — no timer involved.

Made for AI assistants and quick backfills: note the wall-clock time when
the work started and when it ended, and btrack computes the duration.

Times accept HH:MM (today, or --date's day) or a full timestamp:

  btrack add "fix JWT clock skew" --from 14:03 --to 15:10 -p myapp
  btrack add "code review" --from 09:30 --to 10:00 --date 2026-09-16
  btrack add "deploy hotfix" --from "2026-09-16 22:10" --to "2026-09-16 22:40"

Instead of --to you can give --for with a duration:

  btrack add "standup prep" --from 08:45 --for 25m

Usage:
  btrack add <task> --from <time> --to <time> [--date D] [-p project] [-m message]`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		task := strings.Join(args, " ")
		fromRaw, _ := cmd.Flags().GetString("from")
		toRaw, _ := cmd.Flags().GetString("to")
		forRaw, _ := cmd.Flags().GetString("for")
		dateRaw, _ := cmd.Flags().GetString("date")
		project, _ := cmd.Flags().GetString("project")
		message, _ := cmd.Flags().GetString("message")

		if fromRaw == "" {
			return fmt.Errorf("--from is required (when the work started, e.g. 14:03)")
		}
		if toRaw == "" && forRaw == "" {
			return fmt.Errorf("give either --to (when it ended) or --for (how long it took)")
		}
		if toRaw != "" && forRaw != "" {
			return fmt.Errorf("--to and --for are mutually exclusive")
		}

		day := time.Now()
		if dateRaw != "" {
			d, err := parseReportDate(dateRaw, time.Now())
			if err != nil {
				return fmt.Errorf("--date: %w", err)
			}
			day = d
		}

		start, err := parseAddTime(fromRaw, day)
		if err != nil {
			return fmt.Errorf("--from: %w", err)
		}

		var end time.Time
		if toRaw != "" {
			end, err = parseAddTime(toRaw, day)
			if err != nil {
				return fmt.Errorf("--to: %w", err)
			}
			if !end.After(start) {
				// Session crossed midnight: 23:40 → 00:20.
				end = end.Add(24 * time.Hour)
			}
		} else {
			dur, err := ParseStopwatch(forRaw)
			if err != nil {
				return fmt.Errorf("--for: %w", err)
			}
			end = start.Add(dur)
		}

		if end.Sub(start) > 16*time.Hour {
			return fmt.Errorf("session is longer than 16h (%s – %s) — check the times",
				start.Format("02.01 15:04"), end.Format("02.01 15:04"))
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

		if project == "" {
			if cwd, err := os.Getwd(); err == nil {
				if pf, _ := config.FindProjectFile(cwd); pf != nil {
					project = pf.Project
				}
			}
		}

		sess := &db.Session{
			TaskName:  task,
			StartTime: start,
			EndTime:   &end,
			Message:   message,
			Project:   project,
		}
		if err := store.CreateSession(sess); err != nil {
			return fmt.Errorf("create session: %w", err)
		}

		ui.Blank()
		ui.OK("added " + ui.StyleHighlight.Render(task))
		ui.KV("when", start.Format("02.01.2006 15:04")+" – "+end.Format("15:04"))
		ui.KV("duration", formatDur(end.Sub(start)))
		if project != "" {
			ui.KV("project", "@"+project)
		}
		ui.Blank()
		return nil
	},
}

// parseAddTime reads HH:MM (interpreted on day's date) or a full timestamp.
func parseAddTime(s string, day time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse("15:04", s); err == nil {
		return time.Date(day.Year(), day.Month(), day.Day(),
			t.Hour(), t.Minute(), 0, 0, time.Local), nil
	}
	for _, layout := range []string{
		"2006-01-02 15:04",
		"2006-01-02T15:04",
		time.RFC3339,
		"02.01.2006 15:04",
	} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised time %q — use HH:MM or \"2006-01-02 15:04\"", s)
}

func init() {
	addCmd.Flags().String("from", "", "when the work started (HH:MM or full timestamp)")
	addCmd.Flags().String("to", "", "when the work ended (HH:MM or full timestamp)")
	addCmd.Flags().String("for", "", "duration instead of --to (45m, 1:30:00, 1.05.00)")
	addCmd.Flags().String("date", "", "day for HH:MM times (default today; 21.08 or 2026-08-21)")
	addCmd.Flags().StringP("project", "p", "", "assign the session to a project")
	addCmd.Flags().StringP("message", "m", "", "closing message for the session")
	rootCmd.AddCommand(addCmd)
}
