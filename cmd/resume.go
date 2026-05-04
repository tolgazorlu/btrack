package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/daemon"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var resumeCmd = &cobra.Command{
	Use:     "resume",
	Aliases: []string{"r"},
	Short:   "Resume the last stopped session",
	Long: `Start a new session copying the task name from your last session.

Usage:
  btrack resume
  btrack r        (short alias)

Examples:
  btrack r   (picks up where you left off)

Tips:
  · Useful after a break, lunch, or interruption
  · Copies the task name and git info from the previous session
  · Creates a new session — does not modify the old one`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := daemon.NewClient()
		resp, err := client.Send(daemon.ActionResume, nil)
		if err != nil {
			return err
		}
		if !resp.Success {
			return fmt.Errorf("%s", resp.Error)
		}

		var sess daemon.SessionDTO
		json.Unmarshal(resp.Data, &sess)

		fmt.Printf("\n  %s  %s\n",
			ui.StyleSuccess.Render("▶"),
			ui.StyleTitle.Render(sess.TaskName),
		)
		fmt.Printf("\n  %s\n\n",
			ui.StyleDimmed.Render("session resumed"),
		)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(resumeCmd)
}
