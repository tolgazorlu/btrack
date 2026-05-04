package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tolgaozgun/btrack/internal/daemon"
	"github.com/tolgaozgun/btrack/internal/ui"
)

var logCmd = &cobra.Command{
	Use:   "log <note>",
	Short: "Add a checkpoint note to the active session",
	Args:  cobra.MinimumNArgs(1),
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
