package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/daemon"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var switchCmd = &cobra.Command{
	Use:     "switch <new task>",
	Aliases: []string{"sw"},
	Short:   "Stop current session and start a new one",
	Long: `Stop the active session and immediately start a new one.
Saves the context-switch in one command.

Usage:
  btrack switch "new task name"
  btrack sw "new task name"     (short alias)

Examples:
  btrack sw "review PR #43"
  btrack sw "fix urgent prod bug"
  btrack switch "back to feature work"

Flags:
  -m, --message   Closing message for the stopped session (optional)

Tips:
  · If no -m is given, the session is stopped without a closing message
  · Git branch is captured for the new session automatically`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		message, _ := cmd.Flags().GetString("message")
		newTask := strings.Join(args, " ")
		client := daemon.NewClient()

		// Stop current session if active.
		stopPayload := daemon.StopPayload{Message: message}
		stopResp, err := client.Send(daemon.ActionStop, stopPayload)
		if err != nil {
			return err
		}
		if !stopResp.Success {
			return fmt.Errorf("%s", stopResp.Error)
		}
		var stopped daemon.SessionDTO
		json.Unmarshal(stopResp.Data, &stopped)

		ui.Blank()
		ui.Sign(ui.StyleDimmed.Render(ui.Sym.Stop), ui.StyleDimmed.Render(stopped.TaskName))

		// Start new session.
		startPayload := daemon.StartPayload{
			TaskName:  newTask,
			GitBranch: gitBranch(),
			GitRepo:   gitRepo(),
		}
		startResp, err := client.Send(daemon.ActionStart, startPayload)
		if err != nil {
			return err
		}
		if !startResp.Success {
			return fmt.Errorf("%s", startResp.Error)
		}
		var started daemon.SessionDTO
		json.Unmarshal(startResp.Data, &started)

		line := ui.StyleSuccess.Render(ui.Sym.Start) + "  " + ui.StyleHighlight.Render(started.TaskName)
		if started.GitBranch != "" {
			line += "  " + ui.StyleDimmed.Render(ui.Sym.Branch+" "+started.GitBranch)
		}
		fmt.Println(ui.Indent + line)
		ui.Hint("`btrack w` to watch · `btrack x -m \"msg\"` to stop")
		ui.Blank()
		return nil
	},
}

func init() {
	switchCmd.Flags().StringP("message", "m", "", "closing message for the stopped session")
	rootCmd.AddCommand(switchCmd)
}
