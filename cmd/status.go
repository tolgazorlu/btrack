package cmd

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/tolgaozgun/btrack/internal/daemon"
	"github.com/tolgaozgun/btrack/internal/ui"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the active session with live elapsed time",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := daemon.NewClient()
		model := ui.NewStatusModel(client)

		p := tea.NewProgram(model, tea.WithAltScreen())
		_, err := p.Run()
		return err
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
