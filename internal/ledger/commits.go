// Package ledger joins sessions (hand-kept and imported) with the commits
// they produced, and summarises a period of work.
package ledger

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/tolgazorlu/btrack/internal/db"
)

const (
	recSep   = "\x1e"
	fieldSep = "\x1f"
)

// ScanCommits returns the commits authored by the repo's configured
// user.email on any local branch since the given time. Commits by other
// people are not the user's work and are left out; agents committing on
// the user's behalf use the same identity, so they are included.
func ScanCommits(repoDir, project string, since time.Time) ([]*db.Commit, error) {
	email, err := exec.Command("git", "-C", repoDir, "config", "user.email").Output()
	if err != nil || strings.TrimSpace(string(email)) == "" {
		return nil, fmt.Errorf("%s: no git user.email configured", repoDir)
	}
	args := []string{"-C", repoDir, "log", "--branches", "--no-merges",
		"--author=" + strings.TrimSpace(string(email)),
		"--pretty=format:" + recSep + "%H" + fieldSep + "%aI" + fieldSep + "%s",
		"--numstat",
	}
	if !since.IsZero() {
		args = append(args, "--since="+since.Format(time.RFC3339))
	}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("git log in %s: %w", repoDir, err)
	}
	return parseLog(string(out), repoDir, project), nil
}

func parseLog(out, repoDir, project string) []*db.Commit {
	var commits []*db.Commit
	for _, rec := range strings.Split(out, recSep) {
		rec = strings.TrimSpace(rec)
		if rec == "" {
			continue
		}
		lines := strings.Split(rec, "\n")
		head := strings.SplitN(lines[0], fieldSep, 3)
		if len(head) < 3 {
			continue
		}
		at, err := time.Parse(time.RFC3339, head[1])
		if err != nil {
			continue
		}
		c := &db.Commit{Project: project, RepoPath: repoDir, SHA: head[0], AuthoredAt: at, Subject: head[2]}
		for _, l := range lines[1:] {
			f := strings.Split(l, "\t")
			if len(f) < 3 {
				continue
			}
			c.Files++
			// Binary files report "-" for both counts.
			if n, err := strconv.Atoi(f[0]); err == nil {
				c.Insertions += n
			}
			if n, err := strconv.Atoi(f[1]); err == nil {
				c.Deletions += n
			}
		}
		commits = append(commits, c)
	}
	return commits
}

// LinkWindow is how far outside a session a commit may land and still be
// its product: a commit is often made just after the agent's last message
// (lead) or prepared a little before the session formally starts.
var LinkWindow = struct{ Before, After time.Duration }{5 * time.Minute, 15 * time.Minute}

// span is a stretch of work attributable to one session: a hand-kept
// session as a whole, or one segment of an imported session.
type span struct {
	sessionID  int64
	project    string
	start, end time.Time
}

func spans(sessions []*db.Session, segments []db.Segment) []span {
	byID := map[int64]*db.Session{}
	var out []span
	for _, s := range sessions {
		byID[s.ID] = s
		if s.Source == db.SourceManual && s.Project != "" && s.EndTime != nil {
			out = append(out, span{s.ID, s.Project, s.StartTime, *s.EndTime})
		}
	}
	for _, g := range segments {
		if s, ok := byID[g.SessionID]; ok && s.Project != "" {
			out = append(out, span{s.ID, s.Project, g.Start, g.End})
		}
	}
	return out
}

// Link picks, for each commit, the session that produced it: same
// project, commit time inside one of the session's stretches of work
// (a hand-kept session, or a segment of an imported one) widened by
// LinkWindow. A session left open for days only claims commits made
// while it was actually active. When several match, a stretch that
// contains the commit beats one that only matches through the widening,
// and among equals the one that started most recently wins. Returns
// commit ID -> session ID; unmatched commits are absent.
func Link(sessions []*db.Session, segments []db.Segment, commits []*db.Commit) map[int64]int64 {
	byProject := map[string][]span{}
	for _, sp := range spans(sessions, segments) {
		byProject[sp.project] = append(byProject[sp.project], sp)
	}
	out := map[int64]int64{}
	for _, c := range commits {
		var best *span
		bestInside := false
		for i := range byProject[c.Project] {
			sp := &byProject[c.Project][i]
			t := c.AuthoredAt
			if t.Before(sp.start.Add(-LinkWindow.Before)) || t.After(sp.end.Add(LinkWindow.After)) {
				continue
			}
			inside := !t.Before(sp.start) && !t.After(sp.end)
			if best == nil || (inside && !bestInside) ||
				(inside == bestInside && sp.start.After(best.start)) {
				best, bestInside = sp, inside
			}
		}
		if best != nil {
			out[c.ID] = best.sessionID
		}
	}
	return out
}
