package cmd

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/daemon"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var statusCmd = &cobra.Command{
	Use:     "status",
	Aliases: []string{"w"},
	Short:   "Watch the active session (live elapsed time)",
	Long: `Show a live view of your current tracking session.

Usage:
  btrack status
  btrack w       (short alias — w = watch)

  Also: running btrack with no arguments opens this same view.

Controls:
  q / esc   Quit

What you'll see (session active):
  · Task name with pulsing indicator
  · Elapsed time and progress bar toward your daily target
  · Git branch and repo
  · Recent checkpoint notes

What you'll see (no session running):
  · Idle indicator with today's total time so far

Tips:
  · Set your daily target with: btrack config hours 8
  · Start a session with:       btrack s "your task"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
		dailyHours := 8
		idleMinutes := 0
		if cfg != nil {
			if cfg.Work.DailyHours > 0 {
				dailyHours = cfg.Work.DailyHours
			}
			idleMinutes = cfg.Work.IdleMinutes
		}

		client := daemon.NewClient()
		model := ui.NewStatusModel(client, dailyHours, idleMinutes)

		p := tea.NewProgram(model, tea.WithAltScreen())
		_, err := p.Run()
		return err
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
