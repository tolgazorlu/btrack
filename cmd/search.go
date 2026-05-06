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

var searchCmd = &cobra.Command{
	Use:     "search <query>",
	Aliases: []string{"find", "f"},
	Short:   "Search sessions by task name or message",
	Long: `Full-text search across task names and stop messages.

Usage:
  btrack search "query"
  btrack f "query"       (short alias)

Examples:
  btrack search "JWT"
  btrack f "auth middleware"
  btrack search "#bugfix"

Tips:
  · Search is case-insensitive
  · Searches both task names and stop messages
  · Results are sorted by most recent first`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.Join(args, " ")

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		store, err := db.Open(cfg)
		if err != nil {
			return err
		}
		defer store.Close()

		sessions, err := store.SearchSessions(query)
		if err != nil {
			return fmt.Errorf("search: %w", err)
		}

		ui.Header("search", `"`+query+`"  · `+fmt.Sprintf("%d results", len(sessions)))

		if len(sessions) == 0 {
			ui.Hint("no sessions match")
			ui.Blank()
			return nil
		}

		var total time.Duration
		for _, s := range sessions {
			d := s.Duration()
			total += d

			date := s.StartTime.Local().Format("Mon Jan 02")
			startClock := s.StartTime.Local().Format("15:04")

			task := s.TaskName
			if len(task) > 34 {
				task = task[:31] + "..."
			}

			fmt.Printf("%s%s  %s  %s  %s\n",
				ui.Indent,
				ui.StyleDimmed.Render(fmt.Sprintf("%-13s", date)),
				ui.StyleDimmed.Render(startClock),
				padVisible(highlight(task, query), 34),
				ui.StyleElapsed.Render(formatDur(d)),
			)
			if s.Message != "" {
				msg := s.Message
				if len(msg) > 58 {
					msg = msg[:55] + "..."
				}
				fmt.Printf("%s              %s\n",
					ui.Indent,
					ui.StyleDimmed.Render(highlight(msg, query)),
				)
			}
		}

		ui.Rule()
		fmt.Printf("%s%s  %s\n",
			ui.Indent,
			ui.StyleElapsed.Render(formatDur(total)),
			ui.StyleDimmed.Render(fmt.Sprintf("· %d sessions", len(sessions))),
		)
		ui.Blank()
		return nil
	},
}

// padVisible right-pads a string (containing ANSI escapes) to a target visible width.
func padVisible(s string, width int) string {
	visible := 0
	in := false
	for _, r := range s {
		if r == 0x1b {
			in = true
			continue
		}
		if in {
			if r == 'm' {
				in = false
			}
			continue
		}
		visible++
	}
	if visible >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visible)
}

// highlight wraps the matched portion in a different style.
func highlight(text, query string) string {
	lower := strings.ToLower(text)
	lq := strings.ToLower(query)
	idx := strings.Index(lower, lq)
	if idx < 0 {
		return ui.StyleHighlight.Render(text)
	}
	before := text[:idx]
	match := text[idx : idx+len(query)]
	after := text[idx+len(query):]
	return ui.StyleHighlight.Render(before) +
		ui.StyleSuccess.Render(match) +
		ui.StyleHighlight.Render(after)
}

func init() {
	rootCmd.AddCommand(searchCmd)
}
