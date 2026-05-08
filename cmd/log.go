package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/daemon"
	"github.com/tolgazorlu/btrack/internal/db"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var logCmd = &cobra.Command{
	Use:     "note <text>",
	Aliases: []string{"n", "log"},
	Short:   "Add a checkpoint note to the active session",
	Long: `Add a timestamped note to the currently running session.

Requires an active session — start one with: btrack s "task"

Usage:
  btrack note "text"
  btrack n "text"      (short alias)

Examples:
  btrack n "reproduced the bug on staging"
  btrack n "tried approach A, didn't work"
  btrack n "found root cause: JWT clock skew"
  btrack n "PR ready for review"

Tips:
  · Notes appear as a tree under their session in: btrack h
  · AI uses your notes to write better standup summaries
  · Add as many as you want — they tell the story of your work
  · No active session? Start one first: btrack s "task name"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		note := strings.Join(args, " ")
		sessionID, _ := cmd.Flags().GetInt64("session")
		force, _ := cmd.Flags().GetBool("force")

		if sessionID > 0 {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			store, err := db.Open(cfg)
			if err != nil {
				return err
			}
			defer store.Close()

			sess, err := store.GetSessionByID(sessionID)
			if err != nil || sess == nil {
				return fmt.Errorf("session %d not found", sessionID)
			}
			if sess.EndTime != nil && !force {
				return fmt.Errorf("session #%d is already closed — use --force to add a note anyway", sessionID)
			}

			entry := &db.LogEntry{
				SessionID: sessionID,
				Note:      note,
				Timestamp: time.Now(),
			}
			if err := store.CreateLogEntry(entry); err != nil {
				return fmt.Errorf("add note: %w", err)
			}

			ui.Sign(
				ui.StyleSuccess.Render(ui.Sym.OK),
				ui.StyleDimmed.Render(fmt.Sprintf("#%d", sessionID))+"  "+ui.StyleHighlight.Render(note),
			)
			return nil
		}

		payload := daemon.LogPayload{Note: note}
		client := daemon.NewClient()
		resp, err := client.Send(daemon.ActionLog, payload)
		if err != nil {
			return err
		}
		if !resp.Success {
			return fmt.Errorf("%s", resp.Error)
		}

		ui.OK(ui.StyleHighlight.Render(note))
		return nil
	},
}

func init() {
	logCmd.Flags().Int64P("session", "i", 0, "add note to a past session by ID")
	logCmd.Flags().Bool("force", false, "allow adding a note to a closed session")
	rootCmd.AddCommand(logCmd)
}
