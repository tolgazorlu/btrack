package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/daemon"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var pomoCmd = &cobra.Command{
	Use:   "pomo [task]",
	Short: "Run a Pomodoro focus session",
	Long: `Start a full-screen Pomodoro timer that automatically tracks your sessions.

Each focus interval creates a btrack session. When the interval ends it is
automatically stopped with message "pomodoro complete #pomo".

Usage:
  btrack pomo "fix login bug"
  btrack pomo "write tests" --work 45 --break 10

Flags:
  -w, --work         focus interval in minutes (default 25)
  -b, --break        short break in minutes (default 5)
  -l, --long-break   long break after all rounds (default 15)
  -r, --rounds       focus rounds before long break (default 4)

Controls:
  q / esc   Stop current session and quit`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskName := "focus"
		if len(args) > 0 {
			taskName = strings.Join(args, " ")
		}

		workMins, _ := cmd.Flags().GetInt("work")
		breakMins, _ := cmd.Flags().GetInt("break")
		longBreakMins, _ := cmd.Flags().GetInt("long-break")
		rounds, _ := cmd.Flags().GetInt("rounds")

		client := daemon.NewClient()

		// Check no session is already active
		resp, err := client.Send(daemon.ActionStatus, nil)
		if err == nil && resp.Success {
			var status daemon.StatusData
			if jsonErr := json.Unmarshal(resp.Data, &status); jsonErr == nil && status.Active {
				return fmt.Errorf("session %q is already active — stop it first with: btrack x", status.Session.TaskName)
			}
		}

		model := ui.NewPomoModel(taskName, client, workMins, breakMins, longBreakMins, rounds)
		p := tea.NewProgram(model, tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

func init() {
	pomoCmd.Flags().IntP("work", "w", 25, "focus interval in minutes")
	pomoCmd.Flags().IntP("break", "b", 5, "short break in minutes")
	pomoCmd.Flags().IntP("long-break", "l", 15, "long break in minutes")
	pomoCmd.Flags().IntP("rounds", "r", 4, "focus rounds before long break")
	rootCmd.AddCommand(pomoCmd)
}
