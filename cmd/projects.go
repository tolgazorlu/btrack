package cmd

import (
	"fmt"
	"sort"
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

		if jsonOutput {
			return printProjectsJSON(store, cfg, projects)
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

var aliasRemove bool

// ─── JSON output ─────────────────────────────────────────────────────────────

// projectJSON is one entry in `btrack projects --json`.
type projectJSON struct {
	Name         string     `json:"name"`
	TotalSeconds int64      `json:"total_seconds"`
	Sessions     int        `json:"sessions"`
	LastActive   *time.Time `json:"last_active,omitempty"`
	Rate         float64    `json:"rate,omitempty"`
}

func printProjectsJSON(store db.Store, cfg *config.Config, projects []string) error {
	out := struct {
		Projects     []projectJSON `json:"projects"`
		TotalSeconds int64         `json:"total_seconds"`
		Sessions     int           `json:"sessions"`
	}{Projects: []projectJSON{}}

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

		pj := projectJSON{Name: proj, TotalSeconds: int64(total.Seconds()), Sessions: len(sessions)}
		if cfg.Projects != nil {
			if pc, ok := cfg.Projects[proj]; ok && pc.Rate > 0 {
				pj.Rate = pc.Rate
			}
		}
		if !lastActive.IsZero() {
			la := lastActive
			pj.LastActive = &la
		}
		out.Projects = append(out.Projects, pj)
	}

	out.TotalSeconds = int64(grandTotal.Seconds())
	out.Sessions = grandSessions
	return printJSON(out)
}

var projectsAliasCmd = &cobra.Command{
	Use:   "alias [name] [project]",
	Short: "File sessions discovered under one name as another project",
	Long: `Imported agent sessions and commits get their project from the repo
they ran in: a .btrack file, else the git remote's repo name, else the
folder name. When that differs from the name you bill under, alias it.

Already imported sessions and commits are renamed immediately, and future
syncs apply the alias. Sessions you logged by hand are never renamed.
Unlike a .btrack file, an alias needs no file in the repository, which
matters for client-owned repos.

Examples:
  btrack projects alias                           # list aliases
  btrack projects alias acme-backend acme         # file acme-backend as acme
  btrack projects alias acme-backend --remove     # stop aliasing`,
	Args: cobra.MaximumNArgs(2),
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

		switch {
		case aliasRemove:
			if len(args) != 1 {
				return fmt.Errorf("usage: btrack projects alias <name> --remove")
			}
			if err := store.RemoveAlias(args[0]); err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(struct {
					Removed string `json:"removed"`
				}{Removed: args[0]})
			}
			ui.OK("removed alias " + args[0] + " (already renamed sessions keep their project)")
			return nil
		case len(args) == 2:
			ns, nc, err := store.SetAlias(args[0], args[1])
			if err != nil {
				return err
			}
			if jsonOutput {
				return printJSON(struct {
					Alias           string `json:"alias"`
					Project         string `json:"project"`
					SessionsRefiled int    `json:"sessions_refiled"`
					CommitsRefiled  int    `json:"commits_refiled"`
				}{Alias: args[0], Project: args[1], SessionsRefiled: ns, CommitsRefiled: nc})
			}
			ui.OK(fmt.Sprintf("%s → %s  ·  %d imported sessions and %d commits refiled", args[0], args[1], ns, nc))
			return nil
		case len(args) == 1:
			return fmt.Errorf("usage: btrack projects alias <name> <project>")
		}

		aliases, err := store.GetAliases()
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(struct {
				Aliases map[string]string `json:"aliases"`
			}{Aliases: aliases})
		}
		ui.Header("project aliases", "")
		if len(aliases) == 0 {
			ui.Hint("none — add one with `btrack projects alias <name> <project>`")
		}
		names := make([]string, 0, len(aliases))
		for a := range aliases {
			names = append(names, a)
		}
		sort.Strings(names)
		for _, a := range names {
			ui.KV(a, "→ "+aliases[a])
		}
		ui.Blank()
		return nil
	},
}

func init() {
	projectsAliasCmd.Flags().BoolVar(&aliasRemove, "remove", false, "Remove the alias")
	projectsCmd.AddCommand(projectsAliasCmd)
	rootCmd.AddCommand(projectsCmd)
}
