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
	Long: `Start a new session copying the task name from your last stopped session.

Requires at least one previous session — if none exists you will get an error.

Usage:
  btrack resume
  btrack r        (short alias)

Examples:
  btrack r   (picks up where you left off)

Common workflow:
  btrack break      (pause — go grab coffee)
  btrack r          (resume when you are back — same task name)

Tips:
  · Creates a brand new session — does not modify the old one
  · Copies task name and git branch/repo from the last session
  · If a session is already running, stop it first: btrack x -m "msg"`,
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
		if err := json.Unmarshal(resp.Data, &sess); err != nil {
			return fmt.Errorf("parse daemon response: %w", err)
		}

		ui.Blank()
		ui.Sign(ui.StyleSuccess.Render(ui.Sym.Resume), ui.StyleHighlight.Render(sess.TaskName))
		ui.Hint("session resumed")
		ui.Blank()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(resumeCmd)
}
