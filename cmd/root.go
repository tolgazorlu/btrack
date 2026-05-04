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
    btrack s "fix login bug"               start a session       (start)
    btrack n "found the JWT issue"         add a note            (note)
    btrack x -m "fixed JWT #bugfix"        stop and save         (stop)
    btrack r                               continue last session  (resume)

  ` + ui.StyleHighlight.Render("REVIEW YOUR WORK") + `
    btrack w                               live view             (status)
    btrack d                               today as a tree       (day)
    btrack d yesterday                     yesterday's sessions
    btrack d 2026-05-01                    any specific date
    btrack h                               last 20 sessions      (history)
    btrack h -n 50 -v                      50 sessions with notes

  ` + ui.StyleHighlight.Render("AI FEATURES") + `
    btrack ai setup                        configure an API key
    btrack ai sum                          standup from today     (summarize)
    btrack ai sum --days 3                 last 3 days
    btrack ai ins                          productivity dashboard (insights)
    btrack ai ins --no-ai                 stats only, no key needed

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
