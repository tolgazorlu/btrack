package cmd

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/db"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var shippedCmd = &cobra.Command{
	Use:     "shipped",
	Aliases: []string{"sh"},
	Short:   "Show git commits that landed during your sessions",
	Long: `Cross-reference btrack sessions with git history — for each session,
list the commits authored during that time window so you can compare what
you said you would do (task name) with what actually shipped.

Run from inside the repo where the session was tracked.

Usage:
  btrack shipped                last completed session
  btrack shipped -n 5           last 5 sessions
  btrack shipped --today        all sessions from today
  btrack shipped -i 42          a specific session by ID
  btrack sh                     short alias

Tips:
  · "said vs shipped" — task name is the intent, commits are the evidence
  · Active sessions (still running) use start → now as the window
  · All commits across branches in the window are included`,
	RunE: runShipped,
}

func runShipped(cmd *cobra.Command, args []string) error {
	n, _ := cmd.Flags().GetInt("num")
	today, _ := cmd.Flags().GetBool("today")
	sessID, _ := cmd.Flags().GetInt64("session")

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	store, err := db.Open(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	cwdRepo, err := gitToplevel()
	if err != nil {
		return fmt.Errorf("`btrack shipped` must run inside a git repository")
	}
	cwdRepoBase := filepath.Base(cwdRepo)

	sessions, err := pickShippedSessions(store, sessID, today, n)
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		ui.Header("shipped", "")
		ui.Hint("no sessions found")
		ui.Blank()
		return nil
	}

	subtitle := ""
	if len(sessions) > 1 {
		subtitle = fmt.Sprintf("%d sessions", len(sessions))
	}
	ui.Header("shipped", subtitle)

	totalCommits, totalIns, totalDel := 0, 0, 0
	for i, s := range sessions {
		end := time.Now()
		if s.EndTime != nil {
			end = *s.EndTime
		}
		commits := commitsInWindow(s.StartTime, end)

		printShippedSession(s, end, cwdRepoBase, commits)
		for _, c := range commits {
			totalIns += c.Insertions
			totalDel += c.Deletions
		}
		totalCommits += len(commits)
		if i < len(sessions)-1 {
			ui.Blank()
		}
	}

	if len(sessions) > 1 {
		ui.Rule()
		fmt.Printf("%s%s commits  %s  %s\n",
			ui.Indent,
			ui.StyleHighlight.Render(fmt.Sprintf("%d", totalCommits)),
			ui.StyleSuccess.Render(fmt.Sprintf("+%d", totalIns)),
			ui.StyleDimmed.Render(fmt.Sprintf("-%d", totalDel)),
		)
	}
	ui.Blank()
	return nil
}

func pickShippedSessions(store db.Store, id int64, today bool, n int) ([]*db.Session, error) {
	switch {
	case id > 0:
		s, err := store.GetSessionByID(id)
		if err != nil || s == nil {
			return nil, fmt.Errorf("session %d not found", id)
		}
		return []*db.Session{s}, nil
	case today:
		return store.GetSessionsForDate(time.Now())
	default:
		if n < 1 {
			n = 1
		}
		all, err := store.GetRecentSessions(n * 2)
		if err != nil {
			return nil, err
		}
		out := make([]*db.Session, 0, n)
		for _, s := range all {
			if len(out) >= n {
				break
			}
			out = append(out, s)
		}
		return out, nil
	}
}

