package cmd

import (
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
  · Git branch is captured for the new session automatically
  · The stop+start happens atomically inside the daemon`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		message, _ := cmd.Flags().GetString("message")
		newTask := strings.Join(args, " ")
		project, _ := cmd.Flags().GetString("project")

		client := daemon.NewClient()
		data, err := client.Switch(daemon.SwitchPayload{
			TaskName:  newTask,
			Message:   message,
			GitBranch: gitBranch(),
			GitRepo:   gitRepo(),
			Project:   project,
		})
		if err != nil {
			return err
		}

		ui.Blank()
		if data.Stopped != nil {
			ui.Sign(ui.StyleDimmed.Render(ui.Sym.Stop), ui.StyleDimmed.Render(data.Stopped.TaskName))
		}

		started := data.Started
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
	switchCmd.Flags().StringP("project", "p", "", "assign new session to a project")
	rootCmd.AddCommand(switchCmd)
}
