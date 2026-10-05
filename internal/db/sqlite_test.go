package db

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func openTemp(t *testing.T) (*SQLiteStore, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "btrack.db")
	s, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

func ptr[T any](v T) *T { return &v }

func TestFreshDatabaseReachesLatestSchema(t *testing.T) {
	s, _ := openTemp(t)
	var v int
	if err := s.db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != SchemaVersion() {
		t.Fatalf("schema version = %d, want %d", v, SchemaVersion())
	}
}

func TestPragmasAreApplied(t *testing.T) {
	s, _ := openTemp(t)
	var mode string
	var fk int
	s.db.QueryRow(`PRAGMA journal_mode`).Scan(&mode)
	s.db.QueryRow(`PRAGMA foreign_keys`).Scan(&fk)
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want wal", mode)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}
}

// A database written by a pre-migration release (no schema_migrations,
// no project/parent_id columns) must upgrade without losing rows.
func TestLegacyDatabaseUpgradesInPlace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = raw.Exec(`
		CREATE TABLE sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT, task_name TEXT NOT NULL,
			start_time DATETIME NOT NULL, end_time DATETIME, message TEXT DEFAULT '',
			tags TEXT DEFAULT '[]', git_branch TEXT DEFAULT '', git_repo TEXT DEFAULT '');
		CREATE TABLE log_entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT, session_id INTEGER NOT NULL,
			note TEXT NOT NULL, timestamp DATETIME NOT NULL);
		INSERT INTO sessions (task_name, start_time, end_time, message)
			VALUES ('old work', '2026-05-01 09:00:00', '2026-05-01 10:30:00', 'done');
	`)
	raw.Close()
	if err != nil {
		t.Fatal(err)
	}

	s, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	defer s.Close()
	got, err := s.GetRecentSessions(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].TaskName != "old work" || got[0].Source != SourceManual {
		t.Fatalf("legacy row not preserved: %+v", got)
	}
	if d := got[0].Duration(); d != 90*time.Minute {
		t.Errorf("duration = %v, want 1h30m", d)
	}

	// Re-opening must be a no-op.
	s.Close()
	s2, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	s2.Close()
}

func TestCreateSessionRoundTripsLedgerFields(t *testing.T) {
	s, _ := openTemp(t)
	start := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	in := &Session{
		TaskName: "fix credit race", StartTime: start, EndTime: &end, Project: "bungalove",
		Tags: []string{"bugfix"}, Source: "claude-code", ExternalID: "abc-123", Cwd: "/repo",
		ActiveSeconds: ptr[int64](1800), AgentSeconds: ptr[int64](3600), Model: "claude-sonnet-5",
		Usage:   TokenUsage{Input: 10, Output: 20, CacheRead: 30, CacheWrite: 40},
		CostUSD: ptr(1.25),
	}
	if err := s.CreateSession(in); err != nil {
		t.Fatal(err)
	}
	out, err := s.GetSessionByID(in.ID)
	if err != nil || out == nil {
		t.Fatalf("get: %v", err)
	}
	if out.Source != "claude-code" || out.ExternalID != "abc-123" || out.Cwd != "/repo" ||
		*out.ActiveSeconds != 1800 || *out.AgentSeconds != 3600 || out.Model != "claude-sonnet-5" ||
		out.Usage != in.Usage || *out.CostUSD != 1.25 || out.Tags[0] != "bugfix" {
		t.Fatalf("round trip mismatch: %+v", out)
	}
}

func TestManualSessionHasUnknownAgentFields(t *testing.T) {
	s, _ := openTemp(t)
	in := &Session{TaskName: "call client", StartTime: time.Now()}
	if err := s.CreateSession(in); err != nil {
		t.Fatal(err)
	}
	out, _ := s.GetActiveSession()
	if out == nil || out.Source != SourceManual || out.ActiveSeconds != nil || out.CostUSD != nil || out.ExternalID != "" {
		t.Fatalf("manual session = %+v", out)
	}
}

