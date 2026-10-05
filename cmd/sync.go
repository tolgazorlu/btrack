package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/db"
	"github.com/tolgazorlu/btrack/internal/ingest"
	"github.com/tolgazorlu/btrack/internal/ledger"
	"github.com/tolgazorlu/btrack/internal/pricing"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var (
	syncDryRun  bool
	syncSources []string
	syncSince   string
	syncJSON    bool
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

type syncSourceResult struct {
	Source   string `json:"source"`
	Files    int    `json:"files"`
	Sessions int    `json:"sessions"`
	Created  int    `json:"created"`
	Updated  int    `json:"updated"`
	Skipped  int    `json:"skipped"`
	Errors   int    `json:"errors"`
}

type syncProjectRow struct {
	Project       string   `json:"project"`
	Sessions      int      `json:"sessions"`
	ActiveSeconds int64    `json:"active_seconds"`
	AgentSeconds  int64    `json:"agent_seconds"`
	CostUSD       float64  `json:"cost_usd"`
	CostComplete  bool     `json:"cost_complete"`
	Sources       []string `json:"sources"`
}

type syncCommitResult struct {
	Repos   int `json:"repos"`
	Scanned int `json:"scanned"`
	New     int `json:"new"`
	Linked  int `json:"linked"`
	Errors  int `json:"errors"`
}

type syncReport struct {
	DryRun        bool               `json:"dry_run"`
	Commits       *syncCommitResult  `json:"commits,omitempty"`
	Sources       []syncSourceResult `json:"sources"`
	Projects      []syncProjectRow   `json:"projects"`
	UnknownModels []string           `json:"unknown_models"`
	PricesChecked string             `json:"prices_checked_at"`
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
	want := map[string]bool{}
	for _, s := range syncSources {
		want[strings.ToLower(strings.TrimSpace(s))] = true
	}

	var store db.Store
	var err error
	if !syncDryRun {
		cfg, cerr := config.Load()
		if cerr != nil {
			return cerr
		}
		store, err = db.Open(cfg)
		if err != nil {
			return err
		}
		defer store.Close()
	}

	home, _ := os.UserHomeDir()
	resolver := &ingest.ProjectResolver{Home: home}
	if store != nil {
		if resolver.Aliases, err = store.GetAliases(); err != nil {
			return err
		}
	}
	rep := syncReport{DryRun: syncDryRun, PricesChecked: pricing.CheckedAt()}
	projects := map[string]*syncProjectRow{}
	unknown := map[string]bool{}

	for _, r := range ingest.Readers() {
		if len(want) > 0 && !want[r.Source()] {
			continue
		}
		res := syncSourceResult{Source: r.Source()}
		paths, err := r.Discover(home)
		if err != nil {
			return fmt.Errorf("%s: discover: %w", r.Source(), err)
		}
		res.Files = len(paths)
		for _, p := range paths {
			sessions, err := r.Read(p)
			if err != nil {
				res.Errors++
				if !syncJSON {
					ui.Warn(fmt.Sprintf("%s: %s: %v", r.Source(), p, err))
				}
				continue
			}
			for _, s := range sessions {
				conv := ingest.ToDB(s, resolver, ingest.SplitOptions{})
				if conv == nil || (!since.IsZero() && conv.Session.StartTime.Before(since)) {
					res.Skipped++
					continue
				}
				res.Sessions++
				if conv.UnknownModel != "" {
					unknown[conv.UnknownModel] = true
				}
				if store != nil {
					created, err := store.UpsertExternalSession(conv.Session)
					if err != nil {
						res.Errors++
						continue
					}
					if created {
						res.Created++
					} else {
						res.Updated++
					}
					if err := store.ReplaceSegments(conv.Session.ID, conv.Segments); err != nil {
						res.Errors++
					}
				}
				addProjectRow(projects, conv.Session)
			}
		}
		rep.Sources = append(rep.Sources, res)
	}

	if store != nil && !syncNoGit {
		cr, err := syncCommits(store, resolver.RepoDirs(), since)
		if err != nil {
			return err
		}
		rep.Commits = cr
	}

	for _, row := range projects {
		sort.Strings(row.Sources)
		rep.Projects = append(rep.Projects, *row)
	}
	sort.Slice(rep.Projects, func(i, j int) bool {
		a, b := rep.Projects[i], rep.Projects[j]
		if a.ActiveSeconds != b.ActiveSeconds {
			return a.ActiveSeconds > b.ActiveSeconds
		}
		return a.Project < b.Project
	})
	for m := range unknown {
		rep.UnknownModels = append(rep.UnknownModels, m)
	}
	sort.Strings(rep.UnknownModels)

	if syncJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	}
	printSyncReport(rep)
	return nil
}

