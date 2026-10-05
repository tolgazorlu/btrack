package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/db"
	"github.com/tolgazorlu/btrack/internal/ingest"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var (
	syncDryRun  bool
	syncSources []string
	syncSince   string
	syncNoGit   bool
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Import AI agent sessions (Claude Code, Codex, Hermes)",
	Long: `Read the session logs your AI coding agents already keep on this
machine and record each session in btrack: when it ran, which project it
belonged to, how much of it was you at the keyboard versus the agent
working alone, which model it used and what it cost.

Safe to run repeatedly: sessions are keyed by their id in the source tool,
so a re-run only refreshes sessions that grew. Titles and projects you
edit in btrack are never overwritten.

It also records your own commits (by your git user.email) in every repo
those sessions worked in, and links each commit to the session that
produced it.

Imported sessions are kept apart from your hand-kept timesheet: btrack h,
export and report keep showing only sessions you recorded yourself.

Time split is a heuristic: a pause over 10 minutes before a prompt counts
as you being away (2 minutes credited for reading and typing), and agent
gaps over 30 minutes count as stalls. Cost is estimated from the token
counts the tool logged and an embedded price table.

Examples:
  btrack sync                     # every source, everything
  btrack sync --dry-run           # show what would be imported
  btrack sync --source codex      # one source
  btrack sync --since 2026-09-01  # only sessions started on or after`,
	RunE: runSync,
}

func runSync(cmd *cobra.Command, args []string) error {
	var since time.Time
	if syncSince != "" {
		t, err := time.ParseInLocation("2006-01-02", syncSince, time.Local)
		if err != nil {
			return fmt.Errorf("--since must be YYYY-MM-DD: %w", err)
		}
		since = t
	}
	var store db.Store
	if !syncDryRun {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if store, err = db.Open(cfg); err != nil {
			return err
		}
		defer store.Close()
	}
	home, _ := os.UserHomeDir()
	opt := ingest.RunOptions{Home: home, Sources: syncSources, Since: since, NoGit: syncNoGit}
	if !jsonOutput {
		opt.Warn = ui.Warn
	}
	rep, err := ingest.Run(store, opt)
	if err != nil {
		return err
	}
	if jsonOutput {
		return printJSON(rep)
	}
	printSyncReport(*rep)
	return nil
}

func printSyncReport(rep ingest.Report) {
	title := "sync"
	if rep.DryRun {
		title = "sync (dry run, nothing written)"
	}
	ui.Header(title, "")
	for _, s := range rep.Sources {
		line := fmt.Sprintf("%-12s %4d sessions", s.Source, s.Sessions)
		if !rep.DryRun {
			line += fmt.Sprintf("  ·  %d new, %d refreshed", s.Created, s.Updated)
		}
		if s.Errors > 0 {
			line += fmt.Sprintf("  ·  %d errors", s.Errors)
		}
		if s.Files == 0 {
			line = fmt.Sprintf("%-12s not found on this machine", s.Source)
		}
		ui.Plain(line)
	}
	if c := rep.Commits; c != nil {
		ui.Plain(fmt.Sprintf("%-12s %4d yours in %d repos  ·  %d new, %d linked to a session",
			"git", c.Scanned, c.Repos, c.New, c.Linked))
	}
	if len(rep.Projects) == 0 {
		ui.Blank()
		return
	}
	ui.Blank()
	ui.Plain(fmt.Sprintf("%-24s %8s %8s %8s %10s", "project", "sessions", "you", "agent", "est. cost"))
	ui.Rule()
	for _, p := range rep.Projects {
		cost := fmt.Sprintf("$%.2f", p.CostUSD)
		if !p.CostComplete {
			cost += "+"
		}
		ui.Plain(fmt.Sprintf("%-24s %8d %8s %8s %10s", truncate(p.Project, 24), p.Sessions,
			shortDur(p.ActiveSeconds), shortDur(p.AgentSeconds), cost))
	}
	ui.Rule()
	ui.Dim("you = time at the keyboard, agent = agent working alone; both heuristic.")
	ui.Dim("est. cost at API list prices (checked " + rep.PricesChecked + "); subscriptions bill differently. '+' = some sessions unpriced.")
	if len(rep.UnknownModels) > 0 {
		ui.Warn("no price for: " + strings.Join(rep.UnknownModels, ", "))
	}
	ui.Blank()
}

func shortDur(sec int64) string {
	d := time.Duration(sec) * time.Second
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dh%02dm", h, m)
}

func init() {
	syncCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "Parse and summarise without writing anything")
	syncCmd.Flags().StringSliceVar(&syncSources, "source", nil, "Only these sources (claude-code, codex, hermes)")
	syncCmd.Flags().StringVar(&syncSince, "since", "", "Only sessions started on or after this date (YYYY-MM-DD)")
	syncCmd.Flags().BoolVar(&syncNoGit, "no-git", false, "Skip scanning repos for your commits")
	rootCmd.AddCommand(syncCmd)
}
