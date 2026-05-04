package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/db"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var editCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Edit a past session's task name or message",
	Long: `Edit the task name or closing message of a past session.
Find session IDs with: btrack history

Usage:
  btrack edit <id> -t "new task name"
  btrack edit <id> -m "new message"
  btrack edit <id> -t "new task" -m "new message"

Examples:
  btrack edit 42 -t "fix JWT expiry bug"
  btrack edit 42 -m "fixed JWT clock skew in auth middleware #bugfix"
  btrack edit 42 -t "auth work" -m "refactored token handling #refactor"

Flags:
  -t, --task      New task name
  -m, --message   New closing message`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid session ID %q — use a number (find IDs with: btrack history)", args[0])
		}

		newTask, _ := cmd.Flags().GetString("task")
		newMsg, _ := cmd.Flags().GetString("message")

		if newTask == "" && newMsg == "" {
			return fmt.Errorf("provide at least -t (task) or -m (message)")
		}

		cfg, err := config.Load()
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

		// Show what's changing.
		fmt.Println()
		if newTask != "" && newTask != sess.TaskName {
			fmt.Printf("  %s  %s  %s  %s\n",
				ui.StyleDimmed.Render("task"),
				ui.StyleDimmed.Render(truncate(sess.TaskName, 30)),
				ui.StyleDimmed.Render("→"),
				ui.StyleHighlight.Render(newTask),
			)
			sess.TaskName = newTask
		}
		if newMsg != "" && newMsg != sess.Message {
			fmt.Printf("  %s  %s  %s  %s\n",
				ui.StyleDimmed.Render("msg "),
				ui.StyleDimmed.Render(truncate(sess.Message, 30)),
				ui.StyleDimmed.Render("→"),
				ui.StyleHighlight.Render(newMsg),
			)
			// Re-extract tags from new message.
			sess.Message = newMsg
			sess.Tags = extractTagsFromMessage(newMsg)
		}

		if err := store.UpdateSession(sess); err != nil {
			return fmt.Errorf("update session: %w", err)
		}

		fmt.Printf("\n  %s  session %d updated\n\n",
			ui.StyleSuccess.Render("✓"),
			id,
		)
		return nil
	},
}

func extractTagsFromMessage(msg string) []string {
	words := strings.Fields(msg)
	var tags []string
	seen := map[string]bool{}
	for _, w := range words {
		if strings.HasPrefix(w, "#") {
			t := strings.ToLower(w)
			if !seen[t] {
				tags = append(tags, t)
				seen[t] = true
			}
		}
	}
	return tags
}

func init() {
	editCmd.Flags().StringP("task", "t", "", "new task name")
	editCmd.Flags().StringP("message", "m", "", "new closing message")
	rootCmd.AddCommand(editCmd)
}
