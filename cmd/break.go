package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/daemon"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var breakCmd = &cobra.Command{
	Use:   "break",
	Short: "Pause the active session for a break",
	Long: `Stop the current session and mark it as a break.
Use "btrack r" to resume when you're back.

Usage:
  btrack break

Examples:
  btrack break          (pause — go grab coffee)
  btrack r              (resume when you're back)

Tips:
  · The session is stopped with message "[break]"
  · "btrack r" picks up the same task name immediately
  · Your work time is preserved accurately — break time is not counted`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := daemon.NewClient()

		// Check there's an active session first.
		statusResp, err := client.Send(daemon.ActionStatus, nil)
		if err != nil || !statusResp.Success {
			return fmt.Errorf("no active session — nothing to pause")
		}
		var status daemon.StatusData
		if err := json.Unmarshal(statusResp.Data, &status); err != nil || !status.Active {
			return fmt.Errorf("no active session — nothing to pause")
		}

		// Stop with a break marker.
		stopPayload := daemon.StopPayload{Message: "[break]"}
		resp, err := client.Send(daemon.ActionStop, stopPayload)
		if err != nil {
			return err
		}
		if !resp.Success {
			return fmt.Errorf("%s", resp.Error)
		}

		var sess daemon.SessionDTO
		json.Unmarshal(resp.Data, &sess)

		start, _ := time.Parse(time.RFC3339, sess.StartTime)
		elapsed := time.Since(start)

		fmt.Printf("\n  %s  %s\n",
			ui.StyleWarning.Render("⏸"),
			ui.StyleTitle.Render(sess.TaskName),
		)
		fmt.Printf("  %s  %s\n\n",
			ui.StyleDimmed.Render("worked   "),
			ui.FormatDuration(elapsed),
		)
		fmt.Printf("  %s\n\n",
			ui.StyleDimmed.Render("on break — run `btrack r` when you're back"),
		)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(breakCmd)
}