func printShippedSession(s *db.Session, end time.Time, cwdRepo string, commits []commitInfo) {
	elapsed := end.Sub(s.StartTime)
	timeline := s.StartTime.Local().Format("Mon Jan 02 15:04") +
		ui.StyleDimmed.Render(" → ") +
		end.Local().Format("15:04")
	fmt.Printf("%s%s  %s\n",
		ui.Indent,
		ui.StyleHighlight.Render(timeline),
		ui.StyleElapsed.Render(formatDur(elapsed)),
	)

	ui.KV("said", ui.StyleHighlight.Render(s.TaskName))
	if s.Message != "" {
		ui.KV("done", ui.StyleHighlight.Render(s.Message))
	}
	if s.GitRepo != "" && cwdRepo != "" && s.GitRepo != cwdRepo {
		ui.KV("note", ui.StyleWarning.Render(
			fmt.Sprintf("session ran in @%s — you are now in @%s", s.GitRepo, cwdRepo),
		))
	}

	if len(commits) == 0 {
		ui.KV("commits", ui.StyleDimmed.Render("none in window"))
		return
	}

	ins, del := 0, 0
	for _, c := range commits {
		ins += c.Insertions
		del += c.Deletions
	}
	summary := ui.StyleHighlight.Render(fmt.Sprintf("%d", len(commits))) + " " +
		ui.StyleDimmed.Render(plural(len(commits), "commit", "commits")) + "  " +
		ui.StyleSuccess.Render(fmt.Sprintf("+%d", ins)) + "  " +
		ui.StyleDimmed.Render(fmt.Sprintf("-%d", del))
	ui.KV("commits", summary)

	for _, c := range commits {
		subj := c.Subject
		if len(subj) > 42 {
			subj = subj[:39] + "..."
		}
		fmt.Printf("%s  %s  %s  %s\n",
			ui.Indent+"  "+ui.StyleDimmed.Render(c.SHA),
			padVisible(ui.StyleHighlight.Render(subj), 42),
			ui.StyleSuccess.Render(fmt.Sprintf("+%-4d", c.Insertions)),
			ui.StyleDimmed.Render(fmt.Sprintf("-%d", c.Deletions)),
		)
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// ─── git helpers ─────────────────────────────────────────────────────────────

type commitInfo struct {
	SHA        string
	Subject    string
	Time       time.Time
	Files      int
	Insertions int
	Deletions  int
}

func gitToplevel() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func commitsInWindow(start, end time.Time) []commitInfo {
	out, err := exec.Command("git", "log",
		"--all",
		"--since="+start.UTC().Format(time.RFC3339),
		"--until="+end.UTC().Format(time.RFC3339),
		"--shortstat",
		"--pretty=format:COMMIT\x1f%H\x1f%s\x1f%aI",
	).Output()
	if err != nil {
		return nil
	}
	return parseGitLog(string(out))
}

func parseGitLog(out string) []commitInfo {
	var commits []commitInfo
	var cur *commitInfo
	flush := func() {
		if cur != nil {
			commits = append(commits, *cur)
			cur = nil
		}
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "COMMIT\x1f") {
			flush()
			parts := strings.SplitN(line, "\x1f", 4)
			if len(parts) != 4 {
				continue
			}
			t, _ := time.Parse(time.RFC3339, parts[3])
			sha := parts[1]
			if len(sha) > 7 {
				sha = sha[:7]
			}
			cur = &commitInfo{SHA: sha, Subject: parts[2], Time: t}
			continue
		}
		if cur != nil && strings.Contains(line, "changed") {
			cur.Files, cur.Insertions, cur.Deletions = parseShortstat(line)
		}
	}
	flush()
	return commits
}

// parseShortstat reads a "git log --shortstat" line like:
//
//	"1 file changed, 5 insertions(+), 2 deletions(-)"
func parseShortstat(line string) (files, ins, del int) {
	for _, p := range strings.Split(line, ",") {
		p = strings.TrimSpace(p)
		fields := strings.Fields(p)
		if len(fields) == 0 {
			continue
		}
		n, _ := strconv.Atoi(fields[0])
		switch {
		case strings.HasPrefix(p, fields[0]+" file"):
			files = n
		case strings.Contains(p, "insertion"):
			ins = n
		case strings.Contains(p, "deletion"):
			del = n
		}
	}
	return
}

func init() {
	shippedCmd.Flags().IntP("num", "n", 1, "last N completed sessions")
	shippedCmd.Flags().Bool("today", false, "all sessions from today")
	shippedCmd.Flags().Int64P("session", "i", 0, "specific session ID")
	rootCmd.AddCommand(shippedCmd)
}