// syncCommits records the user's commits in every repo an imported
// session was attributed through, then links each commit in the touched
// period to the session that produced it.
func syncCommits(store db.Store, repos map[string]string, since time.Time) (*syncCommitResult, error) {
	res := &syncCommitResult{Repos: len(repos)}
	earliest := time.Now()
	dirs := make([]string, 0, len(repos))
	for d := range repos {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		commits, err := ledger.ScanCommits(dir, repos[dir], since)
		if err != nil {
			res.Errors++
			continue
		}
		for _, c := range commits {
			res.Scanned++
			created, err := store.UpsertCommit(c)
			if err != nil {
				return nil, err
			}
			if created {
				res.New++
			}
			if c.AuthoredAt.Before(earliest) {
				earliest = c.AuthoredAt
			}
		}
	}
	if res.Scanned == 0 {
		return res, nil
	}
	// Sessions can start up to a day before a commit they produced.
	from, to := earliest.Add(-24*time.Hour), time.Now().Add(time.Hour)
	// Any session can own a segment in the period, however long ago it
	// started, so load them all; segments narrow it down.
	sessions, err := store.GetSessionsInRange(time.Time{}, to)
	if err != nil {
		return nil, err
	}
	segments, err := store.GetSegmentsInRange(from, to)
	if err != nil {
		return nil, err
	}
	commits, err := store.GetCommitsInRange(earliest.Add(-time.Second), to)
	if err != nil {
		return nil, err
	}
	links := ledger.Link(sessions, segments, commits)
	for _, c := range commits {
		sid, ok := links[c.ID]
		var want *int64
		if ok {
			want = &sid
			res.Linked++
		}
		if (c.SessionID == nil) != (want == nil) || (want != nil && *c.SessionID != *want) {
			if err := store.LinkCommit(c.ID, want); err != nil {
				return nil, err
			}
		}
	}
	return res, nil
}

func addProjectRow(projects map[string]*syncProjectRow, s *db.Session) {
	name := s.Project
	if name == "" {
		name = "(unassigned)"
	}
	row, ok := projects[name]
	if !ok {
		row = &syncProjectRow{Project: name, CostComplete: true}
		projects[name] = row
	}
	row.Sessions++
	if s.ActiveSeconds != nil {
		row.ActiveSeconds += *s.ActiveSeconds
	}
	if s.AgentSeconds != nil {
		row.AgentSeconds += *s.AgentSeconds
	}
	if s.CostUSD != nil {
		row.CostUSD += *s.CostUSD
	} else {
		row.CostComplete = false
	}
	for _, src := range row.Sources {
		if src == s.Source {
			return
		}
	}
	row.Sources = append(row.Sources, s.Source)
}

func printSyncReport(rep syncReport) {
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
	syncCmd.Flags().BoolVar(&syncJSON, "json", false, "Print the result as JSON")
	syncCmd.Flags().BoolVar(&syncNoGit, "no-git", false, "Skip scanning repos for your commits")
	rootCmd.AddCommand(syncCmd)
}
