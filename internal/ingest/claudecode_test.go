package ingest

import (
	"path/filepath"
	"testing"
	"time"
)

func TestClaudeCodeSource(t *testing.T) {
	if (ClaudeCodeReader{}).Source() != "claude-code" {
		t.Fatal("wrong source id")
	}
}

func TestClaudeCodeDiscover(t *testing.T) {
	home := filepath.Join("testdata", "claude-code", "home")
	paths, err := (ClaudeCodeReader{}).Discover(home)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(home, ".claude", "projects", "-Users-dev-demo", "11111111-1111-1111-1111-111111111111.jsonl"),
		filepath.Join(home, ".claude", "projects", "-Users-dev-demo", "22222222-2222-2222-2222-222222222222.jsonl"),
		filepath.Join(home, ".claude", "projects", "-Users-dev-demo", "44444444-4444-4444-4444-444444444444.jsonl"),
	}
	if len(paths) != len(want) {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
	for i, p := range want {
		if paths[i] != p {
			t.Fatalf("paths[%d] = %q, want %q", i, paths[i], p)
		}
	}
	// The subagent transcript must never be discovered as its own session.
	for _, p := range paths {
		if filepath.Base(filepath.Dir(p)) == "subagents" {
			t.Fatalf("discovered a subagent file as a session: %s", p)
		}
	}
}

func TestClaudeCodeDiscoverMissingDir(t *testing.T) {
	paths, err := (ClaudeCodeReader{}).Discover(filepath.Join("testdata", "claude-code", "does-not-exist"))
	if err != nil {
		t.Fatal(err)
	}
	if paths != nil {
		t.Fatalf("paths = %v, want nil", paths)
	}
}

func TestClaudeCodeReadBasicSession(t *testing.T) {
	path := filepath.Join("testdata", "claude-code", "home", ".claude", "projects", "-Users-dev-demo",
		"11111111-1111-1111-1111-111111111111.jsonl")
	sessions, err := (ClaudeCodeReader{}).Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	s := sessions[0]

	if s.Source != "claude-code" {
		t.Errorf("Source = %q", s.Source)
	}
	if s.ExternalID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("ExternalID = %q", s.ExternalID)
	}
	// custom-title wins over the first Human prompt.
	if s.Title != "Health check endpoint" {
		t.Errorf("Title = %q", s.Title)
	}
	if s.Cwd != "/Users/dev/demo" || s.GitBranch != "main" {
		t.Errorf("Cwd/GitBranch = %q/%q", s.Cwd, s.GitBranch)
	}

	if got := HumanTurns(s.Events); got != 1 {
		t.Fatalf("HumanTurns = %d, want 1 (only the typed prompt, not the tool_result)", got)
	}
	wantKinds := []EventKind{Human, Agent, Agent, Agent, Agent}
	if len(s.Events) != len(wantKinds) {
		t.Fatalf("got %d events, want %d: %+v", len(s.Events), len(wantKinds), s.Events)
	}
	for i, k := range wantKinds {
		if s.Events[i].Kind != k {
			t.Errorf("Events[%d].Kind = %v, want %v", i, s.Events[i].Kind, k)
		}
	}
	if !s.Events[0].At.Equal(time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("Events[0].At = %v", s.Events[0].At)
	}

	// msg_A1 is split across two content-block lines with identical usage:
	// must be counted once. msg_A2 is a second, separate call.
	if len(s.Calls) != 2 {
		t.Fatalf("got %d calls, want 2 (usage deduped by message.id): %+v", len(s.Calls), s.Calls)
	}
	if s.Calls[0].Input != 10 || s.Calls[0].Output != 20 || s.Calls[0].CacheRead != 200 || s.Calls[0].CacheWrite != 100 {
		t.Errorf("Calls[0] = %+v", s.Calls[0])
	}
	if s.Calls[1].Input != 5 || s.Calls[1].Output != 15 || s.Calls[1].CacheRead != 300 {
		t.Errorf("Calls[1] = %+v", s.Calls[1])
	}
}

func TestClaudeCodeReadSkipsMetaAndUsesFirstHumanTitle(t *testing.T) {
	path := filepath.Join("testdata", "claude-code", "home", ".claude", "projects", "-Users-dev-demo",
		"22222222-2222-2222-2222-222222222222.jsonl")
	sessions, err := (ClaudeCodeReader{}).Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	s := sessions[0]

	if s.Title != "Refactor the logger module please" {
		t.Errorf("Title = %q, want the first real human prompt (isMeta skill injection must not win)", s.Title)
	}
	if got := HumanTurns(s.Events); got != 1 {
		t.Fatalf("HumanTurns = %d, want 1 (isMeta content block is not a human prompt)", got)
	}
	// Human text prompt + isMeta injection (counted Agent) + assistant text.
	// The trailing system/local_command record carries no "user"/"assistant"
	// type and is not a timeline event at all.
	if len(s.Events) != 3 {
		t.Fatalf("got %d events, want 3: %+v", len(s.Events), s.Events)
	}
	if s.Events[0].Kind != Human || s.Events[1].Kind != Agent || s.Events[2].Kind != Agent {
		t.Fatalf("event kinds = %v, %v, %v", s.Events[0].Kind, s.Events[1].Kind, s.Events[2].Kind)
	}
}

func TestClaudeCodeReadSkipsSidechainSubagentFile(t *testing.T) {
	path := filepath.Join("testdata", "claude-code", "home", ".claude", "projects", "-Users-dev-demo",
		"33333333-3333-3333-3333-333333333333", "subagents", "agent-aaaaaaaaaaaaaaaaa.jsonl")
	sessions, err := (ClaudeCodeReader{}).Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Fatalf("got %d sessions, want 0: isSidechain/agentId records must never become a session", len(sessions))
	}
}

func TestClaudeCodeReadNoEvents(t *testing.T) {
	path := filepath.Join("testdata", "claude-code", "home", ".claude", "projects", "-Users-dev-demo",
		"44444444-4444-4444-4444-444444444444.jsonl")
	sessions, err := (ClaudeCodeReader{}).Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Fatalf("got %d sessions, want 0 (file has no user/assistant records)", len(sessions))
	}
}

func TestClaudeCodeReadMalformedLineDoesNotFailFile(t *testing.T) {
	// session 1's fixture embeds one deliberately invalid JSON line between
	// the tool_result and the final assistant message; Read must still
	// return the session with the valid records on either side of it.
	path := filepath.Join("testdata", "claude-code", "home", ".claude", "projects", "-Users-dev-demo",
		"11111111-1111-1111-1111-111111111111.jsonl")
	sessions, err := (ClaudeCodeReader{}).Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("malformed line sank the whole file: got %d sessions", len(sessions))
	}
}

func TestClaudeCodeStableExternalID(t *testing.T) {
	path := filepath.Join("testdata", "claude-code", "home", ".claude", "projects", "-Users-dev-demo",
		"11111111-1111-1111-1111-111111111111.jsonl")
	s1, err := (ClaudeCodeReader{}).Read(path)
	if err != nil {
		t.Fatal(err)
	}
	s2, err := (ClaudeCodeReader{}).Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if s1[0].ExternalID != s2[0].ExternalID {
		t.Fatalf("ExternalID not stable across re-reads: %q vs %q", s1[0].ExternalID, s2[0].ExternalID)
	}
}
