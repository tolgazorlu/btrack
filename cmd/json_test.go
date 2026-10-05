package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tolgazorlu/btrack/internal/db"
)

// setupJSONTestDB points BTRACK_DB at a fresh temp sqlite file and returns
// a store opened against it for seeding test data. internal/config caches
// a singleton on first Load(), but config.SQLitePath() always re-reads
// $BTRACK_DB first, so this is safe to call from multiple tests.
func setupJSONTestDB(t *testing.T) db.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "jsontest.db")
	t.Setenv("BTRACK_DB", path)

	store, err := db.NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// withJSONOutput runs fn with the global --json flag set, restoring it
// afterwards, and returns whatever fn wrote to stdout.
func withJSONOutput(t *testing.T, fn func() error) string {
	t.Helper()
	prev := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = prev }()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	runErr := fn()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	if runErr != nil {
		t.Fatalf("command returned error: %v", runErr)
	}
	return buf.String()
}

func mustCreateSession(t *testing.T, store db.Store, s *db.Session) *db.Session {
	t.Helper()
	if err := store.CreateSession(s); err != nil {
		t.Fatalf("create session: %v", err)
	}
	return s
}

func TestHistoryJSONDay(t *testing.T) {
	store := setupJSONTestDB(t)

	start := time.Now().Add(-90 * time.Minute)
	end := time.Now().Add(-30 * time.Minute)
	sess := mustCreateSession(t, store, &db.Session{
		TaskName:  "write json tests",
		StartTime: start,
		EndTime:   &end,
		Project:   "btrack",
	})

	out := withJSONOutput(t, func() error {
		return historyCmd.RunE(historyCmd, nil)
	})

	var got historyJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, out)
	}
	if len(got.Sessions) != 1 {
		t.Fatalf("want 1 session, got %d: %+v", len(got.Sessions), got.Sessions)
	}
	if got.Sessions[0].ID != sess.ID {
		t.Errorf("want session id %d, got %d", sess.ID, got.Sessions[0].ID)
	}
	if got.Sessions[0].Project != "btrack" {
		t.Errorf("want project btrack, got %q", got.Sessions[0].Project)
	}
	wantSeconds := int64(end.Sub(start).Seconds())
	if got.TotalSeconds != wantSeconds {
		t.Errorf("want total_seconds %d, got %d", wantSeconds, got.TotalSeconds)
	}
}

func TestHistoryJSONTable(t *testing.T) {
	store := setupJSONTestDB(t)

	for i := 0; i < 3; i++ {
		start := time.Now().Add(-time.Duration(i+1) * time.Hour)
		end := start.Add(10 * time.Minute)
		mustCreateSession(t, store, &db.Session{
			TaskName:  "task",
			StartTime: start,
			EndTime:   &end,
		})
	}

	if err := historyCmd.Flags().Set("limit", "2"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = historyCmd.Flags().Set("limit", "0") })

	out := withJSONOutput(t, func() error {
		return historyCmd.RunE(historyCmd, nil)
	})

	var got historyJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, out)
	}
	if len(got.Sessions) != 2 {
		t.Fatalf("want 2 sessions (limit), got %d", len(got.Sessions))
	}
}

func TestProjectsJSON(t *testing.T) {
	store := setupJSONTestDB(t)

	start := time.Now().Add(-time.Hour)
	end := time.Now()
	mustCreateSession(t, store, &db.Session{
		TaskName:  "ship feature",
		StartTime: start,
		EndTime:   &end,
		Project:   "acme",
	})

	out := withJSONOutput(t, func() error {
		return projectsCmd.RunE(projectsCmd, nil)
	})

	var got struct {
		Projects     []projectJSON `json:"projects"`
		TotalSeconds int64         `json:"total_seconds"`
		Sessions     int           `json:"sessions"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, out)
	}
	if len(got.Projects) != 1 || got.Projects[0].Name != "acme" {
		t.Fatalf("want one project named acme, got %+v", got.Projects)
	}
	if got.Projects[0].Sessions != 1 {
		t.Errorf("want 1 session for acme, got %d", got.Projects[0].Sessions)
	}
	if got.Sessions != 1 {
		t.Errorf("want 1 session overall, got %d", got.Sessions)
	}
}

func TestStatusJSON(t *testing.T) {
	store := setupJSONTestDB(t)

	active := mustCreateSession(t, store, &db.Session{
		TaskName:  "live session",
		StartTime: time.Now().Add(-5 * time.Minute),
	})

	out := withJSONOutput(t, func() error {
		return statusCmd.RunE(statusCmd, nil)
	})

	var got statusJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, out)
	}
	if got.Active == nil {
		t.Fatalf("want an active session, got nil")
	}
	if got.Active.ID != active.ID {
		t.Errorf("want active id %d, got %d", active.ID, got.Active.ID)
	}
	if !got.Active.Active {
		t.Errorf("want active=true on the session")
	}
	if got.TodaySeconds <= 0 {
		t.Errorf("want today_seconds > 0, got %d", got.TodaySeconds)
	}
}
