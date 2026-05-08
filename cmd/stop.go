package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/ai"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/daemon"
	"github.com/tolgazorlu/btrack/internal/gcal"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var stopCmd = &cobra.Command{
	Use:     "stop",
	Aliases: []string{"x"},
	Short:   "Stop the active session",
	Long: `Stop the active session. The closing message is optional.

Usage:
  btrack stop                stop without a message
  btrack stop -m "message"   stop with a message
  btrack x -m "message"      short alias

Examples:
  btrack x                                  (AI suggests a message; skip = save without one)
  btrack x --no-ai                          (skip AI, save without a message)
  btrack x -m "fixed JWT expiry #bugfix"
  btrack x -m "added 12 unit tests #test"

Flags:
  -m, --message   Closing message (optional)
      --no-ai     Skip AI message suggestion

Tips:
  · Add #tags at the end to categorize your work
  · Common tags: #bugfix #feature #test #docs #refactor #ci
  · btrack shipped to compare what you said vs what landed in git`,
	RunE: func(cmd *cobra.Command, args []string) error {
		message, _ := cmd.Flags().GetString("message")
		noAI, _ := cmd.Flags().GetBool("no-ai")

		if message == "" && !noAI {
			message = suggestMessage()
		}

		if message != "" {
			existingTags := map[string]bool{}
			for _, w := range strings.Fields(message) {
				if strings.HasPrefix(w, "#") {
					existingTags[strings.ToLower(w)] = true
				}
			}
			for _, tag := range ai.CategorizeTask(message) {
				if !existingTags[tag] {
					message += " " + tag
				}
			}
		}

		payload := daemon.StopPayload{Message: message}
		client := daemon.NewClient()
		resp, err := client.Send(daemon.ActionStop, payload)
		if err != nil {
			return err
		}
		if !resp.Success {
			return fmt.Errorf("%s", resp.Error)
		}

		var sess daemon.SessionDTO
		if err := json.Unmarshal(resp.Data, &sess); err != nil {
			return fmt.Errorf("parse daemon response: %w", err)
		}

		start, _ := time.Parse(time.RFC3339, sess.StartTime)
		end := time.Now()
		elapsed := end.Sub(start)

		ui.Blank()
		ui.Sign(ui.StyleSuccess.Render(ui.Sym.Stop), ui.StyleHighlight.Render(sess.TaskName))
		ui.KV("duration", ui.FormatDuration(elapsed))
		if message != "" {
			ui.KV("message", ui.StyleHighlight.Render(message))
		}

		if cfg, err := config.Load(); err == nil && cfg.GCal.AutoSync && cfg.GCal.ClientID != "" {
			fmt.Printf("%s%s  syncing to Google Calendar…\r", ui.Indent, ui.StyleDimmed.Render(ui.Sym.Up))
			svc, err := gcal.NewService(cfg.GCal.ClientID, cfg.GCal.ClientSecret, config.DataDir())
			if err == nil {
				_, err := gcal.PushSession(svc, cfg.GCal.CalendarID, sess.TaskName, sess.Project, start, end)
				if err == nil {
					ui.Sign(ui.StyleSuccess.Render(ui.Sym.Up), ui.StyleDimmed.Render("synced to Google Calendar"))
				}
			}
		}
		ui.Blank()
		return nil
	},
}

func init() {
	stopCmd.Flags().StringP("message", "m", "", "closing message for the session (optional)")
	stopCmd.Flags().Bool("no-ai", false, "skip AI message suggestion")
	rootCmd.AddCommand(stopCmd)
}

func suggestMessage() string {
	cfg, err := config.Load()
	if err != nil || cfg.AI.ActiveKey() == "" {
		return ""
	}

	provider, err := ai.NewProvider(cfg)
	if err != nil {
		return ""
	}

	client := daemon.NewClient()
	resp, err := client.Send(daemon.ActionStatus, nil)
	if err != nil || !resp.Success {
		return ""
	}
	var status daemon.StatusData
	if err := json.Unmarshal(resp.Data, &status); err != nil || !status.Active {
		return ""
	}

	var notes []string
	for _, l := range status.RecentLog {
		notes = append(notes, l.Note)
	}

	fmt.Print(ui.Indent + ui.StyleDimmed.Render("✦ asking AI for a commit message…\r"))
	aiCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	suggestion, err := ai.SuggestCommitMessage(aiCtx, provider,
		status.Session.TaskName, notes)
	fmt.Print("                                                     \r")

	if err != nil {
		return ""
	}
	ui.Blank()
	ui.KV("ai suggests", ui.StyleHighlight.Render(suggestion))
	fmt.Printf("%s%s ", ui.Indent, ui.StyleDimmed.Render("use this? [y/N] "))

	var input string
	fmt.Scanln(&input)
	if strings.ToLower(strings.TrimSpace(input)) == "y" {
		return suggestion
	}
	return ""
}
