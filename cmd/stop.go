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
	"github.com/tolgazorlu/btrack/internal/ui"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the active session and save it",
	Long: `Stop the active session and save it with a closing message.

Examples:
  btrack stop -m "fixed JWT expiry in auth middleware #bugfix"
  btrack stop -m "added 12 unit tests, all passing #test"
  btrack stop          (AI suggests a message based on your notes)

Flags:
  -m, --message   Closing message describing what you did
      --no-ai     Skip AI message suggestion

Tips:
  · Add #tags at the end to categorize your work
  · Common tags: #bugfix #feature #test #docs #refactor #ci
  · View past sessions with: btrack history`,
	RunE: func(cmd *cobra.Command, args []string) error {
		message, _ := cmd.Flags().GetString("message")
		noAI, _ := cmd.Flags().GetBool("no-ai")

		// If no message provided, optionally ask AI.
		if message == "" && !noAI {
			message = suggestMessage()
		}

		if message == "" {
			return fmt.Errorf("a commit message is required (-m / --message), e.g.:\n  btrack stop -m \"fixed the login redirect #bugfix\"")
		}

		// Auto-detect tags and append if not already present.
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
		json.Unmarshal(resp.Data, &sess)

		start, _ := time.Parse(time.RFC3339, sess.StartTime)
		elapsed := time.Since(start)

		fmt.Printf("\n  %s  %s\n",
			ui.StyleSuccess.Render("■"),
			ui.StyleTitle.Render(sess.TaskName),
		)
		fmt.Printf("  %s  %s\n",
			ui.StyleDimmed.Render("duration"),
			ui.FormatDuration(elapsed),
		)
		fmt.Printf("  %s  %s\n\n",
			ui.StyleDimmed.Render("message "),
			ui.StyleHighlight.Render(message),
		)
		return nil
	},
}

func init() {
	stopCmd.Flags().StringP("message", "m", "", "commit message for the session (required)")
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

	// Get current session info for the suggestion.
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

	fmt.Print(ui.StyleDimmed.Render("  ✦ asking AI for a commit message suggestion...\r"))
	suggestion, err := ai.SuggestCommitMessage(context.Background(), provider,
		status.Session.TaskName, notes)
	fmt.Print("                                                     \r")

	if err != nil {
		return ""
	}
	fmt.Printf("\n  %s %s\n  %s ",
		ui.StyleDimmed.Render("AI suggests:"),
		ui.StyleHighlight.Render(suggestion),
		ui.StyleDimmed.Render("use this? [y/N] "),
	)

	var input string
	fmt.Scanln(&input)
	if strings.ToLower(strings.TrimSpace(input)) == "y" {
		return suggestion
	}
	return ""
}
