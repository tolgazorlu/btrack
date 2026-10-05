package ledger

import (
	"sort"
	"strings"
	"time"

	"github.com/tolgazorlu/btrack/internal/db"
)

// ProjectRecap is one project's share of a period.
type ProjectRecap struct {
	Project string `json:"project"`
	// Time is the best estimate of the user's own time: per day, the
	// larger of hand-logged time and keyboard time measured in agent
	// sessions. Taking the larger, not the sum, means work that was both
	// logged by hand and done through an agent is never counted twice.
	Time   time.Duration `json:"-"`
	Logged time.Duration `json:"-"` // hand-kept sessions
	You    time.Duration `json:"-"` // keyboard time inside agent sessions
	Agent  time.Duration `json:"-"` // agent working alone
	Days   int           `json:"days"`

	Sessions     int     `json:"sessions"`
	Commits      int     `json:"commits"`
	Linked       int     `json:"commits_linked"`
	Insertions   int     `json:"insertions"`
	Deletions    int     `json:"deletions"`
	CostUSD      float64 `json:"cost_usd"`
	CostComplete bool    `json:"cost_complete"`

	// Highlights are what was done: commit subjects, newest first; for
	// a project without commits, the titles of its sessions.
	Highlights []string `json:"highlights"`

	TimeSeconds   int64 `json:"time_seconds"`
	LoggedSeconds int64 `json:"logged_seconds"`
	YouSeconds    int64 `json:"you_seconds"`
	AgentSeconds  int64 `json:"agent_seconds"`
}

// Recap summarises [From, To).
type Recap struct {
	From     time.Time      `json:"from"`
	To       time.Time      `json:"to"`
	Projects []ProjectRecap `json:"projects"`
	Total    ProjectRecap   `json:"total"`
}

// MaxHighlights bounds the highlight list per project.
const MaxHighlights = 6

