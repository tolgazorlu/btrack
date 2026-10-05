package ledger

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/tolgazorlu/btrack/internal/db"
)

func ts(h, m int) time.Time { return time.Date(2026, 10, 1, h, m, 0, 0, time.Local) }

func i64(v int64) *int64 { return &v }

func TestParseLog(t *testing.T) {
	out := recSep + "aaa" + fieldSep + "2026-10-01T10:00:00+03:00" + fieldSep + "feat: add sync\n" +
		"10\t2\tcmd/sync.go\n-\t-\tlogo.png\n\n" +
		recSep + "bbb" + fieldSep + "2026-10-01T11:00:00+03:00" + fieldSep + "fix: x\n" +
		recSep + "broken"
	got := parseLog(out, "/r", "app")
	if len(got) != 2 {
		t.Fatalf("got %d commits", len(got))
	}
	if c := got[0]; c.SHA != "aaa" || c.Files != 2 || c.Insertions != 10 || c.Deletions != 2 || c.Subject != "feat: add sync" || c.Project != "app" {
		t.Errorf("first = %+v", c)
	}
	if got[1].Files != 0 {
		t.Errorf("second = %+v", got[1])
	}
}

func TestScanCommitsOnlyUsersOwn(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	git := func(env []string, args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git(nil, "init", "-q")
	git(nil, "config", "user.email", "me@example.com")
	git(nil, "config", "user.name", "Me")
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("1\n2\n"), 0o644)
	git(nil, "add", ".")
	git(nil, "commit", "-qm", "feat: mine")
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("x\n"), 0o644)
	git(nil, "add", ".")
	git([]string{"GIT_AUTHOR_EMAIL=other@example.com", "GIT_AUTHOR_NAME=Other"}, "commit", "-qm", "feat: theirs")

	got, err := ScanCommits(dir, "app", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Subject != "feat: mine" || got[0].Insertions != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestLink(t *testing.T) {
	end1, end2, end3 := ts(11, 0), ts(12, 30), ts(10, 30)
	m := db.SourceManual
	sessions := []*db.Session{
		{ID: 1, Project: "app", Source: m, StartTime: ts(9, 0), EndTime: &end1},
		{ID: 2, Project: "app", Source: m, StartTime: ts(10, 0), EndTime: &end2},
		{ID: 3, Project: "other", Source: m, StartTime: ts(9, 0), EndTime: &end3},
		{ID: 4, Project: "app", Source: m, StartTime: ts(13, 0)}, // still running: ignored
	}
	commits := []*db.Commit{
		{ID: 10, Project: "app", AuthoredAt: ts(9, 30)},   // only session 1
		{ID: 11, Project: "app", AuthoredAt: ts(10, 30)},  // 1 and 2 overlap: 2 started later
		{ID: 12, Project: "app", AuthoredAt: ts(12, 40)},  // within 15m after session 2
		{ID: 13, Project: "app", AuthoredAt: ts(14, 0)},   // nothing
		{ID: 14, Project: "other", AuthoredAt: ts(9, 15)}, // project must match
		{ID: 15, Project: "app", AuthoredAt: ts(11, 5)},   // inside 2 beats widened 1
	}
	got := Link(sessions, nil, commits)
	want := map[int64]int64{10: 1, 11: 2, 12: 2, 14: 3, 15: 2}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for c, s := range want {
		if got[c] != s {
			t.Errorf("commit %d -> %d, want %d", c, got[c], s)
		}
	}
}

func TestBuildTakesLargerOfLoggedAndAgentTimePerDay(t *testing.T) {
	from, to := ts(0, 0), ts(0, 0).AddDate(0, 0, 1)
	e1, e2, e3 := ts(10, 0), ts(12, 0), ts(15, 0)
	cost := 1.5
	sessions := []*db.Session{
		// Logged by hand: 1h.
		{ID: 1, Project: "client", Source: db.SourceManual, TaskName: "call", StartTime: ts(9, 0), EndTime: &e1},
		// Same work through an agent: 90m at the keyboard. Day total is 90m, not 150m.
		{ID: 2, Project: "client", Source: "claude-code", TaskName: "fix export", StartTime: ts(10, 0), EndTime: &e2,
			ActiveSeconds: i64(5400), AgentSeconds: i64(1800), CostUSD: &cost},
		// Own project, agent only, unpriced.
		{ID: 3, Project: "mine", Source: "codex", TaskName: "spike", StartTime: ts(14, 0), EndTime: &e3,
			ActiveSeconds: i64(600), AgentSeconds: i64(1200)},
	}
	sid := int64(2)
	commits := []*db.Commit{
		{ID: 1, Project: "client", Subject: "fix: csv export", AuthoredAt: ts(11, 0), SessionID: &sid, Insertions: 5},
		{ID: 2, Project: "client", Subject: "fix: csv export", AuthoredAt: ts(11, 30), Insertions: 1},
	}
	rc := Build(sessions, nil, commits, from, to, "")
	if len(rc.Projects) != 2 || rc.Projects[0].Project != "client" {
		t.Fatalf("projects = %+v", rc.Projects)
	}
	c := rc.Projects[0]
	if c.Time != 90*time.Minute || c.Logged != time.Hour || c.You != 90*time.Minute || c.Agent != 30*time.Minute {
		t.Errorf("client times: %+v", c)
	}
	if c.Commits != 2 || c.Linked != 1 || c.Insertions != 6 || c.CostUSD != 1.5 || !c.CostComplete {
		t.Errorf("client counts: %+v", c)
	}
	if len(c.Highlights) != 1 || c.Highlights[0] != "fix: csv export" {
		t.Errorf("highlights deduped by subject: %v", c.Highlights)
	}
	m := rc.Projects[1]
	if m.CostComplete || len(m.Highlights) != 1 || m.Highlights[0] != "spike" {
		t.Errorf("mine: %+v", m)
	}
	if rc.Total.Time != 100*time.Minute || rc.Total.Commits != 2 {
		t.Errorf("total: %+v", rc.Total)
	}

	only := Build(sessions, nil, commits, from, to, "mine")
	if len(only.Projects) != 1 || only.Total.Commits != 0 {
		t.Errorf("project filter: %+v", only)
	}
}

// An imported session left open for three days links only commits made
// while one of its segments was active, and its time lands on the days
// the segments happened.
func TestSegmentsDriveLinkingAndDays(t *testing.T) {
	day := func(d, h int) time.Time { return time.Date(2026, 10, d, h, 0, 0, 0, time.Local) }
	end := day(4, 12)
	cost := 3.0
	sessions := []*db.Session{{ID: 7, Project: "app", Source: "claude-code", TaskName: "long",
		StartTime: day(1, 9), EndTime: &end, ActiveSeconds: i64(3600), AgentSeconds: i64(3600), CostUSD: &cost}}
	segs := []db.Segment{
		{SessionID: 7, Start: day(1, 9), End: day(1, 10), ActiveSeconds: 1800, AgentSeconds: 1800},
		{SessionID: 7, Start: day(4, 11), End: day(4, 12), ActiveSeconds: 1800, AgentSeconds: 1800},
	}
	commits := []*db.Commit{
		{ID: 1, Project: "app", AuthoredAt: day(1, 10)},
		{ID: 2, Project: "app", AuthoredAt: day(2, 15)}, // session open, but idle: not linked
		{ID: 3, Project: "app", AuthoredAt: day(4, 11)},
	}
	links := Link(sessions, segs, commits)
	if links[1] != 7 || links[3] != 7 || links[2] != 0 {
		t.Fatalf("links = %v", links)
	}
	rc := Build(sessions, segs, nil, day(4, 0), day(5, 0), "")
	// Half the session's time is on day 4, so half its cost.
	if len(rc.Projects) != 1 || rc.Total.You != 30*time.Minute || rc.Total.CostUSD != 1.5 || rc.Total.Days != 1 {
		t.Fatalf("day 4 recap = %+v", rc.Total)
	}
}

func TestDailySplitsSessionAcrossDaysAndIgnoresManual(t *testing.T) {
	day := func(d, h int) time.Time { return time.Date(2026, 10, d, h, 0, 0, 0, time.Local) }
	end := day(3, 12)
	cost := 4.0
	sessions := []*db.Session{
		{ID: 1, Project: "acme", Source: "claude-code", StartTime: day(1, 9), EndTime: &end, CostUSD: &cost},
		{ID: 2, Project: "acme", Source: db.SourceManual, StartTime: day(1, 9), EndTime: &end},
		{ID: 3, Project: "other", Source: "codex", StartTime: day(1, 9), EndTime: &end},
	}
	segs := []db.Segment{
		{SessionID: 1, Start: day(1, 9), End: day(1, 10), ActiveSeconds: 900, AgentSeconds: 2700},
		{SessionID: 1, Start: day(1, 14), End: day(1, 15), ActiveSeconds: 1800, AgentSeconds: 1800},
		{SessionID: 1, Start: day(3, 11), End: day(3, 12), ActiveSeconds: 3600},
		{SessionID: 3, Start: day(1, 9), End: day(1, 10), ActiveSeconds: 3600},
	}
	got := Daily(sessions, segs, day(1, 0), day(4, 0), "acme")
	if len(got) != 2 {
		t.Fatalf("days = %+v", got)
	}
	d1, d3 := got[0], got[1]
	// Day 1 holds 2h of the session's 3h of work, so 2/3 of its $4.
	near := func(a, b float64) bool { return a-b < 1e-9 && b-a < 1e-9 }
	if d1.You != 45*time.Minute || d1.Agent != 75*time.Minute || d1.Sessions != 1 || !near(d1.CostUSD, 8.0/3) {
		t.Errorf("day 1 = %+v", d1)
	}
	if d3.You != time.Hour || !near(d3.CostUSD, 4.0/3) || !d3.CostComplete {
		t.Errorf("day 3 = %+v", d3)
	}
}
