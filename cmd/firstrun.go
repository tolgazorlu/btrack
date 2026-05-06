package cmd

import (
	"fmt"

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

	ui.Header("welcome", "first-time setup")
	ui.Plain(ui.StyleDimmed.Render("btrack uses AI to summarize your sessions and give insights."))
	ui.Plain(ui.StyleDimmed.Render("Configuring a key takes about 30 seconds."))
	ui.Blank()
	fmt.Printf("%s%s ", ui.Indent, ui.StyleDimmed.Render("set up AI now? [Y/n]"))

	var input string
	fmt.Scanln(&input)
	if input == "n" || input == "N" {
		ui.Hint("ok — run `btrack ai setup` any time")
		ui.Blank()
		return
	}

	ui.Blank()
	if err := runSetupWizard(); err != nil {
		ui.FailLine("setup error: " + err.Error())
	}
}

