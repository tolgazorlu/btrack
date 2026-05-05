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
	Long: `Show all current btrack settings or update them.

Examples:
  btrack config                          show all settings
  btrack config hours 6                  set daily work target
  btrack config idle 15                  auto-stop after 15 min idle (0 = off)
  btrack config project myapp rate 150   set hourly rate for a project

Other settings:
  btrack ai setup            configure AI provider key
  btrack github connect      link GitHub account

Config file: ~/.config/btrack/config.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		sep := ui.StyleDimmed.Render(strings.Repeat("─", 42))
		kv := func(key, val string) string {
			k := ui.StyleDimmed.Render(fmt.Sprintf("    %-16s", key))
			v := ui.StyleHighlight.Render(val)
			return k + v
		}
		dim := func(s string) string { return ui.StyleDimmed.Render(s) }

		fmt.Println()
		fmt.Printf("  %s\n", ui.StyleTitle.Render("btrack config"))
		fmt.Println("  " + sep)

		fmt.Println("  " + dim("work"))
		fmt.Println("  " + kv("hours", fmt.Sprintf("%dh / day", cfg.Work.DailyHours)))
		idleVal := "off"
		if cfg.Work.IdleMinutes > 0 {
			idleVal = fmt.Sprintf("%d min", cfg.Work.IdleMinutes)
		}
		fmt.Println("  " + kv("idle auto-stop", idleVal))
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

		if len(cfg.Projects) > 0 {
			fmt.Println("  " + dim("projects"))
			for name, pc := range cfg.Projects {
				if pc.Rate > 0 {
					fmt.Println("  " + kv("@"+name, fmt.Sprintf("$%.2f/h", pc.Rate)))
				}
			}
			fmt.Println()
		}

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
	Args:  cobra.ExactArgs(1),
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

var configIdleCmd = &cobra.Command{
	Use:   "idle <minutes>",
	Short: "Set idle auto-stop threshold in minutes (0 = disabled)",
	Long: `Auto-stop the active session after N minutes of inactivity.

The daemon tracks the last time any btrack command was run. If no command
is issued within the threshold, the session is automatically stopped with
message "auto-stopped: idle" and tagged #idle.

Examples:
  btrack config idle 15    auto-stop after 15 minutes
  btrack config idle 0     disable idle detection`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		minutes, err := strconv.Atoi(args[0])
		if err != nil || minutes < 0 {
			return fmt.Errorf("minutes must be a non-negative number")
		}
		if err := config.SaveIdleMinutes(minutes); err != nil {
			return err
		}
		if minutes == 0 {
			fmt.Printf("\n  %s  idle auto-stop disabled\n\n", ui.StyleSuccess.Render("✓"))
		} else {
			fmt.Printf("\n  %s  idle auto-stop set to %s\n\n",
				ui.StyleSuccess.Render("✓"),
				ui.StyleHighlight.Render(fmt.Sprintf("%d min", minutes)),
			)
		}
		return nil
	},
}

var configProjectCmd = &cobra.Command{
	Use:   "project <name> rate <amount>",
	Short: "Set the hourly billing rate for a project",
	Long: `Set an hourly billing rate for a project used by btrack invoice.

Examples:
  btrack config project myapp rate 150
  btrack config project client-x rate 95.50`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		projName := args[0]
		if args[1] != "rate" {
			return fmt.Errorf("usage: btrack config project <name> rate <amount>")
		}
		rate, err := strconv.ParseFloat(args[2], 64)
		if err != nil || rate < 0 {
			return fmt.Errorf("rate must be a positive number")
		}
		if err := config.SaveProjectRate(projName, rate); err != nil {
			return err
		}
		fmt.Printf("\n  %s  @%s rate set to %s\n\n",
			ui.StyleSuccess.Render("✓"),
			projName,
			ui.StyleHighlight.Render(fmt.Sprintf("$%.2f/h", rate)),
		)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configHoursCmd, configIdleCmd, configProjectCmd)
	rootCmd.AddCommand(configCmd)
}
