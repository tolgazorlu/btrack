package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tolgaozgun/btrack/internal/daemon"
	"github.com/tolgaozgun/btrack/internal/ui"
)

var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume the last stopped session",
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
