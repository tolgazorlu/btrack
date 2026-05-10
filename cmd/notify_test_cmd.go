package cmd

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/notify"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var notifyTestCmd = &cobra.Command{
	Use:   "notify-test",
	Short: "Fire a sample OS notification + sound to verify alerts work",
	Long: `Send a test notification and play the alert sound. Useful for:

  · Granting macOS notification permission to btrack on first run
  · Confirming notify-send is installed on Linux
  · Verifying your terminal isn't muting the bell

If you see a banner and hear a sound, your reminders and pomo alerts
will work. Otherwise check your OS notification settings.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Blank()
		ui.OK("firing test notification + sound")
		ui.Hint("if nothing happens, check your OS notification settings for btrack / Terminal")
		ui.Blank()
		notify.Notify("btrack — test", "If you see this, OS notifications work.")
		notify.Bell()
		// Give the goroutines a moment to actually invoke the platform
		// command before the process exits.
		time.Sleep(1500 * time.Millisecond)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(notifyTestCmd)
}
