package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/daemon"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var logCmd = &cobra.Command{
	Use:     "note <text>",
	Aliases: []string{"n", "log"},
	Short:   "Add a checkpoint note to the active session",
	Long: `Add a timestamped note to the currently running session.

Requires an active session — start one with: btrack s "task"

Usage:
  btrack note "text"
  btrack n "text"      (short alias)

Examples:
  btrack n "reproduced the bug on staging"
  btrack n "tried approach A, didn't work"
  btrack n "found root cause: JWT clock skew"
  btrack n "PR ready for review"

Tips:
  · Notes appear as a tree under their session in: btrack d
  · AI uses your notes to write better standup summaries
  · Add as many as you want — they tell the story of your work
  · No active session? Start one first: btrack s "task name"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		note := strings.Join(args, " ")

		payload := daemon.LogPayload{Note: note}
		client := daemon.NewClient()
		resp, err := client.Send(daemon.ActionLog, payload)
		if err != nil {
			return err
		}
		if !resp.Success {
			return fmt.Errorf("%s", resp.Error)
		}

		fmt.Printf("  %s  %s\n",
			ui.StyleSuccess.Render("✓"),
			ui.StyleHighlight.Render(note),
		)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(logCmd)
}