// Build summarises work in [from, to). segments must hold every segment
// of the sessions passed, including those outside the period, because an
// imported session's cost is shared by its segments' share of its time.
//
// Build summarises hand-kept sessions by their start,
// imported sessions by the segments that start in the period (so a
// session open across several days is split across them, cost
// proportionally), and commits by author time. An empty project filter
// keeps everything; unassigned sessions are grouped under "(unassigned)".
func Build(sessions []*db.Session, segments []db.Segment, commits []*db.Commit, from, to time.Time, project string) Recap {
	type dayKey struct {
		project, day string
	}
	logged := map[dayKey]time.Duration{}
	you := map[dayKey]time.Duration{}
	rows := map[string]*ProjectRecap{}
	row := func(p string) *ProjectRecap {
		if p == "" {
			p = "(unassigned)"
		}
		r, ok := rows[p]
		if !ok {
			r = &ProjectRecap{Project: p, CostComplete: true}
			rows[p] = r
		}
		return r
	}
	untitled := map[string][]string{}
	hasCommit := map[int64]bool{}
	for _, c := range commits {
		if c.SessionID != nil {
			hasCommit[*c.SessionID] = true
		}
	}

	segsBySession := map[int64][]db.Segment{}
	for _, g := range segments {
		segsBySession[g.SessionID] = append(segsBySession[g.SessionID], g)
	}
	inRange := func(t time.Time) bool { return !t.Before(from) && t.Before(to) }

	for _, s := range sessions {
		if s.EndTime == nil || (project != "" && s.Project != project) {
			continue
		}
		if s.Source == db.SourceManual {
			if !inRange(s.StartTime) {
				continue
			}
			r := row(s.Project)
			r.Sessions++
			d := s.EndTime.Sub(s.StartTime)
			r.Logged += d
			logged[dayKey{r.Project, s.StartTime.Local().Format("2006-01-02")}] += d
			if !hasCommit[s.ID] {
				untitled[r.Project] = append(untitled[r.Project], s.TaskName)
			}
			continue
		}

		segs, total := sessionSegments(s, segsBySession[s.ID])
		var inPeriod int64
		counted := false
		var r *ProjectRecap
		for _, g := range segs {
			if !inRange(g.Start) {
				continue
			}
			if r == nil {
				r = row(s.Project)
			}
			counted = true
			inPeriod += g.ActiveSeconds + g.AgentSeconds
			a := time.Duration(g.ActiveSeconds) * time.Second
			r.You += a
			you[dayKey{r.Project, g.Start.Local().Format("2006-01-02")}] += a
			r.Agent += time.Duration(g.AgentSeconds) * time.Second
		}
		if !counted {
			continue
		}
		r.Sessions++
		switch {
		case s.CostUSD == nil:
			r.CostComplete = false
		case total == 0:
			r.CostUSD += *s.CostUSD
		default:
			r.CostUSD += *s.CostUSD * float64(inPeriod) / float64(total)
		}
		if !hasCommit[s.ID] && !strings.HasPrefix(s.TaskName, "(untitled") {
			untitled[r.Project] = append(untitled[r.Project], s.TaskName)
		}
	}

	days := map[dayKey]bool{}
	for k := range logged {
		days[k] = true
	}
	for k := range you {
		days[k] = true
	}
	for k := range days {
		t := logged[k]
		if you[k] > t {
			t = you[k]
		}
		rows[k.project].Time += t
		rows[k.project].Days++
	}

	// Newest first for highlights.
	sorted := append([]*db.Commit(nil), commits...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].AuthoredAt.After(sorted[j].AuthoredAt) })
	seen := map[string]map[string]bool{}
	for _, c := range sorted {
		if c.AuthoredAt.Before(from) || !c.AuthoredAt.Before(to) {
			continue
		}
		if project != "" && c.Project != project {
			continue
		}
		r := row(c.Project)
		r.Commits++
		if c.SessionID != nil {
			r.Linked++
		}
		r.Insertions += c.Insertions
		r.Deletions += c.Deletions
		if seen[r.Project] == nil {
			seen[r.Project] = map[string]bool{}
		}
		if len(r.Highlights) < MaxHighlights && !seen[r.Project][c.Subject] {
			seen[r.Project][c.Subject] = true
			r.Highlights = append(r.Highlights, c.Subject)
		}
	}
	for p, titles := range untitled {
		r := rows[p]
		if len(r.Highlights) > 0 {
			continue
		}
		for i := len(titles) - 1; i >= 0 && len(r.Highlights) < MaxHighlights; i-- {
			r.Highlights = append(r.Highlights, titles[i])
		}
	}

	rc := Recap{From: from, To: to, Total: ProjectRecap{Project: "total", CostComplete: true}}
	for _, r := range rows {
		r.fillSeconds()
		rc.Projects = append(rc.Projects, *r)
		t := &rc.Total
		t.Time += r.Time
		t.Logged += r.Logged
		t.You += r.You
		t.Agent += r.Agent
		t.Days += r.Days
		t.Sessions += r.Sessions
		t.Commits += r.Commits
		t.Linked += r.Linked
		t.Insertions += r.Insertions
		t.Deletions += r.Deletions
		t.CostUSD += r.CostUSD
		t.CostComplete = t.CostComplete && r.CostComplete
	}
	rc.Total.fillSeconds()
	sort.Slice(rc.Projects, func(i, j int) bool {
		a, b := rc.Projects[i], rc.Projects[j]
		if a.Time != b.Time {
			return a.Time > b.Time
		}
		return a.Project < b.Project
	})
	return rc
}

func (r *ProjectRecap) fillSeconds() {
	r.TimeSeconds = int64(r.Time.Seconds())
	r.LoggedSeconds = int64(r.Logged.Seconds())
	r.YouSeconds = int64(r.You.Seconds())
	r.AgentSeconds = int64(r.Agent.Seconds())
}
