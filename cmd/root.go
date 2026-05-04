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
	Long: ui.StyleTitle.Render("btrack") + `  ` + ui.StyleDimmed.Render("— time tracker for developers") + `

  ` + ui.StyleHighlight.Render("DAILY WORKFLOW") + `
    btrack start "fix login bug"           start a session
    btrack note "found the JWT issue"      add a checkpoint note
    btrack stop -m "fixed JWT #bugfix"     stop and save
    btrack resume                          continue the last session

  ` + ui.StyleHighlight.Render("REVIEW YOUR WORK") + `
    btrack status                          live view (press q to exit)
    btrack day                             today as a tree with notes
    btrack day yesterday                   yesterday's sessions
    btrack day 2026-05-01                  any specific date
    btrack history                         last 20 sessions in a table
    btrack history -n 50 -v               50 sessions with notes

  ` + ui.StyleHighlight.Render("AI FEATURES") + `
    btrack ai setup                        configure an API key
    btrack ai summarize                    standup summary from today
    btrack ai summarize --days 3           last 3 days
    btrack ai insights                     weekly productivity dashboard
    btrack ai insights --no-ai            stats only, no AI key needed

  ` + ui.StyleHighlight.Render("SETTINGS") + `
    btrack config                          show all current settings
    btrack config hours 6                  set daily target to 6 hours
    btrack version                         print version

  Use ` + ui.StyleDimmed.Render("btrack <command> --help") + ` for details on any command.`,
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
		checkWelcome()
	})
}
