package cmd

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/daemon"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the active session with live elapsed time",
	Long: `Show a live view of your current tracking session.

Examples:
  btrack status

Controls:
  q / esc   Quit

What you'll see:
  · Task name with pulsing indicator
  · Elapsed time and progress toward your daily goal
  · Git branch and repo
  · Recent notes`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
		dailyHours := 8
		if cfg != nil && cfg.Work.DailyHours > 0 {
			dailyHours = cfg.Work.DailyHours
		}

		client := daemon.NewClient()
		model := ui.NewStatusModel(client, dailyHours)

		p := tea.NewProgram(model, tea.WithAltScreen())
		_, err := p.Run()
		return err
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
