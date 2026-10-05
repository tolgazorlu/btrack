package ledger

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tolgazorlu/btrack/internal/db"
)

func tempStore(t *testing.T) db.Store {
	t.Helper()
	s, err := db.NewSQLiteStore(filepath.Join(t.TempDir(), "b.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestRecordValidatesAndRefusesOverlap(t *testing.T) {
	store := tempStore(t)
	now := time.Date(2026, 10, 5, 18, 0, 0, 0, time.Local)
	at := func(h, m int) time.Time { return time.Date(2026, 10, 5, h, m, 0, 0, time.Local) }

	s, err := Record(store, RecordInput{Task: " fix export ", Project: "acme", Start: at(9, 0), End: at(10, 0)}, now)
	if err != nil || s.ID == 0 || s.TaskName != "fix export" {
		t.Fatalf("record: %v %+v", err, s)
	}
	// An imported agent session overlapping is fine: it is not the timesheet.
	end := at(11, 0)
	store.UpsertExternalSession(&db.Session{TaskName: "agent", StartTime: at(9, 30), EndTime: &end, Source: "codex", ExternalID: "1"})

	cases := map[string]RecordInput{
		"task is required":   {Start: at(11, 0), End: at(12, 0)},
		"must be after":      {Task: "x", Start: at(12, 0), End: at(12, 0)},
		"longer than":        {Task: "x", Start: at(0, 0).Add(-20 * time.Hour), End: at(1, 0)},
		"in the future":      {Task: "x", Start: at(17, 0), End: at(19, 0)},
		"overlaps session #": {Task: "x", Start: at(9, 30), End: at(9, 45)},
	}
	for want, in := range cases {
		if _, err := Record(store, in, now); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: got %v", want, err)
		}
	}
	if _, err := Record(store, RecordInput{Task: "next", Start: at(10, 0), End: at(10, 30)}, now); err != nil {
		t.Errorf("touching end-to-start is not an overlap: %v", err)
	}
}

func TestUnlogged(t *testing.T) {
	day := func(d, h int) time.Time { return time.Date(2026, 9, d, h, 0, 0, 0, time.Local) }
	e1, e2 := day(16, 12), day(17, 12)
	sessions := []*db.Session{
		{ID: 1, Project: "acme", Source: "claude-code", StartTime: day(16, 9), EndTime: &e2},
		{ID: 2, Project: "acme", Source: db.SourceManual, StartTime: day(16, 9), EndTime: &e1},
	}
	segs := []db.Segment{
		{SessionID: 1, Start: day(16, 9), End: day(16, 10), ActiveSeconds: 3600},
		{SessionID: 1, Start: day(17, 9), End: day(17, 10), ActiveSeconds: 1680},
		{SessionID: 1, Start: day(17, 15), End: day(17, 16), ActiveSeconds: 0, AgentSeconds: 600},
	}
	got := Unlogged(sessions, segs, day(1, 0), day(30, 0), "acme")
	if len(got) != 1 || got[0].Date.Day() != 17 || got[0].YouSeconds != 1680 {
		t.Fatalf("unlogged = %+v", got)
	}
}

func TestPeriodResolve(t *testing.T) {
	now := time.Date(2026, 10, 8, 15, 0, 0, 0, time.Local) // a Thursday
	d := func(m time.Month, day int) time.Time { return time.Date(2026, m, day, 0, 0, 0, 0, time.Local) }
	cases := []struct {
		p        PeriodSpec
		from, to time.Time
	}{
		{PeriodSpec{}, d(10, 5), d(10, 12)},
		{PeriodSpec{Kind: "day"}, d(10, 8), d(10, 9)},
		{PeriodSpec{Kind: "month"}, d(10, 1), d(11, 1)},
		{PeriodSpec{LastDays: 7}, d(10, 2), d(10, 9)},
		{PeriodSpec{Since: "2026-09-01", Until: "2026-09-30", Kind: "day"}, d(9, 1), d(10, 1)},
	}
	for _, c := range cases {
		from, to, _, err := c.p.Resolve(now)
		if err != nil || !from.Equal(c.from) || !to.Equal(c.to) {
			t.Errorf("%+v -> %v %v %v", c.p, from, to, err)
		}
	}
	if _, _, _, err := (PeriodSpec{Kind: "fortnight"}).Resolve(now); err == nil {
		t.Error("unknown kind accepted")
	}
	if _, _, _, err := (PeriodSpec{Since: "2026-10-05", Until: "2026-10-01"}).Resolve(now); err == nil {
		t.Error("reversed range accepted")
	}
}
