package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/ui"
)

// maybeFirstRun triggers the AI setup wizard the very first time btrack
// is used and no AI provider is configured yet.
func maybeFirstRun() {
	cfg, err := config.Load()
	if err != nil {
		return
	}
	// If any key is already configured, nothing to do.
	if cfg.AI.OpenAIKey != "" || cfg.AI.ClaudeKey != "" || cfg.AI.GeminiKey != "" {
		return
	}

	fmt.Println()
	fmt.Println(ui.StyleTitle.Render("  Welcome to btrack!") + "  " + ui.StyleDimmed.Render("first-time setup"))
	fmt.Println()
	fmt.Println(ui.StyleSubtle.Render("  btrack uses AI to summarize your sessions and give insights."))
	fmt.Println(ui.StyleSubtle.Render("  Let's configure an API key — it takes about 30 seconds."))
	fmt.Println()
	fmt.Print(ui.StyleDimmed.Render("  Set up AI now? [Y/n] "))

	var input string
	fmt.Scanln(&input)
	if input == "n" || input == "N" {
		fmt.Printf("\n  %s\n\n",
			ui.StyleDimmed.Render("ok, run `btrack ai setup` any time to configure it later"),
		)
		return
	}

	fmt.Println()
	if err := runSetupWizard(); err != nil {
		fmt.Println(ui.StyleError.Render("  setup error: " + err.Error()))
	}
}

func init() {
	// Hook into root command PersistentPreRun so it fires before any subcommand.
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		// Skip for daemon (internal) and setup itself.
		if cmd.Name() == "start" || cmd.Name() == "stop" ||
			cmd.Name() == "log" || cmd.Name() == "resume" ||
			cmd.Name() == "history" || cmd.Name() == "hist" ||
			cmd.Name() == "status" || cmd.Name() == "version" {
			maybeFirstRun()
		}
	}
}
