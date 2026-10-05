package ingest

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCodexReader_Source(t *testing.T) {
	if (CodexReader{}).Source() != "codex" {
		t.Fatal("unexpected source id")
	}
}

func TestCodexReader_DiscoverMissingDir(t *testing.T) {
	r := CodexReader{}
	home := t.TempDir() // no .codex at all
	paths, err := r.Discover(home)
	if err != nil {
		t.Fatalf("Discover returned error for missing dir: %v", err)
	}
	if paths != nil {
		t.Fatalf("expected nil paths, got %v", paths)
	}
}

func TestCodexReader_DiscoverFindsRolloutFiles(t *testing.T) {
	r := CodexReader{}
	home := t.TempDir()
	day := filepath.Join(home, ".codex", "sessions", "2026", "01", "02")
	if err := os.MkdirAll(day, 0o755); err != nil {
		t.Fatal(err)
	}
	good := filepath.Join(day, "rollout-2026-01-02T00-00-00-abc.jsonl")
	if err := os.WriteFile(good, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Non-matching files must not be picked up.
	if err := os.WriteFile(filepath.Join(day, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	archivedDay := filepath.Join(home, ".codex", "archived_sessions", "2026", "01", "03")
	if err := os.MkdirAll(archivedDay, 0o755); err != nil {
		t.Fatal(err)
	}
	archived := filepath.Join(archivedDay, "rollout-2026-01-03T00-00-00-def.jsonl")
	if err := os.WriteFile(archived, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	paths, err := r.Discover(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 rollout files, got %d: %v", len(paths), paths)
	}
}

func TestCodexReader_Basic(t *testing.T) {
	r := CodexReader{}
	sessions, err := r.Read(filepath.Join("testdata", "codex", "basic.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	s := sessions[0]

	if s.Source != "codex" {
		t.Errorf("Source = %q", s.Source)
	}
	if s.ExternalID != "sess-basic-1" {
		t.Errorf("ExternalID = %q", s.ExternalID)
	}
	if s.Cwd != "/Users/dev/proj" {
		t.Errorf("Cwd = %q", s.Cwd)
	}
	if s.GitBranch != "main" {
		t.Errorf("GitBranch = %q", s.GitBranch)
	}
	// Title comes from the first real Human prompt (a response_item with
	// role "user" and no injected tag), not the developer/plugin-list noise
	// that precedes the next user-role item.
	if s.Title != "Fix the failing build" {
		t.Errorf("Title = %q", s.Title)
	}

	human := HumanTurns(s.Events)
	if human != 2 {
		t.Errorf("HumanTurns = %d, want 2 (1 response_item prompt + 1 event_msg user_message)", human)
	}
	agent := 0
	for _, e := range s.Events {
		if e.Kind == Agent {
			agent++
		}
	}
	// reasoning + function_call + function_call_output + 2 assistant
	// messages = 5. The developer item, the <recommended_plugins> item, and
	// the item_completed duplicate must NOT add events.
	if agent != 5 {
		t.Errorf("Agent events = %d, want 5", agent)
	}
	if len(s.Events) != human+agent {
		t.Errorf("total events = %d, want %d", len(s.Events), human+agent)
	}

	// Calls: one per turn's token_count, using last_token_usage (per-turn,
	// not the cumulative total_token_usage), with the duplicate token_count
	// at the end of turn 2 deduped away.
	if len(s.Calls) != 2 {
		t.Fatalf("expected 2 ModelCalls, got %d: %+v", len(s.Calls), s.Calls)
	}
	c0, c1 := s.Calls[0], s.Calls[1]
	if c0.Model != "gpt-5.1-codex" || c0.Input != 800 || c0.Output != 50 || c0.CacheRead != 200 {
		t.Errorf("Calls[0] = %+v", c0)
	}
	if c1.Model != "gpt-5.1-codex-mini" || c1.Input != 500 || c1.Output != 70 || c1.CacheRead != 300 {
		t.Errorf("Calls[1] = %+v", c1)
	}

	wantStart := time.Date(2026, 1, 1, 9, 0, 2, 0, time.UTC)
	if !s.Start().Equal(wantStart) {
		t.Errorf("Start = %v, want %v", s.Start(), wantStart)
	}
}

func TestCodexReader_MalformedLinesAreSkipped(t *testing.T) {
	r := CodexReader{}
	sessions, err := r.Read(filepath.Join("testdata", "codex", "malformed.jsonl"))
	if err != nil {
		t.Fatalf("Read must not fail on malformed lines: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	s := sessions[0]
	// The truncated-JSON assistant message, the line that isn't JSON at
	// all, and the record with an unparseable timestamp must all be
	// skipped without aborting the file; the valid user_message and the
	// valid assistant message must still come through.
	if human := HumanTurns(s.Events); human != 1 {
		t.Errorf("HumanTurns = %d, want 1", human)
	}
	agent := 0
	for _, e := range s.Events {
		if e.Kind == Agent {
			agent++
		}
	}
	if agent != 1 {
		t.Errorf("Agent events = %d, want 1", agent)
	}
}

func TestCodexReader_SessionWithNoRealEventsIsDropped(t *testing.T) {
	r := CodexReader{}
	sessions, err := r.Read(filepath.Join("testdata", "codex", "empty.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if sessions != nil {
		t.Fatalf("expected no sessions (only injected developer content), got %d", len(sessions))
	}
}

func TestCodexReader_SubagentTurnFoldsIntoParentSession(t *testing.T) {
	r := CodexReader{}
	sessions, err := r.Read(filepath.Join("testdata", "codex", "subagent.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	// A child turn (turn_context.turn_id != root_turn_id) never gets its
	// own rollout file in real Codex data; it must not produce a second
	// Session here either, and its Agent events / token usage must still
	// be present on the one session this file yields.
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	s := sessions[0]
	if human := HumanTurns(s.Events); human != 1 {
		t.Errorf("HumanTurns = %d, want 1", human)
	}
	if len(s.Calls) != 1 {
		t.Fatalf("expected 1 ModelCall (from the child turn's token_count), got %d", len(s.Calls))
	}
	if s.Calls[0].Model != "gpt-5.1-codex-mini" {
		t.Errorf("Calls[0].Model = %q", s.Calls[0].Model)
	}
	// function_call + 2 assistant messages are response_item records = 3.
	// The line using the unhandled top-level type "function_call_output"
	// (instead of "response_item") exercises the default/ignore path for
	// unknown top-level record types, so it contributes no event.
	agent := 0
	for _, e := range s.Events {
		if e.Kind == Agent {
			agent++
		}
	}
	if agent != 3 {
		t.Errorf("Agent events = %d, want 3", agent)
	}
}

func TestCodexReader_NoSessionMetaYieldsNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rollout-no-meta.jsonl")
	content := `{"timestamp":"2026-05-01T00:00:00.000Z","type":"event_msg","payload":{"type":"user_message","message":"hi"}}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	r := CodexReader{}
	sessions, err := r.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if sessions != nil {
		t.Fatalf("expected nil sessions without a session_meta external id, got %v", sessions)
	}
}
