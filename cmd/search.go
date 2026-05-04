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

		sep := ui.StyleDimmed.Render(strings.Repeat("─", 72))

		fmt.Println()
		fmt.Printf("  %s  %s  %s\n",
			ui.StyleTitle.Render("btrack search"),
			ui.StyleHighlight.Render(`"`+query+`"`),
			ui.StyleDimmed.Render(fmt.Sprintf("(%d results)", len(sessions))),
		)
		fmt.Println("  " + sep)

		if len(sessions) == 0 {
			fmt.Printf("\n  %s\n\n", ui.StyleSubtle.Render("no sessions found"))
			return nil
		}

		var total time.Duration
		for _, s := range sessions {
			d := s.Duration()
			total += d

			date := s.StartTime.Local().Format("Mon Jan 02")
			startClock := s.StartTime.Local().Format("15:04")

			taskStr := highlight(s.TaskName, query)
			if len(s.TaskName) > 34 {
				taskStr = highlight(s.TaskName[:31]+"...", query)
			}

			fmt.Printf("  %s  %s  %-34s  %s\n",
				ui.StyleDimmed.Render(fmt.Sprintf("%-13s", date)),
				ui.StyleDimmed.Render(startClock),
				taskStr,
				ui.StyleElapsed.Render(formatDur(d)),
			)
			if s.Message != "" {
				msg := s.Message
				if len(msg) > 58 {
					msg = msg[:55] + "..."
				}
				fmt.Printf("  %s\n",
					ui.StyleDimmed.Render("               "+highlight(msg, query)),
				)
			}
		}

		fmt.Println("  " + sep)
		fmt.Printf("  %s  %s sessions  ·  %s total\n\n",
			ui.StyleDimmed.Render("found"),
			ui.StyleHighlight.Render(fmt.Sprintf("%d", len(sessions))),
			ui.StyleElapsed.Render(formatDur(total)),
		)
		return nil
	},
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
