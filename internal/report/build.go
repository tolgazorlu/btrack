package report

import (
	"sort"
	"time"
)

// Build groups commits and stopwatch entries into ascending day buckets.
func Build(commits []Commit, entries []Entry) []Day {
	byDay := map[string]*Day{}

	touch := func(t time.Time) *Day {
		key := t.Format("2006-01-02")
		d, ok := byDay[key]
		if !ok {
			y, m, dd := t.Date()
			d = &Day{Date: time.Date(y, m, dd, 0, 0, 0, 0, t.Location())}
			byDay[key] = d
		}
		return d
	}

	for _, c := range commits {
		d := touch(c.Date)
		d.Commits = append(d.Commits, c)
	}
	for _, e := range entries {
		d := touch(e.Date)
		d.Entries = append(d.Entries, e)
	}

	out := make([]Day, 0, len(byDay))
	for _, d := range byDay {
		sort.SliceStable(d.Commits, func(i, j int) bool {
			return d.Commits[i].Date.Before(d.Commits[j].Date)
		})
		sort.SliceStable(d.Entries, func(i, j int) bool {
			return d.Entries[i].Date.Before(d.Entries[j].Date)
		})
		out = append(out, *d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out
}
