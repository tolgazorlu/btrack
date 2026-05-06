package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/daemon"
)

var promptCmd = &cobra.Command{
	Use:   "prompt",
	Short: "Output current session for shell prompt (PS1/Starship)",
	Long: `Output the current btrack session in a format suitable for your shell prompt.

Outputs nothing (silent) when no session is active.

Formats:
  plain      fix login bug · 1h23m          (default)
  starship   JSON for Starship custom module
  json       machine-readable JSON

Shell integration:

  Bash / Zsh — add to ~/.bashrc or ~/.zshrc:
    PS1='$(btrack prompt) $ '

  Starship — add to ~/.config/starship.toml:
    [custom.btrack]
    command = "btrack prompt --format starship"
    when    = "btrack prompt"
    format  = "[$output]($style) "
    style   = "fg:#52e0c4 bold"

  Fish — add to ~/.config/fish/config.fish:
    function fish_prompt
        set bt (btrack prompt)
        if test -n "$bt"
            echo -n "[$bt] "
        end
        echo -n "> "
    end`,
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("format")
		maxLen, _ := cmd.Flags().GetInt("max-len")

		status := daemon.NewClient().QuietStatus()
		if status == nil || !status.Active || status.Session == nil {
			return nil
		}

		startTime, err := time.Parse(time.RFC3339, status.Session.StartTime)
		if err != nil {
			return nil
		}
		elapsed := time.Since(startTime)

		task := status.Session.TaskName
		if len(task) > maxLen {
			task = task[:maxLen-1] + "…"
		}

		elapsedStr := formatPromptDur(elapsed)

		switch format {
		case "starship":
			out := map[string]string{
				"text":  task + " · " + elapsedStr,
				"style": "fg:#52e0c4 bold",
			}
			b, _ := json.Marshal(out)
			fmt.Print(string(b))
		case "json":
			out := map[string]interface{}{
				"task":            status.Session.TaskName,
				"elapsed_seconds": int(elapsed.Seconds()),
				"elapsed":         elapsedStr,
			}
			if status.Session.Project != "" {
				out["project"] = status.Session.Project
			}
			b, _ := json.Marshal(out)
			fmt.Print(string(b))
		default: // plain
			if status.Session.Project != "" {
				fmt.Printf("@%s  %s · %s", status.Session.Project, task, elapsedStr)
			} else {
				fmt.Printf("%s · %s", task, elapsedStr)
			}
		}
		return nil
	},
}

func formatPromptDur(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func init() {
	promptCmd.Flags().StringP("format", "f", "plain", "output format: plain | starship | json")
	promptCmd.Flags().IntP("max-len", "n", 30, "max task name length")
	rootCmd.AddCommand(promptCmd)
}
