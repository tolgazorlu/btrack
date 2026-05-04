package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var rootCmd = &cobra.Command{
	Use:   "btrack",
	Short: ui.StyleTitle.Render("btrack") + " — time tracker for developers",
	Long: ui.StyleTitle.Render("btrack") + `

  A minimalist CLI time tracker with AI-powered summaries.

  Workflow:
    btrack start "fix login bug"           start tracking
    btrack note "found the issue"          add a checkpoint note
    btrack stop -m "fixed JWT #bugfix"     stop and save

  Review:
    btrack status                          live view of current session
    btrack day                             today's sessions as a tree
    btrack history                         past sessions in a table

  AI:
    btrack ai summarize                    generate a standup summary
    btrack ai insights                     stats dashboard + AI analysis
    btrack ai setup                        configure API key

  Config:
    btrack config                          show settings
    btrack config hours 8                  set daily work target`,
	SilenceUsage: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, ui.StyleError.Render("error: ")+err.Error())
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(func() {
		if _, err := config.Load(); err != nil {
			fmt.Fprintln(os.Stderr, ui.StyleError.Render("config error: ")+err.Error())
		}
	})
}
