package ingest

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tolgazorlu/btrack/internal/db"
	"github.com/tolgazorlu/btrack/internal/ledger"
	"github.com/tolgazorlu/btrack/internal/pricing"
)

type SourceResult struct {
	Source   string `json:"source"`
	Files    int    `json:"files"`
	Sessions int    `json:"sessions"`
	Created  int    `json:"created"`
	Updated  int    `json:"updated"`
	Skipped  int    `json:"skipped"`
	Errors   int    `json:"errors"`
}

type ProjectRow struct {
	Project       string   `json:"project"`
	Sessions      int      `json:"sessions"`
	ActiveSeconds int64    `json:"active_seconds"`
	AgentSeconds  int64    `json:"agent_seconds"`
	CostUSD       float64  `json:"cost_usd"`
	CostComplete  bool     `json:"cost_complete"`
	Sources       []string `json:"sources"`
}

type CommitResult struct {
	Repos   int `json:"repos"`
	Scanned int `json:"scanned"`
	New     int `json:"new"`
	Linked  int `json:"linked"`
	Errors  int `json:"errors"`
}

type Report struct {
	DryRun        bool           `json:"dry_run"`
	Commits       *CommitResult  `json:"commits,omitempty"`
	Sources       []SourceResult `json:"sources"`
	Projects      []ProjectRow   `json:"projects"`
	UnknownModels []string       `json:"unknown_models"`
	PricesChecked string         `json:"prices_checked_at"`
}

// RunOptions configures a sync.
type RunOptions struct {
	Home    string    // home directory the readers search
	Sources []string  // only these source ids; empty means all
	Since   time.Time // skip sessions that started earlier; zero means all
	NoGit   bool      // do not scan repos for commits
	Split   SplitOptions
	Warn    func(string) // per-file read problems; nil to ignore
}

// Run imports every reader's sessions into store, refreshes their
// segments, then records and links the user's commits in the repos they
// ran in. A nil store is a dry run: everything is parsed and summarised,
// nothing is written.
func Run(store db.Store, opt RunOptions) (*Report, error) {
	want := map[string]bool{}
	for _, s := range opt.Sources {
		want[strings.ToLower(strings.TrimSpace(s))] = true
	}
	resolver := &ProjectResolver{Home: opt.Home}
	if store != nil {
		var err error
		if resolver.Aliases, err = store.GetAliases(); err != nil {
			return nil, err
		}
	}
	rep := &Report{DryRun: store == nil, PricesChecked: pricing.CheckedAt()}
	projects := map[string]*ProjectRow{}
	unknown := map[string]bool{}

	for _, r := range Readers() {
		if len(opt.Sources) > 0 && !want[r.Source()] {
			continue
		}
		res := SourceResult{Source: r.Source()}
		paths, err := r.Discover(opt.Home)
		if err != nil {
			return nil, fmt.Errorf("%s: discover: %w", r.Source(), err)
		}
		res.Files = len(paths)
		for _, p := range paths {
			sessions, err := r.Read(p)
			if err != nil {
				res.Errors++
				if opt.Warn != nil {
					opt.Warn(fmt.Sprintf("%s: %s: %v", r.Source(), p, err))
				}
				continue
			}
			for _, s := range sessions {
				conv := ToDB(s, resolver, opt.Split)
				if conv == nil || (!opt.Since.IsZero() && conv.Session.StartTime.Before(opt.Since)) {
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

	if store != nil && !opt.NoGit {
		cr, err := syncCommits(store, resolver.RepoDirs(), opt.Since)
		if err != nil {
			return nil, err
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

	return rep, nil
}

// syncCommits records the user's commits in every repo an imported
// session was attributed through, then links each commit in the touched
// period to the session that produced it.
func syncCommits(store db.Store, repos map[string]string, since time.Time) (*CommitResult, error) {
	res := &CommitResult{Repos: len(repos)}
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

func addProjectRow(projects map[string]*ProjectRow, s *db.Session) {
	name := s.Project
	if name == "" {
		name = "(unassigned)"
	}
	row, ok := projects[name]
	if !ok {
		row = &ProjectRow{Project: name, CostComplete: true}
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
