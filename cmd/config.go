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
  btrack config max-hours 12             cap any session at 12h (0 = off)
  btrack config reminder 60              ping every 60 min while session runs (0 = off)
  btrack config pomo-sound off           silence pomo phase-change sound
  btrack config pomo-notify off          silence pomo phase-change notifications
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

		ui.Header("config", "")

		ui.Section("work")
		ui.KV("hours", ui.StyleHighlight.Render(fmt.Sprintf("%dh / day", cfg.Work.DailyHours)))
		idleVal := "off"
		if cfg.Work.IdleMinutes > 0 {
			idleVal = fmt.Sprintf("%d min", cfg.Work.IdleMinutes)
		}
		ui.KV("idle stop", ui.StyleHighlight.Render(idleVal))
		maxVal := "off"
		if cfg.Work.MaxHours > 0 {
			maxVal = fmt.Sprintf("%d h", cfg.Work.MaxHours)
		}
		ui.KV("max session", ui.StyleHighlight.Render(maxVal))
		reminderVal := "off"
		if cfg.Work.ReminderMinutes > 0 {
			reminderVal = fmt.Sprintf("every %d min", cfg.Work.ReminderMinutes)
		}
		ui.KV("reminder", ui.StyleHighlight.Render(reminderVal))
		ui.Blank()

		ui.Section("pomo")
		ui.KV("sound", ui.StyleHighlight.Render(onOff(cfg.Pomo.Sound)))
		ui.KV("notify", ui.StyleHighlight.Render(onOff(cfg.Pomo.Notify)))
		ui.Blank()

		ui.Section("ai")
		provider := cfg.AI.Provider
		if provider == "" {
			provider = ui.StyleDimmed.Render("(not set — `btrack ai setup`)")
		} else {
			provider = ui.StyleHighlight.Render(provider)
		}
		ui.KV("provider", provider)
		model := cfg.AI.Model
		if model == "" {
			model = ui.StyleDimmed.Render("(default)")
		} else {
			model = ui.StyleHighlight.Render(model)
		}
		ui.KV("model", model)
		ui.Blank()

		ui.Section("github")
		if cfg.GitHub.Username != "" {
			ui.KV("user", ui.StyleHighlight.Render("@"+cfg.GitHub.Username))
		} else {
			ui.KV("user", ui.StyleDimmed.Render("(not set — `btrack github connect`)"))
		}
		ui.Blank()

		if len(cfg.Projects) > 0 {
			ui.Section("projects")
			for name, pc := range cfg.Projects {
				if pc.Rate > 0 {
					ui.KV("@"+name, ui.StyleHighlight.Render(fmt.Sprintf("$%.2f/h", pc.Rate)))
				}
			}
			ui.Blank()
		}

		ui.Section("database")
		ui.KV("type", ui.StyleHighlight.Render(cfg.Database.Type))
		ui.Footer("file: " + config.ConfigPath())
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
		ui.Blank()
		ui.OK("daily target → " + ui.StyleHighlight.Render(fmt.Sprintf("%dh", hours)))
		ui.Blank()
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
		ui.Blank()
		if minutes == 0 {
			ui.OK("idle auto-stop disabled")
		} else {
			ui.OK("idle auto-stop → " + ui.StyleHighlight.Render(fmt.Sprintf("%d min", minutes)))
		}
		ui.Blank()
		return nil
	},
}

func parseOnOff(arg string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(arg)) {
	case "on", "true", "yes", "1", "enable", "enabled":
		return true, nil
	case "off", "false", "no", "0", "disable", "disabled":
		return false, nil
	}
	return false, fmt.Errorf("expected on|off, got %q", arg)
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

var configPomoSoundCmd = &cobra.Command{
	Use:   "pomo-sound <on|off>",
	Short: "Play a sound when a pomodoro phase ends",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		on, err := parseOnOff(args[0])
		if err != nil {
			return err
		}
		if err := config.SavePomoSound(on); err != nil {
			return err
		}
		ui.Blank()
		ui.OK("pomo sound → " + ui.StyleHighlight.Render(onOff(on)))
		ui.Blank()
		return nil
	},
}

var configPomoNotifyCmd = &cobra.Command{
	Use:   "pomo-notify <on|off>",
	Short: "Send an OS notification when a pomodoro phase ends",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		on, err := parseOnOff(args[0])
		if err != nil {
			return err
		}
		if err := config.SavePomoNotify(on); err != nil {
			return err
		}
		ui.Blank()
		ui.OK("pomo notify → " + ui.StyleHighlight.Render(onOff(on)))
		ui.Blank()
		return nil
	},
}

var configReminderCmd = &cobra.Command{
	Use:   "reminder <minutes>",
	Short: "OS notification every N minutes while a session is running (0 = off)",
	Long: `Send an OS notification + sound every N minutes the active session
keeps running. Designed to catch forgotten sessions — if you start a
timer and walk away, the daemon will ping you on a regular cadence.

The reminder counter resets when a session ends (via stop, switch, or
auto-stop), so a fresh session starts with a clean cadence.

Examples:
  btrack config reminder 60    ping every 1h
  btrack config reminder 30    ping every 30 min (more aggressive)
  btrack config reminder 0     disable reminders

Tip: run  btrack notify-test  to verify notifications work on your machine.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		minutes, err := strconv.Atoi(args[0])
		if err != nil || minutes < 0 {
			return fmt.Errorf("minutes must be a non-negative number")
		}
		if err := config.SaveReminderMinutes(minutes); err != nil {
			return err
		}
		ui.Blank()
		if minutes == 0 {
			ui.OK("session reminders disabled")
		} else {
			ui.OK("session reminder → " + ui.StyleHighlight.Render(fmt.Sprintf("every %d min", minutes)))
		}
		ui.Hint("restart the daemon to apply: btrack daemon restart")
		ui.Blank()
		return nil
	},
}

var configMaxHoursCmd = &cobra.Command{
	Use:   "max-hours <hours>",
	Short: "Cap any single session at N hours (0 = disabled)",
	Long: `Hard cap on how long a single session can run before the daemon
auto-stops it. Backdates the end time to start + cap so a forgotten
session doesn't pollute your records with phantom hours.

The auto-stopped session is tagged #runaway and gets the message
"auto-stopped: exceeded max duration (Nh)".

Examples:
  btrack config max-hours 12   cap sessions at 12h (default)
  btrack config max-hours 8    cap at 8h
  btrack config max-hours 0    disable the hard cap`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		hours, err := strconv.Atoi(args[0])
		if err != nil || hours < 0 || hours > 168 {
			return fmt.Errorf("hours must be a non-negative number ≤ 168")
		}
		if err := config.SaveMaxHours(hours); err != nil {
			return err
		}
		ui.Blank()
		if hours == 0 {
			ui.OK("max session cap disabled")
		} else {
			ui.OK("max session cap → " + ui.StyleHighlight.Render(fmt.Sprintf("%dh", hours)))
		}
		ui.Hint("restart the daemon to apply: btrack daemon restart")
		ui.Blank()
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
		ui.Blank()
		ui.OK("@" + projName + " rate → " + ui.StyleHighlight.Render(fmt.Sprintf("$%.2f/h", rate)))
		ui.Blank()
		return nil
	},
}

func init() {
	configCmd.AddCommand(
		configHoursCmd,
		configIdleCmd,
		configMaxHoursCmd,
		configReminderCmd,
		configPomoSoundCmd,
		configPomoNotifyCmd,
		configProjectCmd,
	)
	rootCmd.AddCommand(configCmd)
}
