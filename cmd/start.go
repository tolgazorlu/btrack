package cmd

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/daemon"
	"github.com/tolgazorlu/btrack/internal/db"
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
  · In the status view: n to add a note, s for sub-note, x to stop`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskName := strings.Join(args, " ")
		project, _ := cmd.Flags().GetString("project")

		payload := daemon.StartPayload{
			TaskName:  taskName,
			GitBranch: gitBranch(),
			GitRepo:   gitRepo(),
			Project:   project,
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

		// Print brief confirmation before opening TUI
		fmt.Printf("\n  %s  %s\n",
			ui.StyleSuccess.Render("▶"),
			ui.StyleTitle.Render(sess.TaskName),
		)
		if sess.Project != "" {
			fmt.Printf("  %s\n", ui.StyleTag.Render("@"+sess.Project))
		}
		if sess.GitBranch != "" {
			fmt.Printf("  %s\n\n", ui.StyleSubtle.Render("⎇  "+sess.GitBranch))
		} else {
			fmt.Println()
		}

		// Open status TUI automatically
		cfg, _ := config.Load()
		dailyHours := 8
		idleMinutes := 0
		if cfg != nil {
			if cfg.Work.DailyHours > 0 {
				dailyHours = cfg.Work.DailyHours
			}
			idleMinutes = cfg.Work.IdleMinutes
		}

		var store db.Store
		if cfg != nil {
			if s, err := db.Open(cfg); err == nil {
				store = s
				defer store.Close()
			}
		}

		model := ui.NewStatusModel(client, dailyHours, idleMinutes, store, Version)
		p := tea.NewProgram(model, tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

func init() {
	startCmd.Flags().StringP("project", "p", "", "assign session to a project")
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
