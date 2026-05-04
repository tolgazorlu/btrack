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
	Short: "Show or update btrack settings",
	Long: `Show all current btrack settings.

Examples:
  btrack config              (show all settings)
  btrack config hours 6      (set daily work target to 6 hours)
  btrack config hours 8      (set back to 8 hours)

Subcommands:
  btrack config hours <n>    Set daily work hours target (1-24)

Other settings are configured via their own commands:
  btrack ai setup            Set AI provider and key (OpenAI, Claude, Gemini)
  btrack github connect      Link your GitHub account

Config file location is shown at the bottom of: btrack config`,
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
		fmt.Println("  " + kv("hours", fmt.Sprintf("%dh / day", cfg.Work.DailyHours)))
		fmt.Println()

		fmt.Println("  " + dim("ai"))
		provider := cfg.AI.Provider
		if provider == "" {
			provider = "(not set — run: btrack ai setup)"
		}
		fmt.Println("  " + kv("provider", provider))
		model := cfg.AI.Model
		if model == "" {
			model = "(default)"
		}
		fmt.Println("  " + kv("model", model))
		fmt.Println()

		fmt.Println("  " + dim("github"))
		if cfg.GitHub.Username != "" {
			fmt.Println("  " + kv("connected", "@"+cfg.GitHub.Username))
		} else {
			fmt.Println("  " + kv("connected", "(not set — run: btrack github connect)"))
		}
		fmt.Println()

		fmt.Println("  " + dim("database"))
		fmt.Println("  " + kv("type", cfg.Database.Type))
		fmt.Println("  " + sep)
		fmt.Printf("  %s\n\n", ui.StyleDimmed.Render("config file: "+config.ConfigPath()))

		return nil
	},
}

var configHoursCmd = &cobra.Command{
	Use:   "hours <n>",
	Short: "Set your daily work hours target",
	Long: `Set the daily work hours target used in progress bars.

Examples:
  btrack config hours 6
  btrack config hours 8

Valid range: 1–24`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		hours, err := strconv.Atoi(args[0])
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
	configCmd.AddCommand(configHoursCmd)
	rootCmd.AddCommand(configCmd)
}
