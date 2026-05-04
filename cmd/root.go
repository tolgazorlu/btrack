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
	Short: ui.StyleTitle.Render("btrack") + " — git-style time tracker for developers",
	Long: ui.StyleTitle.Render("btrack") + `

  A minimalist, AI-native CLI time tracker with git-style workflow.

  Examples:
    btrack start "fix login bug"
    btrack log "isolated the JWT expiry issue"
    btrack stop -m "fixed JWT expiry in auth middleware #bugfix"
    btrack status
    btrack ai summarize`,
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
