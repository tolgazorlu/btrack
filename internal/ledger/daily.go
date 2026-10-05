package ledger

import (
	"sort"
	"time"

	"github.com/tolgazorlu/btrack/internal/db"
)

// DayWork is one day of imported agent work for a project.
type DayWork struct {
	Date         time.Time // local midnight
	You          time.Duration
	Agent        time.Duration
	CostUSD      float64
	CostComplete bool
	Sessions     int
}

// Daily buckets imported agent sessions of one project by the local day
// each segment started on, sharing a session's cost by its segments'
// share of its time. segments must hold every segment of the sessions
// passed (see Build). Hand-kept sessions are ignored: they are the
// timesheet this view is compared against.
func Daily(sessions []*db.Session, segments []db.Segment, from, to time.Time, project string) []DayWork {
	segsBySession := map[int64][]db.Segment{}
	for _, g := range segments {
		segsBySession[g.SessionID] = append(segsBySession[g.SessionID], g)
	}
	days := map[string]*DayWork{}
	seen := map[string]map[int64]bool{}
	for _, s := range sessions {
		if s.Source == db.SourceManual || s.EndTime == nil || s.Project != project {
			continue
		}
		segs, total := sessionSegments(s, segsBySession[s.ID])
		for _, g := range segs {
			if g.Start.Before(from) || !g.Start.Before(to) {
				continue
			}
			l := g.Start.Local()
			key := l.Format("2006-01-02")
			d, ok := days[key]
			if !ok {
				d = &DayWork{Date: time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, time.Local), CostComplete: true}
				days[key] = d
				seen[key] = map[int64]bool{}
			}
			d.You += time.Duration(g.ActiveSeconds) * time.Second
			d.Agent += time.Duration(g.AgentSeconds) * time.Second
			if !seen[key][s.ID] {
				seen[key][s.ID] = true
				d.Sessions++
			}
			switch {
			case s.CostUSD == nil:
				d.CostComplete = false
			case total == 0:
				d.CostUSD += *s.CostUSD / float64(len(segs))
			default:
				d.CostUSD += *s.CostUSD * float64(g.ActiveSeconds+g.AgentSeconds) / float64(total)
			}
		}
	}
	out := make([]DayWork, 0, len(days))
	for _, d := range days {
		out = append(out, *d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out
}

// sessionSegments returns an imported session's segments, or the whole
// session as one segment when it was imported before segments existed,
// plus its total active+agent seconds.
func sessionSegments(s *db.Session, segs []db.Segment) ([]db.Segment, int64) {
	if len(segs) == 0 {
		var a, g int64
		if s.ActiveSeconds != nil {
			a = *s.ActiveSeconds
		}
		if s.AgentSeconds != nil {
			g = *s.AgentSeconds
		}
		segs = []db.Segment{{SessionID: s.ID, Start: s.StartTime, End: *s.EndTime, ActiveSeconds: a, AgentSeconds: g}}
	}
	var total int64
	for _, g := range segs {
		total += g.ActiveSeconds + g.AgentSeconds
	}
	return segs, total
}