func TestUpsertExternalSessionIsIdempotentAndKeepsUserEdits(t *testing.T) {
	s, _ := openTemp(t)
	start := time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	sess := &Session{TaskName: "auto title", StartTime: start, EndTime: &end,
		Source: "codex", ExternalID: "rollout-1", ActiveSeconds: ptr[int64](600)}

	created, err := s.UpsertExternalSession(sess)
	if err != nil || !created {
		t.Fatalf("first upsert: created=%v err=%v", created, err)
	}
	id := sess.ID

	// The user renames it and assigns a project in btrack.
	edited, _ := s.GetSessionByID(id)
	edited.TaskName, edited.Project = "ship CSV export", "kolay-barkod"
	if err := s.UpdateSession(edited); err != nil {
		t.Fatal(err)
	}

	// The session kept running in the tool; a later sync sees it grown.
	later := end.Add(30 * time.Minute)
	again := &Session{TaskName: "auto title", StartTime: start, EndTime: &later,
		Source: "codex", ExternalID: "rollout-1", ActiveSeconds: ptr[int64](900), CostUSD: ptr(0.4)}
	created, err = s.UpsertExternalSession(again)
	if err != nil || created || again.ID != id {
		t.Fatalf("second upsert: created=%v id=%d err=%v", created, again.ID, err)
	}

	got, _ := s.GetSessionByID(id)
	if got.TaskName != "ship CSV export" || got.Project != "kolay-barkod" {
		t.Errorf("user edits overwritten: %+v", got)
	}
	if !got.EndTime.Equal(later) || *got.ActiveSeconds != 900 || *got.CostUSD != 0.4 {
		t.Errorf("metrics not refreshed: %+v", got)
	}
	all, _ := s.GetSessionsInRange(start.Add(-time.Hour), start.Add(time.Hour))
	if len(all) != 1 {
		t.Errorf("rows = %d, want 1", len(all))
	}
}

func TestUpsertRejectsManualOrMissingID(t *testing.T) {
	s, _ := openTemp(t)
	if _, err := s.UpsertExternalSession(&Session{TaskName: "x", StartTime: time.Now(), ExternalID: "1"}); err == nil {
		t.Error("manual source accepted")
	}
	if _, err := s.UpsertExternalSession(&Session{TaskName: "x", StartTime: time.Now(), Source: "codex"}); err == nil {
		t.Error("missing external id accepted")
	}
}

func TestGetSessionsInRange(t *testing.T) {
	s, _ := openTemp(t)
	base := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		st := base.AddDate(0, 0, i)
		if err := s.CreateSession(&Session{TaskName: "d", StartTime: st}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.GetSessionsInRange(base.AddDate(0, 0, 1), base.AddDate(0, 0, 3))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d sessions, want 2", len(got))
	}
}

func TestDeletingSessionCascadesNotes(t *testing.T) {
	s, _ := openTemp(t)
	sess := &Session{TaskName: "n", StartTime: time.Now()}
	s.CreateSession(sess)
	s.CreateLogEntry(&LogEntry{SessionID: sess.ID, Note: "hi", Timestamp: time.Now()})
	if _, err := s.db.Exec(`DELETE FROM sessions WHERE id=?`, sess.ID); err != nil {
		t.Fatal(err)
	}
	logs, _ := s.GetAllLogs(sess.ID)
	if len(logs) != 0 {
		t.Fatalf("notes survived session delete: %d", len(logs))
	}
}

