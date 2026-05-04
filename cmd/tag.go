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

var tagCmd = &cobra.Command{
	Use:   "tag <#tag>",
	Short: "Filter history by tag",
	Long: `Show all past sessions that have a specific tag.

Usage:
  btrack tag #bugfix
  btrack tag bugfix    (# prefix is optional)

Examples:
  btrack tag #bugfix
  btrack tag #feature
  btrack tag #test

How tags are added:
  1. Manually in your stop message:
       btrack x -m "fixed login redirect #bugfix"
  2. Auto-detected from keywords (fix/feat/refactor/test/doc):
       btrack x -m "fixed the JWT issue"  ->  auto-adds #bugfix
  3. Auto-extracted from GitHub commits when using: btrack github sync

Common tags: #bugfix #feature #test #docs #refactor #ci

Tips:
  · See all your tags with a count: btrack ai insights
  · Filter history to see all work in a category: btrack tag #feature`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tag := strings.ToLower(args[0])
		if !strings.HasPrefix(tag, "#") {
			tag = "#" + tag
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

		sessions, err := store.GetRecentSessions(500)
		if err != nil {
			return fmt.Errorf("load sessions: %w", err)
		}

		var matched []*db.Session
		for _, s := range sessions {
			for _, t := range s.Tags {
				if t == tag {
					matched = append(matched, s)
					break
				}
			}
		}

		if len(matched) == 0 {
			fmt.Printf("\n  %s\n\n",
				ui.StyleSubtle.Render("no sessions found with tag "+tag),
			)
			return nil
		}

		sep := ui.StyleDimmed.Render(strings.Repeat("─", 70))
		fmt.Println()
		fmt.Printf("  %s  %s\n",
			ui.StyleTitle.Render("btrack tag"),
			ui.StyleTag.Render(tag),
		)
		fmt.Println("  " + sep)

		var total time.Duration
		for _, s := range matched {
			d := s.Duration()
			total += d

			date := s.StartTime.Local().Format("Mon Jan 02")
			startClock := s.StartTime.Local().Format("15:04")

			taskStr := s.TaskName
			if len(taskStr) > 32 {
				taskStr = taskStr[:29] + "..."
			}

			fmt.Printf("  %s  %s  %s  %s\n",
				ui.StyleDimmed.Render(fmt.Sprintf("%-13s", date)),
				ui.StyleDimmed.Render(startClock),
				ui.StyleHighlight.Render(fmt.Sprintf("%-33s", taskStr)),
				ui.StyleElapsed.Render(formatDur(d)),
			)
			if s.Message != "" {
				msg := s.Message
				if len(msg) > 55 {
					msg = msg[:52] + "..."
				}
				fmt.Printf("  %s\n",
					ui.StyleDimmed.Render("              "+msg),
				)
			}
		}

		fmt.Println("  " + sep)
		fmt.Printf("  %s  %s sessions  ·  %s total\n\n",
			ui.StyleDimmed.Render("tag "+tag),
			ui.StyleHighlight.Render(fmt.Sprintf("%d", len(matched))),
			ui.StyleElapsed.Render(formatDur(total)),
		)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(tagCmd)
}
