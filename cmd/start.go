package cmd

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/daemon"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var startCmd = &cobra.Command{
	Use:     "start <task>",
	Aliases: []string{"s"},
	Short:   "Start tracking a task",
	Long: `Start a new tracking session for a task.

Usage:
  btrack start "fix login redirect bug"
  btrack s "fix login redirect bug"      (short alias)

Examples:
  btrack s "fix login redirect bug"
  btrack s "write unit tests for auth module"
  btrack s "review PR #42"

Tips:
  · Git branch and repo are captured automatically
  · Only one session can be active at a time
  · Add notes while working: btrack n "found the issue"
  · Finish with:           btrack x -m "what you did"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskName := strings.Join(args, " ")

		payload := daemon.StartPayload{
			TaskName:  taskName,
			GitBranch: gitBranch(),
			GitRepo:   gitRepo(),
		}

		client := daemon.NewClient()
		resp, err := client.Send(daemon.ActionStart, payload)
		if err != nil {
			return err
		}
		if !resp.Success {
			return fmt.Errorf("%s", resp.Error)
		}

		var sess daemon.SessionDTO
		json.Unmarshal(resp.Data, &sess)

		fmt.Printf("\n  %s  %s\n",
			ui.StyleSuccess.Render("▶"),
			ui.StyleTitle.Render(sess.TaskName),
		)
		if sess.GitBranch != "" {
			fmt.Printf("  %s\n", ui.StyleSubtle.Render("⎇  "+sess.GitBranch))
		}
		fmt.Printf("\n  %s\n\n",
			ui.StyleDimmed.Render("run `btrack status` to watch · `btrack stop -m \"msg\"` to finish"),
		)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func gitBranch() string {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func gitRepo() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	return filepath.Base(strings.TrimSpace(string(out)))
}
