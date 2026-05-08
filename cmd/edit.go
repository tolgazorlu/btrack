package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/db"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var editCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Edit a past session's task, times, project, or message",
	Long: `Edit any field of a past session.
Find session IDs with: btrack history -n 20

Usage:
  btrack edit <id> -t "new task name"
  btrack edit <id> -m "new message"
  btrack edit <id> --start 09:30 --end 11:45
  btrack edit <id> -p myapp

Examples:
  btrack edit 42 -t "fix JWT expiry bug"
  btrack edit 42 -m "fixed JWT clock skew #bugfix"
  btrack edit 42 --start 09:00 --end 17:30
  btrack edit 42 -p myapp -t "auth refactor"

Flags:
  -t, --task      New task name
  -m, --message   New closing message
  -s, --start     New start time (HH:MM, same day as session)
  -e, --end       New end time (HH:MM, same day as session)
  -p, --project   Assign to project`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid session ID %q — use a number (find IDs with: btrack history)", args[0])
		}

		newTask, _ := cmd.Flags().GetString("task")
		newMsg, _ := cmd.Flags().GetString("message")
		newStart, _ := cmd.Flags().GetString("start")
		newEnd, _ := cmd.Flags().GetString("end")
		newProject, _ := cmd.Flags().GetString("project")

		if newTask == "" && newMsg == "" && newStart == "" && newEnd == "" && newProject == "" {
			return fmt.Errorf("provide at least one flag: -t (task), -m (message), --start, --end, -p (project)")
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

		ui.Header("edit", fmt.Sprintf("#%d", id))

		change := func(key, oldV, newV string) {
			ui.KV(key, ui.StyleDimmed.Render(oldV)+" "+
				ui.StyleDimmed.Render(ui.Sym.Arrow)+" "+
				ui.StyleHighlight.Render(newV))
		}

		if newTask != "" && newTask != sess.TaskName {
			change("task", truncate(sess.TaskName, 28), newTask)
			sess.TaskName = newTask
		}
		if newMsg != "" && newMsg != sess.Message {
			change("message", truncate(sess.Message, 28), newMsg)
			sess.Message = newMsg
			sess.Tags = extractTagsFromMessage(newMsg)
		}
		if newProject != "" && newProject != sess.Project {
			change("project", sess.Project, "@"+newProject)
			sess.Project = newProject
		}
		if newStart != "" {
			t, err := parseTimeOnDay(newStart, sess.StartTime)
			if err != nil {
				return fmt.Errorf("--start: %w", err)
			}
			change("start", sess.StartTime.Local().Format("15:04"), t.Format("15:04"))
			sess.StartTime = t
		}
		if newEnd != "" {
			t, err := parseTimeOnDay(newEnd, sess.StartTime)
			if err != nil {
				return fmt.Errorf("--end: %w", err)
			}
			endStr := "—"
			if sess.EndTime != nil {
				endStr = sess.EndTime.Local().Format("15:04")
			}
			change("end", endStr, t.Format("15:04"))
			sess.EndTime = &t
		}

		if sess.EndTime != nil && !sess.EndTime.After(sess.StartTime) {
			return fmt.Errorf("end time %s must be after start time %s",
				sess.EndTime.Local().Format("15:04"),
				sess.StartTime.Local().Format("15:04"))
		}
		if err := store.UpdateSession(sess); err != nil {
			return fmt.Errorf("update session: %w", err)
		}

		ui.Rule()
		ui.OK(fmt.Sprintf("session #%d updated", id))
		ui.Blank()
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

// parseTimeOnDay parses "HH:MM" and places it on the same calendar day as ref.
func parseTimeOnDay(hhmm string, ref time.Time) (time.Time, error) {
	var h, m int
	if _, err := fmt.Sscanf(hhmm, "%d:%d", &h, &m); err != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return time.Time{}, fmt.Errorf("invalid time %q — use HH:MM (e.g. 09:30)", hhmm)
	}
	y, mo, d := ref.Local().Date()
	return time.Date(y, mo, d, h, m, 0, 0, time.Local), nil
}

func init() {
	editCmd.Flags().StringP("task", "t", "", "new task name")
	editCmd.Flags().StringP("message", "m", "", "new closing message")
	editCmd.Flags().StringP("start", "s", "", "new start time HH:MM (same day)")
	editCmd.Flags().StringP("end", "e", "", "new end time HH:MM (same day)")
	editCmd.Flags().StringP("project", "p", "", "assign to project")
	rootCmd.AddCommand(editCmd)
}
