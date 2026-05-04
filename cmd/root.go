package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/daemon"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var rootCmd = &cobra.Command{
	Use:              "btrack",
	TraverseChildren: true,
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
    btrack week                            full week tree view
    btrack h                               last 20 sessions      (history)
    btrack h -n 50 -v                      50 sessions with notes
    btrack tag #bugfix                     filter by tag

  ` + ui.StyleHighlight.Render("AI FEATURES") + `
    btrack ai setup                        configure an API key
    btrack ai sum                          standup from today     (summarize)
    btrack ai sum --days 3                 last 3 days
    btrack ai ins                          productivity dashboard (insights)
    btrack ai ins --no-ai                 stats only, no key needed

  ` + ui.StyleHighlight.Render("EXPORT & SETTINGS") + `
    btrack export                          export sessions to CSV
    btrack export --format json            export as JSON
    btrack export --days 30 --out f.csv    last 30 days to file
    btrack config                          show all current settings
    btrack config hours 6                  set daily target to 6 hours
    btrack version                         print version

  Use ` + ui.StyleDimmed.Render("btrack <command> --help") + ` for details on any command.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// btrack with no args shows live status instead of help.
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
