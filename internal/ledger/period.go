package ledger

import (
	"fmt"
	"time"

	"github.com/tolgazorlu/btrack/internal/db"
)

// PeriodSpec names a reporting period. Since/Until (YYYY-MM-DD, Until
// inclusive) win over LastDays, which wins over Kind ("day", "week",
// "month", "year"; default "week", Monday-based).
type PeriodSpec struct {
	Kind     string
	LastDays int
	Since    string
	Until    string
}

// Resolve returns [from, to) in local time and a short human label.
func (p PeriodSpec) Resolve(now time.Time) (from, to time.Time, label string, err error) {
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	switch {
	case p.Since != "" || p.Until != "":
		to = day.AddDate(0, 0, 1)
		if p.Since != "" {
			if from, err = time.ParseInLocation("2006-01-02", p.Since, time.Local); err != nil {
				return from, to, "", fmt.Errorf("since must be YYYY-MM-DD: %w", err)
			}
		}
		if p.Until != "" {
			if to, err = time.ParseInLocation("2006-01-02", p.Until, time.Local); err != nil {
				return from, to, "", fmt.Errorf("until must be YYYY-MM-DD: %w", err)
			}
			to = to.AddDate(0, 0, 1)
		}
		if !to.After(from) {
			return from, to, "", fmt.Errorf("until is before since")
		}
		return from, to, "", nil
	case p.LastDays > 0:
		return day.AddDate(0, 0, 1-p.LastDays), day.AddDate(0, 0, 1), fmt.Sprintf("last %d days", p.LastDays), nil
	}
	switch p.Kind {
	case "day", "today":
		return day, day.AddDate(0, 0, 1), "today", nil
	case "month":
		m := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
		return m, m.AddDate(0, 1, 0), "this month", nil
	case "year":
		y := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.Local)
		return y, y.AddDate(1, 0, 0), "this year", nil
	case "", "week":
		w := day.AddDate(0, 0, -((int(day.Weekday()) + 6) % 7))
		return w, w.AddDate(0, 0, 7), "this week", nil
	}
	return time.Time{}, time.Time{}, "", fmt.Errorf("unknown period %q (day, week, month, year)", p.Kind)
}

// LoadRecap reads what Build needs from store and builds the recap.
func LoadRecap(store db.Store, from, to time.Time, project string) (Recap, error) {
	// Sessions that started before the period can own segments in it, and
	// cost shares need every segment of a session, so load them all.
	sessions, err := store.GetSessionsInRange(time.Time{}, to)
	if err != nil {
		return Recap{}, err
	}
	segments, err := store.GetSegmentsInRange(time.Time{}, to.AddDate(100, 0, 0))
	if err != nil {
		return Recap{}, err
	}
	commits, err := store.GetCommitsInRange(from, to)
	if err != nil {
		return Recap{}, err
	}
	return Build(sessions, segments, commits, from, to, project), nil
}

// LoadUnlogged reads what Unlogged needs from store.
func LoadUnlogged(store db.Store, from, to time.Time, project string) ([]UnloggedDay, error) {
	sessions, err := store.GetSessionsInRange(time.Time{}, to)
	if err != nil {
		return nil, err
	}
	segments, err := store.GetSegmentsInRange(time.Time{}, to.AddDate(100, 0, 0))
	if err != nil {
		return nil, err
	}
	return Unlogged(sessions, segments, from, to, project), nil
}
