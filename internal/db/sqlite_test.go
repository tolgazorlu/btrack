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
	all, _ := s.GetRecentSessions(10)
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
