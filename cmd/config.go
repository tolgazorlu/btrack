package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show or set btrack configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		sep := ui.StyleDimmed.Render(strings.Repeat("─", 38))
		kv := func(key, val string) string {
			k := ui.StyleDimmed.Render(fmt.Sprintf("    %-14s", key))
			v := ui.StyleHighlight.Render(val)
			return k + v
		}
		dim := func(s string) string { return ui.StyleDimmed.Render(s) }

		fmt.Println()
		fmt.Printf("  %s\n", ui.StyleTitle.Render("btrack config"))
		fmt.Println("  " + sep)

		fmt.Println("  " + dim("work"))
		fmt.Println("  " + kv("daily_hours", fmt.Sprintf("%dh", cfg.Work.DailyHours)))
		fmt.Println()

		fmt.Println("  " + dim("ai"))
		provider := cfg.AI.Provider
		if provider == "" {
			provider = "(not set)"
		}
		fmt.Println("  " + kv("provider", provider))
		model := cfg.AI.Model
		if model == "" {
			model = "(default)"
		}
		fmt.Println("  " + kv("model", model))
		fmt.Println()

		fmt.Println("  " + dim("database"))
		fmt.Println("  " + kv("type", cfg.Database.Type))
		fmt.Println("  " + sep)
		fmt.Printf("  %s\n\n", ui.StyleDimmed.Render("config file: "+config.ConfigPath()))

		return nil
	},
}

var configDayCmd = &cobra.Command{
	Use:   "day",
	Short: "Configure daily work settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			fmt.Printf("\n  %s  %s\n\n",
				ui.StyleDimmed.Render("daily_hours"),
				ui.StyleHighlight.Render(fmt.Sprintf("%dh", cfg.Work.DailyHours)),
			)
			return nil
		}

		if len(args) != 2 || args[0] != "time" {
			return fmt.Errorf("usage: btrack config day time <hours>")
		}

		hours, err := strconv.Atoi(args[1])
		if err != nil || hours < 1 || hours > 24 {
			return fmt.Errorf("hours must be a number between 1 and 24")
		}

		if err := config.SaveDailyHours(hours); err != nil {
			return err
		}

		fmt.Printf("\n  %s  daily target set to %s\n\n",
			ui.StyleSuccess.Render("✓"),
			ui.StyleHighlight.Render(fmt.Sprintf("%dh", hours)),
		)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configDayCmd)
	rootCmd.AddCommand(configCmd)
}
