package ledger

import (
	"fmt"
	"strings"
	"time"

	"github.com/tolgazorlu/btrack/internal/db"
)

// MaxSessionLength rejects records that are almost certainly a typo.
const MaxSessionLength = 16 * time.Hour

// RecordInput is a finished piece of work to put on the timesheet.
type RecordInput struct {
	Task    string
	Project string
	Message string
	Start   time.Time
	End     time.Time
	Tags    []string
}

// Record validates and stores a finished, hand-kept session. It refuses
// future times, non-positive or implausibly long durations, and any
// overlap with an existing hand-kept session: the timesheet is what
// clients are billed from, so the same minutes must never be in it twice.
func Record(store db.Store, in RecordInput, now time.Time) (*db.Session, error) {
	in.Task = strings.TrimSpace(in.Task)
	switch {
	case in.Task == "":
		return nil, fmt.Errorf("task is required")
	case !in.End.After(in.Start):
		return nil, fmt.Errorf("end (%s) must be after start (%s)", in.End.Format("02.01 15:04"), in.Start.Format("02.01 15:04"))
	case in.End.Sub(in.Start) > MaxSessionLength:
		return nil, fmt.Errorf("session is longer than %s (%s – %s): check the times",
			MaxSessionLength, in.Start.Format("02.01 15:04"), in.End.Format("02.01 15:04"))
	case in.End.After(now.Add(time.Minute)):
		return nil, fmt.Errorf("end (%s) is in the future", in.End.Format("02.01 15:04"))
	}

	existing, err := store.GetSessionsInRange(in.Start.Add(-MaxSessionLength), in.End)
	if err != nil {
		return nil, err
	}
	for _, s := range existing {
		if s.Source != db.SourceManual {
			continue
		}
		sEnd := now
		if s.EndTime != nil {
			sEnd = *s.EndTime
		}
		if s.StartTime.Before(in.End) && in.Start.Before(sEnd) {
			return nil, fmt.Errorf("overlaps session #%d %q (%s – %s); edit or remove it first",
				s.ID, s.TaskName, s.StartTime.Format("02.01 15:04"), sEnd.Format("15:04"))
		}
	}

	sess := &db.Session{
		TaskName: in.Task, Project: in.Project, Message: in.Message, Tags: in.Tags,
		StartTime: in.Start, EndTime: &in.End, Source: db.SourceManual,
	}
	if err := store.CreateSession(sess); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return sess, nil
}

// UnloggedDay is a day where agent logs show keyboard time on a project
// but the timesheet has nothing for it.
type UnloggedDay struct {
	Date time.Time     `json:"date"`
	You  time.Duration `json:"-"`
	// YouSeconds mirrors You for JSON.
	YouSeconds int64 `json:"you_seconds"`
}

// UnloggedThreshold is how much measured keyboard time a day needs before
// a missing hand-kept entry is worth flagging.
const UnloggedThreshold = 15 * time.Minute

// Unlogged lists days in [from, to) with at least UnloggedThreshold of
// keyboard time in the project's agent sessions and no hand-kept session
// of that project: hours that were probably worked and never logged.
func Unlogged(sessions []*db.Session, segments []db.Segment, from, to time.Time, project string) []UnloggedDay {
	logged := map[string]bool{}
	for _, s := range sessions {
		if s.Source == db.SourceManual && s.Project == project && !s.StartTime.Before(from) && s.StartTime.Before(to) {
			logged[s.StartTime.Local().Format("2006-01-02")] = true
		}
	}
	var out []UnloggedDay
	for _, d := range Daily(sessions, segments, from, to, project) {
		if d.You >= UnloggedThreshold && !logged[d.Date.Format("2006-01-02")] {
			out = append(out, UnloggedDay{Date: d.Date, You: d.You, YouSeconds: int64(d.You.Seconds())})
		}
	}
	return out
}
