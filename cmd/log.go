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
	Aliases: []string{"log"},
	Short:   "Add a checkpoint note to the active session",
	Long: `Add a note to the currently running session.

Examples:
  btrack note "reproduced the bug on staging"
  btrack note "tried approach A, didn't work"
  btrack note "found root cause: JWT clock skew"

Tips:
  · Notes appear in: btrack day (tree view)
  · AI uses your notes to write standup summaries
  · Add as many as you want — they tell the story of your work`,
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