func TestListingMethodsHideImportedSessions(t *testing.T) {
	s, _ := openTemp(t)
	now := time.Now()
	end := now.Add(time.Minute)
	s.CreateSession(&Session{TaskName: "manual", StartTime: now, EndTime: &end, Project: "p"})
	if _, err := s.UpsertExternalSession(&Session{TaskName: "agent", StartTime: now, EndTime: &end,
		Project: "q", Source: "claude-code", ExternalID: "x"}); err != nil {
		t.Fatal(err)
	}
	recent, _ := s.GetRecentSessions(10)
	day, _ := s.GetSessionsForDate(now)
	projects, _ := s.GetProjects()
	last, _ := s.GetLastSession()
	if len(recent) != 1 || len(day) != 1 || len(projects) != 1 || last.TaskName != "manual" {
		t.Fatalf("imported session leaked: recent=%d day=%d projects=%v last=%s", len(recent), len(day), projects, last.TaskName)
	}
	all, _ := s.GetSessionsInRange(now.Add(-time.Hour), now.Add(time.Hour))
	if len(all) != 2 {
		t.Fatalf("range should see both sources, got %d", len(all))
	}
}

func TestCommitsUpsertRangeAndLink(t *testing.T) {
	s, _ := openTemp(t)
	at := time.Date(2026, 10, 3, 14, 0, 0, 0, time.UTC)
	c := &Commit{Project: "app", RepoPath: "/r", SHA: "abc", AuthoredAt: at, Subject: "feat: x", Files: 2, Insertions: 10, Deletions: 1}
	created, err := s.UpsertCommit(c)
	if err != nil || !created || c.ID == 0 {
		t.Fatalf("first upsert: %v %v %d", created, err, c.ID)
	}
	sess := &Session{TaskName: "w", StartTime: at.Add(-time.Hour)}
	s.CreateSession(sess)
	if err := s.LinkCommit(c.ID, &sess.ID); err != nil {
		t.Fatal(err)
	}
	// Same commit seen again via another worktree path: no new row, link kept.
	again := &Commit{Project: "app", RepoPath: "/r-wt", SHA: "abc", AuthoredAt: at, Subject: "feat: x"}
	created, err = s.UpsertCommit(again)
	if err != nil || created || again.ID != c.ID {
		t.Fatalf("second upsert: %v %v %d", created, err, again.ID)
	}
	got, err := s.GetCommitsInRange(at.Add(-time.Minute), at.Add(time.Minute))
	if err != nil || len(got) != 1 || got[0].SessionID == nil || *got[0].SessionID != sess.ID || got[0].Insertions != 10 {
		t.Fatalf("range: %v %+v", err, got)
	}
	// Deleting the session unlinks, not deletes, the commit.
	s.db.Exec(`DELETE FROM sessions WHERE id=?`, sess.ID)
	got, _ = s.GetCommitsInRange(at.Add(-time.Minute), at.Add(time.Minute))
	if len(got) != 1 || got[0].SessionID != nil {
		t.Fatalf("after session delete: %+v", got)
	}
}

func TestSegmentsReplaceAndRange(t *testing.T) {
	s, _ := openTemp(t)
	t0 := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	end := t0.Add(72 * time.Hour)
	sess := &Session{TaskName: "long", StartTime: t0, EndTime: &end, Source: "claude-code", ExternalID: "x"}
	s.UpsertExternalSession(sess)
	segs := []Segment{
		{Start: t0, End: t0.Add(time.Hour), ActiveSeconds: 600, AgentSeconds: 1200},
		{Start: t0.Add(48 * time.Hour), End: t0.Add(49 * time.Hour), ActiveSeconds: 300},
	}
	if err := s.ReplaceSegments(sess.ID, segs); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceSegments(sess.ID, segs); err != nil { // re-sync must not duplicate
		t.Fatal(err)
	}
	got, err := s.GetSegmentsInRange(t0.Add(24*time.Hour), t0.Add(72*time.Hour))
	if err != nil || len(got) != 1 || got[0].ActiveSeconds != 300 || got[0].SessionID != sess.ID {
		t.Fatalf("range: %v %+v", err, got)
	}
	all, _ := s.GetSegmentsInRange(t0.Add(-time.Hour), end)
	if len(all) != 2 {
		t.Fatalf("all = %d, want 2", len(all))
	}
}
