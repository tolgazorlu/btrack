package ingest

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// buildHermesFixture creates a minimal state.db at path with the handful of
// columns HermesReader actually reads, modeled on the real Hermes schema.
func buildHermesFixture(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer db.Close()

	schema := `
	CREATE TABLE sessions (
		id TEXT PRIMARY KEY,
		source TEXT NOT NULL,
		parent_session_id TEXT,
		end_reason TEXT,
		started_at REAL NOT NULL,
		cwd TEXT,
		git_branch TEXT,
		git_repo_root TEXT,
		model TEXT,
		title TEXT,
		input_tokens INTEGER DEFAULT 0,
		output_tokens INTEGER DEFAULT 0,
		cache_read_tokens INTEGER DEFAULT 0,
		cache_write_tokens INTEGER DEFAULT 0,
		estimated_cost_usd REAL,
		actual_cost_usd REAL
	);
	CREATE TABLE messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT,
		tool_calls TEXT,
		timestamp REAL NOT NULL
	);
	CREATE TABLE session_model_usage (
		session_id TEXT NOT NULL,
		model TEXT NOT NULL,
		input_tokens INTEGER NOT NULL DEFAULT 0,
		output_tokens INTEGER NOT NULL DEFAULT 0,
		cache_read_tokens INTEGER NOT NULL DEFAULT 0,
		cache_write_tokens INTEGER NOT NULL DEFAULT 0
	);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("exec %q: %v", q, err)
		}
	}

	// s1: a normal interactive session with a real session_model_usage row,
	// a legit human prompt, an injected OUT-OF-BAND message, and an async
	// delegation notice that must both be excluded from Human events.
	exec(`INSERT INTO sessions (id, source, parent_session_id, started_at, cwd, git_branch, git_repo_root, model, title, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, estimated_cost_usd, actual_cost_usd)
	      VALUES ('s1', 'desktop', NULL, 1000, '/Users/dev/proj', 'main', '/Users/dev/proj', 'claude-opus-5', 'Fix the parser bug', 0, 0, 0, 0, NULL, 0.0123)`)
	exec(`INSERT INTO messages (session_id, role, content, timestamp) VALUES ('s1', 'user', 'please fix the parser bug', 1000)`)
	exec(`INSERT INTO messages (session_id, role, content, timestamp) VALUES ('s1', 'assistant', 'looking into it', 1002)`)
	exec(`INSERT INTO messages (session_id, role, content, timestamp) VALUES ('s1', 'tool', 'ran grep', 1003)`)
	exec(`INSERT INTO messages (session_id, role, content, timestamp) VALUES ('s1', 'user', '[OUT-OF-BAND USER MESSAGE] steer now', 1004)`)
	exec(`INSERT INTO messages (session_id, role, content, timestamp) VALUES ('s1', 'user', '[ASYNC DELEGATION BATCH COMPLETE] done', 1005)`)
	exec(`INSERT INTO messages (session_id, role, content, timestamp) VALUES ('s1', 'user', 'thanks, looks good', 1010)`)
	exec(`INSERT INTO session_model_usage (session_id, model, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens) VALUES ('s1', 'claude-opus-5', 100, 50, 10, 5)`)

	// s1-child: a subagent session spawned from s1; must not become its
	// own Session, but its usage should be folded into s1's Calls.
	exec(`INSERT INTO sessions (id, source, parent_session_id, started_at, cwd, git_branch, git_repo_root, model, title, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, estimated_cost_usd, actual_cost_usd)
	      VALUES ('s1-child', 'subagent', 's1', 1001, NULL, NULL, NULL, 'claude-opus-5', NULL, 0, 0, 0, 0, NULL, NULL)`)
	exec(`INSERT INTO messages (session_id, role, content, timestamp) VALUES ('s1-child', 'assistant', 'subagent work', 1001.5)`)
	exec(`INSERT INTO session_model_usage (session_id, model, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens) VALUES ('s1-child', 'claude-opus-5', 20, 10, 0, 0)`)

	// s2: no title recorded, no session_model_usage rows, so the title
	// falls back to the first Human prompt and Calls falls back to the
	// session's own cumulative token columns.
	exec(`INSERT INTO sessions (id, source, parent_session_id, started_at, cwd, git_branch, git_repo_root, model, title, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, estimated_cost_usd, actual_cost_usd)
	      VALUES ('s2', 'cli', NULL, 2000, '/Users/dev/other', NULL, NULL, 'claude-haiku-5', NULL, 40, 20, 0, 0, 0.001, NULL)`)
	exec(`INSERT INTO messages (session_id, role, content, timestamp) VALUES ('s2', 'user', 'how do I run this', 2000)`)
	exec(`INSERT INTO messages (session_id, role, content, timestamp) VALUES ('s2', 'assistant', 'like this', 2001)`)

	// s3: a cron session, must be skipped entirely (no row, no Session).
	exec(`INSERT INTO sessions (id, source, parent_session_id, started_at, cwd, git_branch, git_repo_root, model, title, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, estimated_cost_usd, actual_cost_usd)
	      VALUES ('s3', 'cron', NULL, 3000, NULL, NULL, NULL, 'claude-opus-5', 'Nightly job', 0, 0, 0, 0, NULL, NULL)`)
	exec(`INSERT INTO messages (session_id, role, content, timestamp) VALUES ('s3', 'user', '[Cronjob "Nightly"] run now', 3000)`)

	// s4: only injected/system user rows and no real human prompt, so it
	// must have zero Human events (caller drops it, but we still verify
	// the reader does not count the injected rows as Human).
	exec(`INSERT INTO sessions (id, source, parent_session_id, started_at, cwd, git_branch, git_repo_root, model, title, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, estimated_cost_usd, actual_cost_usd)
	      VALUES ('s4', 'desktop', NULL, 4000, NULL, NULL, NULL, NULL, NULL, 0, 0, 0, 0, NULL, NULL)`)
	exec(`INSERT INTO messages (session_id, role, content, timestamp) VALUES ('s4', 'user', '[System: active skill loaded]', 4000)`)
	exec(`INSERT INTO messages (session_id, role, content, timestamp) VALUES ('s4', 'assistant', 'ack', 4001)`)
}

func TestHermesReaderSource(t *testing.T) {
	if got := (HermesReader{}).Source(); got != "hermes" {
		t.Fatalf("Source() = %q, want hermes", got)
	}
}

func TestHermesReaderDiscover(t *testing.T) {
	home := t.TempDir()

	r := HermesReader{}
	paths, err := r.Discover(home)
	if err != nil {
		t.Fatalf("Discover on missing dirs: %v", err)
	}
	if paths != nil {
		t.Fatalf("Discover on missing dirs = %v, want nil", paths)
	}

	if err := os.MkdirAll(filepath.Join(home, ".hermes", "profiles", "work"), 0o755); err != nil {
		t.Fatal(err)
	}
	rootDB := filepath.Join(home, ".hermes", "state.db")
	buildHermesFixture(t, rootDB)
	workDB := filepath.Join(home, ".hermes", "profiles", "work", "state.db")
	buildHermesFixture(t, workDB)

	paths, err = r.Discover(home)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("Discover found %d paths, want 2: %v", len(paths), paths)
	}
}

func TestHermesReaderRead(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "state.db")
	buildHermesFixture(t, dbPath)

	r := HermesReader{}
	sessions, err := r.Read(dbPath)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	byID := map[string]Session{}
	for _, s := range sessions {
		byID[s.ExternalID] = s
	}

	// cron (s3) and subagent (s1-child) must never appear as sessions.
	if len(sessions) != 3 {
		var ids []string
		for _, s := range sessions {
			ids = append(ids, s.ExternalID)
		}
		t.Fatalf("Read returned %d sessions, want 3: %v", len(sessions), ids)
	}

	s1, ok := byID["default/s1"]
	if !ok {
		t.Fatalf("missing session default/s1")
	}
	if s1.Title != "Fix the parser bug" {
		t.Errorf("s1 Title = %q, want tool-provided title", s1.Title)
	}
	if s1.Cwd != "/Users/dev/proj" {
		t.Errorf("s1 Cwd = %q", s1.Cwd)
	}
	if s1.CostUSD == nil || *s1.CostUSD != 0.0123 {
		t.Errorf("s1 CostUSD = %v, want 0.0123", s1.CostUSD)
	}
	if got := HumanTurns(s1.Events); got != 2 {
		t.Errorf("s1 HumanTurns = %d, want 2 (injected rows excluded)", got)
	}
	for _, e := range s1.Events {
		_ = e
	}
	if len(s1.Calls) != 1 {
		t.Fatalf("s1 Calls = %d, want 1 (merged with subagent child)", len(s1.Calls))
	}
	c := s1.Calls[0]
	if c.Model != "claude-opus-5" || c.Input != 120 || c.Output != 60 || c.CacheRead != 10 || c.CacheWrite != 5 {
		t.Errorf("s1 Calls[0] = %+v, want merged parent+child usage", c)
	}

	s2, ok := byID["default/s2"]
	if !ok {
		t.Fatalf("missing session default/s2")
	}
	if s2.Title != "how do I run this" {
		t.Errorf("s2 Title = %q, want first human prompt fallback", s2.Title)
	}
	if len(s2.Calls) != 1 || s2.Calls[0].Model != "claude-haiku-5" || s2.Calls[0].Input != 40 {
		t.Errorf("s2 Calls = %+v, want session-total fallback", s2.Calls)
	}
	if s2.CostUSD == nil || *s2.CostUSD != 0.001 {
		t.Errorf("s2 CostUSD = %v, want estimated_cost_usd fallback 0.001", s2.CostUSD)
	}

	if _, ok := byID["default/s3"]; ok {
		t.Errorf("cron session s3 must be skipped")
	}
	if _, ok := byID["default/s1-child"]; ok {
		t.Errorf("subagent session s1-child must be skipped as its own session")
	}

	s4, ok := byID["default/s4"]
	if !ok {
		t.Fatalf("missing session default/s4")
	}
	if HumanTurns(s4.Events) != 0 {
		t.Errorf("s4 should have 0 Human events, got %d", HumanTurns(s4.Events))
	}
}

func TestHermesReaderProfileExternalID(t *testing.T) {
	dir := t.TempDir()
	profDir := filepath.Join(dir, "profiles", "work")
	if err := os.MkdirAll(profDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(profDir, "state.db")
	buildHermesFixture(t, dbPath)

	r := HermesReader{}
	sessions, err := r.Read(dbPath)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	found := false
	for _, s := range sessions {
		if s.ExternalID == "work/s1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected ExternalID work/s1, got sessions %+v", sessions)
	}
}

func TestHermesReaderMalformedPathReturnsError(t *testing.T) {
	r := HermesReader{}
	_, err := r.Read(filepath.Join(t.TempDir(), "does-not-exist.db"))
	if err == nil {
		t.Fatalf("Read on missing file should error (no such table), got nil")
	}
}

func TestHermesIsInjected(t *testing.T) {
	cases := []struct {
		content string
		want    bool
	}{
		{"please fix the bug", false},
		{"[15:10, 17.09.2026] Alice: hi there", false}, // pasted chat, still typed by the human
		{"[OUT-OF-BAND USER MESSAGE — steer]", true},
		{"[ASYNC DELEGATION BATCH COMPLETE]", true},
		{"[System: active skill loaded]", true},
		{"[Note: reminder]", true},
		{"[IMPORTANT: background process]", true},
		{"[CONTEXT COMPACTION — reference]", true},
		{"[Cronjob \"daily\"] ran", true},
		{"[Your active task list was preserved]", true},
		{"  [System: padded]", true},
	}
	for _, c := range cases {
		if got := hermesIsInjected(c.content); got != c.want {
			t.Errorf("hermesIsInjected(%q) = %v, want %v", c.content, got, c.want)
		}
	}
}

func TestHermesUnixToTime(t *testing.T) {
	got := hermesUnixToTime(1000.5)
	want := time.Unix(1000, 500000000).UTC()
	if !got.Equal(want) {
		t.Errorf("hermesUnixToTime(1000.5) = %v, want %v", got, want)
	}
}

// A conversation compressed into a fresh session continues the same work:
// the continuation is its own session, and its usage is not folded into
// the parent (that would count it twice).
func TestHermesCompressionContinuationIsKept(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	buildHermesFixture(t, path)
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	mustExec := func(q string) {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	mustExec(`INSERT INTO sessions (id, source, end_reason, started_at) VALUES ('long', 'desktop', 'compression', 5000)`)
	mustExec(`INSERT INTO sessions (id, source, parent_session_id, started_at) VALUES ('long-2', 'desktop', 'long', 6000)`)
	mustExec(`INSERT INTO messages (session_id, role, content, timestamp) VALUES ('long', 'user', 'start the refactor', 5000), ('long', 'assistant', 'ok', 5060)`)
	mustExec(`INSERT INTO messages (session_id, role, content, timestamp) VALUES ('long-2', 'user', 'keep going', 6000), ('long-2', 'assistant', 'done', 6120)`)
	mustExec(`INSERT INTO session_model_usage (session_id, model, input_tokens) VALUES ('long', 'claude-sonnet-5', 10), ('long-2', 'claude-sonnet-5', 7)`)
	db.Close()

	got, err := HermesReader{}.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]Session{}
	for _, s := range got {
		byID[s.ExternalID] = s
	}
	parent, okP := byID["default/long"]
	cont, okC := byID["default/long-2"]
	if !okP || !okC {
		t.Fatalf("want both sessions, got %v", byID)
	}
	sum := func(s Session) (n int64) {
		for _, c := range s.Calls {
			n += c.Input
		}
		return
	}
	if sum(parent) != 10 || sum(cont) != 7 {
		t.Fatalf("usage parent=%d continuation=%d, want 10 and 7", sum(parent), sum(cont))
	}
}
