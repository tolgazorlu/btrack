package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/db"
	"github.com/tolgazorlu/btrack/internal/gcal"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var gcalCmd = &cobra.Command{
	Use:   "gcal",
	Short: "Google Calendar integration",
	Long: `Sync your btrack sessions to Google Calendar.

Setup (one time):
  1. Go to https://console.cloud.google.com
  2. Create a project → Enable "Google Calendar API"
  3. Credentials → Create OAuth 2.0 Client ID → Desktop app
  4. Copy the client ID and secret, then run:

  btrack gcal connect --client-id <id> --client-secret <secret>

After connecting:
  btrack gcal status            check connection
  btrack gcal sync              push last 7 days of sessions
  btrack gcal sync --days 30    push last 30 days
  btrack gcal auto-sync on      push automatically after every btrack x`,
}

var gcalConnectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Authorize btrack to access your Google Calendar",
	RunE: func(cmd *cobra.Command, args []string) error {
		clientID, _ := cmd.Flags().GetString("client-id")
		clientSecret, _ := cmd.Flags().GetString("client-secret")
		calendarID, _ := cmd.Flags().GetString("calendar")

		if clientID == "" || clientSecret == "" {
			return fmt.Errorf("--client-id and --client-secret are required\n\n" +
				"  Get them at: https://console.cloud.google.com\n" +
				"  Create an OAuth 2.0 Client ID (Desktop app type)")
		}

		// Save credentials before running OAuth so they persist regardless of token outcome.
		if err := config.SaveGCal(clientID, clientSecret, calendarID, false); err != nil {
			return fmt.Errorf("save credentials: %w", err)
		}

		if err := gcal.Connect(clientID, clientSecret, config.DataDir()); err != nil {
			return err
		}

		fmt.Printf("\n  %s  connected to Google Calendar\n", ui.StyleSuccess.Render("✓"))
		fmt.Printf("  %s  run %s to push sessions automatically after stop\n\n",
			ui.StyleDimmed.Render("tip"),
			ui.StyleHighlight.Render("btrack gcal auto-sync on"),
		)
		return nil
	},
}

var gcalStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check Google Calendar connection status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		connected := gcal.IsConnected(config.DataDir())
		credOK := cfg.GCal.ClientID != ""

		fmt.Println()
		if connected && credOK {
			fmt.Printf("  %s  Google Calendar connected\n", ui.StyleSuccess.Render("●"))
			calID := cfg.GCal.CalendarID
			if calID == "" {
				calID = "primary"
			}
			fmt.Printf("  %s  calendar: %s\n", ui.StyleDimmed.Render(""), ui.StyleHighlight.Render(calID))
			if cfg.GCal.AutoSync {
				fmt.Printf("  %s  auto-sync: on (events created after every stop)\n", ui.StyleDimmed.Render(""))
			} else {
				fmt.Printf("  %s  auto-sync: off  (run: btrack gcal auto-sync on)\n", ui.StyleDimmed.Render(""))
			}
		} else if credOK {
			fmt.Printf("  %s  credentials saved but not authorized yet\n", ui.StyleWarning.Render("○"))
			fmt.Printf("  %s  run: btrack gcal connect\n", ui.StyleDimmed.Render(""))
		} else {
			fmt.Printf("  %s  not connected\n", ui.StyleDimmed.Render("○"))
			fmt.Printf("  %s  run: btrack gcal connect --client-id <id> --client-secret <secret>\n", ui.StyleDimmed.Render(""))
		}
		fmt.Println()
		return nil
	},
}

var gcalSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Push recent sessions to Google Calendar",
	RunE: func(cmd *cobra.Command, args []string) error {
		days, _ := cmd.Flags().GetInt("days")

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if cfg.GCal.ClientID == "" {
			return fmt.Errorf("not configured — run: btrack gcal connect --client-id <id> --client-secret <secret>")
		}

		svc, err := gcal.NewService(cfg.GCal.ClientID, cfg.GCal.ClientSecret, config.DataDir())
		if err != nil {
			return err
		}

		store, err := db.Open(cfg)
		if err != nil {
			return err
		}
		defer store.Close()

		sessions, err := store.GetRecentSessions(2000)
		if err != nil {
			return err
		}

		cutoff := time.Now().AddDate(0, 0, -days)
		pushed, skipped, failed := 0, 0, 0

		fmt.Println()
		for _, s := range sessions {
			if s.EndTime == nil {
				skipped++
				continue // skip active sessions
			}
			if s.StartTime.Before(cutoff) {
				break
			}
			link, err := gcal.PushSession(svc, cfg.GCal.CalendarID, s.TaskName, s.Project, s.StartTime, *s.EndTime)
			if err != nil {
				fmt.Printf("  %s  session %d: %v\n", ui.StyleDimmed.Render("✗"), s.ID, err)
				failed++
				continue
			}
			_ = link
			fmt.Printf("  %s  %s  %s\n",
				ui.StyleSuccess.Render("✓"),
				ui.StyleDimmed.Render(s.StartTime.Local().Format("Mon Jan 02 15:04")),
				ui.StyleHighlight.Render(truncate(s.TaskName, 40)),
			)
			pushed++
		}

		fmt.Printf("\n  pushed %d  ·  skipped %d active  ·  %d errors\n\n",
			pushed, skipped, failed)
		return nil
	},
}

var gcalAutoSyncCmd = &cobra.Command{
	Use:   "auto-sync [on|off]",
	Short: "Toggle automatic push to Google Calendar after every stop",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		var on bool
		switch args[0] {
		case "on", "true", "1":
			on = true
		case "off", "false", "0":
			on = false
		default:
			return fmt.Errorf("expected on or off, got %q", args[0])
		}

		calID := cfg.GCal.CalendarID
		if err := config.SaveGCal(cfg.GCal.ClientID, cfg.GCal.ClientSecret, calID, on); err != nil {
			return err
		}

		state := "off"
		if on {
			state = "on"
		}
		fmt.Printf("\n  %s  auto-sync %s\n\n", ui.StyleSuccess.Render("✓"), ui.StyleHighlight.Render(state))
		return nil
	},
}

var gcalPushCmd = &cobra.Command{
	Use:   "push <session-id>",
	Short: "Push a specific session to Google Calendar",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var id int64
		if _, err := fmt.Sscanf(args[0], "%d", &id); err != nil {
			return fmt.Errorf("invalid session ID %q", args[0])
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if cfg.GCal.ClientID == "" {
			return fmt.Errorf("not configured — run: btrack gcal connect --client-id <id> --client-secret <secret>")
		}

		svc, err := gcal.NewService(cfg.GCal.ClientID, cfg.GCal.ClientSecret, config.DataDir())
		if err != nil {
			return err
		}

		store, err := db.Open(cfg)
		if err != nil {
			return err
		}
		defer store.Close()

		sess, err := store.GetSessionByID(id)
		if err != nil || sess == nil {
			return fmt.Errorf("session %d not found", id)
		}
		if sess.EndTime == nil {
			return fmt.Errorf("session %d is still active — stop it first", id)
		}

		link, err := gcal.PushSession(svc, cfg.GCal.CalendarID, sess.TaskName, sess.Project, sess.StartTime, *sess.EndTime)
		if err != nil {
			return err
		}

		fmt.Printf("\n  %s  pushed to Google Calendar\n", ui.StyleSuccess.Render("✓"))
		fmt.Printf("  %s  %s\n\n", ui.StyleDimmed.Render("event"), ui.StyleHighlight.Render(link))
		return nil
	},
}

func init() {
	gcalConnectCmd.Flags().String("client-id", "", "Google OAuth2 client ID (Desktop app)")
	gcalConnectCmd.Flags().String("client-secret", "", "Google OAuth2 client secret")
	gcalConnectCmd.Flags().String("calendar", "", "calendar ID to use (default: primary)")

	gcalSyncCmd.Flags().IntP("days", "n", 7, "number of past days to sync")

	gcalCmd.AddCommand(gcalConnectCmd, gcalStatusCmd, gcalSyncCmd, gcalAutoSyncCmd, gcalPushCmd)
	rootCmd.AddCommand(gcalCmd)
}
