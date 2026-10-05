package ingest

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func mkRepo(t *testing.T, dir, origin string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "[core]\n\tbare = false\n"
	if origin != "" {
		cfg += "[remote \"origin\"]\n\turl = " + origin + "\n"
	}
	os.WriteFile(filepath.Join(dir, ".git", "config"), []byte(cfg), 0o644)
}

func TestProjectResolver(t *testing.T) {
	home := t.TempDir()
	mkRepo(t, filepath.Join(home, "code", "shop"), "git@github.com:me/shop-api.git")
	mkRepo(t, filepath.Join(home, "code", "plain"), "")
	os.MkdirAll(filepath.Join(home, "code", "plain", "sub"), 0o755)
	os.MkdirAll(filepath.Join(home, "code", "named"), 0o755)
	os.WriteFile(filepath.Join(home, "code", "named", ".btrack"), []byte("project=acme\n"), 0o644)

	// A linked worktree of shop: .git is a file pointing into the main repo.
	wt := filepath.Join(home, "wt-shop-feature")
	os.MkdirAll(wt, 0o755)
	os.WriteFile(filepath.Join(wt, ".git"),
		[]byte("gitdir: "+filepath.Join(home, "code", "shop", ".git", "worktrees", "feature")+"\n"), 0o644)

	r := &ProjectResolver{Home: home}
	cases := map[string]string{
		"":                                       "",
		home:                                     "",
		filepath.Join(home, "code", "shop"):      "shop-api",
		filepath.Join(home, "code", "plain/sub"): "plain",
		filepath.Join(home, "code", "named"):     "acme",
		wt:                                       "shop-api",
		filepath.Join(home, "gone", "old-app"):   "old-app",
		filepath.Join(home, ".cache", "scratch"): "",
		filepath.Join(home, "Library", "Application Support", "Claude", "scratch-workspaces", "x"): "",
		filepath.Join(home, "Documents", "Codex", "2026-09-08", "open-the-browser"):                "",
	}
	for cwd, want := range cases {
		if got := r.Resolve(cwd); got != want {
			t.Errorf("Resolve(%q) = %q, want %q", cwd, got, want)
		}
	}

	shop := filepath.Join(home, "code", "shop", "main.go")
	plain := filepath.Join(home, "code", "plain", "x.go")
	s := Session{Cwd: home, PathHints: []string{shop, shop, shop, plain, "/etc/hosts"}}
	if got := r.ResolveSession(s); got != "shop-api" {
		t.Errorf("hinted project = %q, want shop-api", got)
	}
	s.PathHints = []string{shop, shop, plain}
	if got := r.ResolveSession(s); got != "" {
		t.Errorf("weak hints claimed project %q", got)
	}

	r.Aliases = map[string]string{"shop-api": "shop"}
	if got := r.ResolveSession(Session{Cwd: filepath.Join(home, "code", "shop")}); got != "shop" {
		t.Errorf("alias not applied: %q", got)
	}
	if r.RepoDirs()[filepath.Join(home, "code", "shop")] != "shop" {
		t.Errorf("repo dirs not aliased: %v", r.RepoDirs())
	}
}

func TestToDB(t *testing.T) {
	home := t.TempDir()
	t0 := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	s := Session{
		Source: "claude-code", ExternalID: "id1", Title: "  fix   the\nbug ", Cwd: home,
		Events: []Event{{t0, Human}, {t0.Add(10 * time.Minute), Agent}},
		Calls: []ModelCall{
			{Model: "claude-sonnet-5", Input: 1_000_000, Output: 100_000},
			{Model: "claude-sonnet-5", CacheRead: 1_000_000},
		},
	}
	c := ToDB(s, &ProjectResolver{Home: home}, SplitOptions{})
	if c == nil {
		t.Fatal("nil conversion")
	}
	d := c.Session
	if d.TaskName != "fix the bug" || d.Model != "claude-sonnet-5" || d.Project != "" {
		t.Errorf("fields: %+v", d)
	}
	if !d.EndTime.Equal(t0.Add(10*time.Minute)) || !d.StartTime.Equal(t0.Add(-2*time.Minute)) {
		t.Errorf("times: %v - %v", d.StartTime, *d.EndTime)
	}
	if *d.ActiveSeconds != 120 || *d.AgentSeconds != 600 {
		t.Errorf("split: active=%d agent=%d", *d.ActiveSeconds, *d.AgentSeconds)
	}
	// $2 input + $1 output + $0.20 cache read at sonnet-5 list price.
	if d.CostUSD == nil || *d.CostUSD < 3.199 || *d.CostUSD > 3.201 {
		t.Errorf("cost = %v, want 3.20", d.CostUSD)
	}

	s.Calls = append(s.Calls, ModelCall{Model: "mystery-model", Input: 5})
	c = ToDB(s, &ProjectResolver{Home: home}, SplitOptions{})
	if c.Session.CostUSD != nil || c.UnknownModel != "mystery-model" {
		t.Errorf("partial pricing should leave cost unknown: %v %q", c.Session.CostUSD, c.UnknownModel)
	}

	if ToDB(Session{Events: []Event{{t0, Agent}}}, &ProjectResolver{Home: home}, SplitOptions{}) != nil {
		t.Error("session without a human prompt was kept")
	}
}
