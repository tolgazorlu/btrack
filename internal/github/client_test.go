package github

import (
	"testing"
	"time"
)

func TestActivity_IsEmpty(t *testing.T) {
	empty := &Activity{}
	if !empty.IsEmpty() {
		t.Error("empty Activity should be IsEmpty() = true")
	}

	withCommit := &Activity{
		Commits: []*Commit{{SHA: "abc1234", Message: "fix bug"}},
	}
	if withCommit.IsEmpty() {
		t.Error("Activity with commits should not be empty")
	}

	withPR := &Activity{
		PullRequests: []*PullRequest{{Number: 1, Title: "my PR"}},
	}
	if withPR.IsEmpty() {
		t.Error("Activity with PRs should not be empty")
	}
}

func TestActivity_Summary(t *testing.T) {
	act := &Activity{
		Commits: []*Commit{
			{SHA: "abc1234", Message: "fix login bug", Repo: "owner/myapp"},
		},
		PullRequests: []*PullRequest{
			{Number: 42, Title: "Add auth", Repo: "owner/myapp", Action: "merged"},
		},
	}

	summary := act.Summary()
	if summary == "" {
		t.Error("Summary() should not be empty for non-empty activity")
	}
	if !contains(summary, "abc1234") {
		t.Error("Summary() should contain commit SHA")
	}
	if !contains(summary, "fix login bug") {
		t.Error("Summary() should contain commit message")
	}
	if !contains(summary, "#42") {
		t.Error("Summary() should contain PR number")
	}
	if !contains(summary, "merged") {
		t.Error("Summary() should contain PR action")
	}
}

func TestActivity_Summary_Empty(t *testing.T) {
	act := &Activity{}
	if act.Summary() != "" {
		t.Error("Summary() on empty Activity should return empty string")
	}
}

func TestSplitByDay(t *testing.T) {
	loc := time.Local
	day1 := time.Date(2026, 5, 1, 10, 0, 0, 0, loc)
	day2 := time.Date(2026, 5, 2, 14, 0, 0, 0, loc)

	act := &Activity{
		Commits: []*Commit{
			{SHA: "aaa1111", Message: "commit on day1", Repo: "owner/repo", Time: day1},
			{SHA: "bbb2222", Message: "commit on day2", Repo: "owner/repo", Time: day2},
			{SHA: "ccc3333", Message: "another on day1", Repo: "owner/repo", Time: day1},
		},
		PullRequests: []*PullRequest{
			{Number: 1, Title: "PR on day2", Action: "merged", Time: day2},
		},
	}

	byDay := SplitByDay(act)

	if len(byDay) != 2 {
		t.Errorf("SplitByDay returned %d days, want 2", len(byDay))
	}

	d1 := byDay["2026-05-01"]
	if d1 == nil {
		t.Fatal("missing bucket for 2026-05-01")
	}
	if len(d1.Commits) != 2 {
		t.Errorf("day1 commits = %d, want 2", len(d1.Commits))
	}

	d2 := byDay["2026-05-02"]
	if d2 == nil {
		t.Fatal("missing bucket for 2026-05-02")
	}
	if len(d2.Commits) != 1 {
		t.Errorf("day2 commits = %d, want 1", len(d2.Commits))
	}
	if len(d2.PullRequests) != 1 {
		t.Errorf("day2 PRs = %d, want 1", len(d2.PullRequests))
	}
}

func TestSplitByDay_Empty(t *testing.T) {
	byDay := SplitByDay(&Activity{})
	if len(byDay) != 0 {
		t.Errorf("SplitByDay on empty activity returned %d buckets, want 0", len(byDay))
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsHelper(s, sub))
}

func containsHelper(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
