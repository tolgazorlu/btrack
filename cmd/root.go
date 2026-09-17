package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var rootCmd = &cobra.Command{
	Use:              "btrack",
	TraverseChildren: true,
	Short: ui.StyleTitle.Render("btrack") + " — time tracker for developers",
	Long: ui.StyleTitle.Render("btrack") + `  ` + ui.StyleDimmed.Render("— time tracker for developers") + `

  ` + ui.StyleHighlight.Render("DAILY WORKFLOW") + `
    btrack s "fix login bug"               start                 (start)
    btrack n "found the JWT issue"         add a note            (note)
    btrack x -m "fixed JWT #bugfix"        stop and save         (stop)
    btrack sw "review PR #43"              stop + start new      (switch)
    btrack r                               continue last session  (resume)

  ` + ui.StyleHighlight.Render("VIEW YOUR WORK") + `
    btrack w                               live status           (status)
    btrack h                               today as a tree       (history)
    btrack h yesterday / 2026-05-01        specific day
    btrack h -w                            this week
    btrack h -m                            this month
    btrack h -y                            this year
    btrack h -n 20                         last 20 sessions (table)
    btrack h -l 5                          last 5 hours

  ` + ui.StyleHighlight.Render("PROJECTS") + `
    btrack s "task" -p myapp               assign to a project
    btrack projects                        list projects with time

  ` + ui.StyleHighlight.Render("DATA & SETTINGS") + `
    btrack add "task" --from 14:03 --to 15:10   record a finished session (AI-friendly)
    btrack export                          export to CSV
    btrack export --format json --out f    export to JSON file
    btrack report client --from 21.08      client work report (HTML)
    btrack import hours.tsv -p client      backfill tracked hours
    btrack edit <id> -t "new name"         edit a past session
    btrack config hours 6                  set daily target
    btrack config idle 15                  auto-stop after 15 min idle
    btrack config project myapp rate 150   set hourly rate
    btrack config                          show all settings

  ` + ui.StyleHighlight.Render("SHELL AUTOCOMPLETE") + `
    btrack completion zsh >> ~/.zshrc      zsh
    btrack completion bash >> ~/.bashrc    bash
    btrack completion fish > completions   fish

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
	})
}
