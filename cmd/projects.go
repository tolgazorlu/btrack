package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/db"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "List all projects with time summary",
	Long: `Show a summary of all projects with total tracked time and session counts.

Examples:
  btrack projects

To filter history by project:
  btrack h -n 50 --project myapp

To start a session in a project:
  btrack s "fix auth bug" -p myapp`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		store, err := db.Open(cfg)
		if err != nil {
			return err
		}
		defer store.Close()

		projects, err := store.GetProjects()
		if err != nil {
			return fmt.Errorf("load projects: %w", err)
		}

		if len(projects) == 0 {
			ui.Header("projects", "")
			ui.Hint("no projects yet — start one with `btrack s \"task\" -p name`")
			ui.Blank()
			return nil
		}

		ui.Header("projects", "")

		var grandTotal time.Duration
		grandSessions := 0

		for _, proj := range projects {
			sessions, err := store.GetSessionsByProject(proj, 5000)
			if err != nil {
				continue
			}

			var total time.Duration
			var lastActive time.Time
			for _, s := range sessions {
				total += s.Duration()
				if s.StartTime.After(lastActive) {
					lastActive = s.StartTime
				}
			}
			grandTotal += total
			grandSessions += len(sessions)

			// Rate hint from config
			rateStr := ""
			if cfg.Projects != nil {
				if pc, ok := cfg.Projects[proj]; ok && pc.Rate > 0 {
					rateStr = ui.StyleDimmed.Render(fmt.Sprintf("  $%.0f/h", pc.Rate))
				}
			}

			lastStr := ""
			if !lastActive.IsZero() {
				if sameDay(lastActive, time.Now()) {
					lastStr = "today"
				} else if sameDay(lastActive, time.Now().AddDate(0, 0, -1)) {
					lastStr = "yesterday"
				} else {
					lastStr = lastActive.Local().Format("Jan 02")
				}
			}

			meta := ui.StyleElapsed.Render(formatDur(total)) +
				"  " + ui.StyleDimmed.Render(fmt.Sprintf("%d sessions", len(sessions))) +
				"  " + ui.StyleDimmed.Render("last "+lastStr)
			ui.KV("@"+proj+rateStr, meta)
		}

		ui.Rule()
		fmt.Printf("%s%s  %s  %s\n",
			ui.Indent,
			ui.StyleDimmed.Render("total"),
			ui.StyleElapsed.Render(formatDur(grandTotal)),
			ui.StyleDimmed.Render(fmt.Sprintf("· %d sessions · %d projects", grandSessions, len(projects))),
		)
		ui.Blank()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(projectsCmd)
}
