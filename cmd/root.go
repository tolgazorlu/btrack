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
    btrack break                           pause for a break

  ` + ui.StyleHighlight.Render("VIEW YOUR WORK") + `
    btrack w                               live status           (status)
    btrack h                               today as a tree       (history)
    btrack h yesterday / 2026-05-01        specific day
    btrack h -w                            this week
    btrack h -m                            this month
    btrack h -y                            this year
    btrack h -n 20                         last 20 sessions (table)
    btrack h -l 5                          last 5 hours
    btrack stats                           quick snapshot
    btrack streak                          working day streak
    btrack tag #bugfix                     filter by tag
    btrack search "JWT"                    search sessions       (find, f)

  ` + ui.StyleHighlight.Render("AI") + `
    btrack ai                              interactive chat
    btrack ai sum                          standup from today
    btrack ai sum --days 3                 last 3 days
    btrack ai ins                          productivity dashboard
    btrack ai ins --no-ai                  stats only, no key needed
    btrack ai setup                        configure API key

  ` + ui.StyleHighlight.Render("PROJECTS & BILLING") + `
    btrack s "task" -p myapp               assign to a project
    btrack projects                        list projects with time
    btrack invoice -p myapp -r 150         generate invoice
    btrack invoice --month 2026-05 -r 100  specific month

  ` + ui.StyleHighlight.Render("FOCUS") + `
    btrack pomo "write tests"              25/5 pomodoro timer
    btrack pomo "task" --work 45           custom interval

  ` + ui.StyleHighlight.Render("DATA & SETTINGS") + `
    btrack export                          export to CSV
    btrack export --format json --out f    export to JSON file
    btrack edit <id> -t "new name"         edit a past session
    btrack config hours 6                  set daily target
    btrack config idle 15                  auto-stop after 15 min idle
    btrack config project myapp rate 150   set hourly rate
    btrack config                          show all settings

  ` + ui.StyleHighlight.Render("SHELL PROMPT") + `
    btrack prompt                          current session for PS1
    btrack prompt --format starship        Starship JSON module

  ` + ui.StyleHighlight.Render("LINKS") + `
    btrack repo                            project links
    btrack repo star / issue / releases    open in browser

  ` + ui.StyleHighlight.Render("SHELL AUTOCOMPLETE") + `
    btrack completion zsh >> ~/.zshrc      zsh
    btrack completion bash >> ~/.bashrc    bash
    btrack completion fish > completions   fish

  Use ` + ui.StyleDimmed.Render("btrack <command> --help") + ` for details on any command.`,
	SilenceUsage: true,
	// RunE is wired in init() to break the rootCmd ↔ runConsole reference cycle.
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, ui.StyleError.Render("error: ")+err.Error())
		os.Exit(1)
	}
}

func init() {
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		// btrack with no args opens the interactive console (Claude Code / Gemini style).
		return runConsole()
	}
	cobra.OnInitialize(func() {
		if _, err := config.Load(); err != nil {
			fmt.Fprintln(os.Stderr, ui.StyleError.Render("config error: ")+err.Error())
		}
	})
	// Welcome after the command is known — skip TUI / machine-parseable commands (see cmd/welcome.go).
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if !welcomeSuppressed(cmd) {
			checkWelcome()
		}
	}
}
