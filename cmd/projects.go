package cmd

import (
	"fmt"
	"strings"
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
			fmt.Println(ui.StyleSubtle.Render("\n  no projects yet\n"))
			fmt.Println(ui.StyleDimmed.Render("  start a session with: btrack s \"task\" -p myproject\n"))
			return nil
		}

		sep := ui.StyleDimmed.Render(strings.Repeat("─", 58))
		fmt.Println()
		fmt.Printf("  %s  %s\n", ui.StyleTitle.Render("btrack"), ui.StyleHighlight.Render("projects"))
		fmt.Println("  " + sep)
		fmt.Println()

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

			fmt.Printf("  %s%s\n",
				ui.StyleHighlight.Render(fmt.Sprintf("@%-20s", proj)),
				rateStr,
			)
			fmt.Printf("  %s  %s  %s  %s\n\n",
				ui.StyleDimmed.Render("   "),
				ui.StyleElapsed.Render(formatDur(total)),
				ui.StyleDimmed.Render(fmt.Sprintf("%d sessions", len(sessions))),
				ui.StyleDimmed.Render("last: "+lastStr),
			)
		}

		fmt.Println("  " + sep)
		fmt.Printf("  %s  %s total  ·  %d sessions across %d projects\n\n",
			ui.StyleDimmed.Render("total"),
			ui.StyleElapsed.Render(formatDur(grandTotal)),
			grandSessions,
			len(projects),
		)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(projectsCmd)
}
