package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/db"
	"github.com/tolgazorlu/btrack/internal/report"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var reportCmd = &cobra.Command{
	Use:     "report [project]",
	Aliases: []string{"rapor", "rep"},
	Short:   "Build a client work report (commits + tracked hours) as HTML",
	Long: `Build a two-part client report for a date range:

  Part 1 — what was done: every non-merge commit on the current branch,
           grouped by day, tagged feat/fix/chore/revert, with PR references.
  Part 2 — tracked hours: your btrack sessions for the same range, with
           per-day totals and a grand total.

The report is written to your home directory as a self-contained HTML file.

Usage:
  btrack report yalcinkaya --from 21.08 --to 16.09
  btrack report yalcinkaya --from 2026-08-21 --to 2026-09-16
  btrack report yalcinkaya -m                       this month
  btrack report yalcinkaya --repo ~/projects/app    scan another repo

Dates accept 21.08, 21.08.2026, or 2026-08-21. A bare day-month uses the
current year.

Flags of note:
  --repo    which git repository to scan (default: the project's configured
            repo, else the current directory)
  --out     write somewhere other than the home directory
  --open    open the finished report in your browser`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project := ""
		if len(args) == 1 {
			project = args[0]
		}

		fromStr, _ := cmd.Flags().GetString("from")
		toStr, _ := cmd.Flags().GetString("to")
		thisMonth, _ := cmd.Flags().GetBool("month")
		repoPath, _ := cmd.Flags().GetString("repo")
		outPath, _ := cmd.Flags().GetString("out")
		clientName, _ := cmd.Flags().GetString("client")
		author, _ := cmd.Flags().GetString("author")
		openAfter, _ := cmd.Flags().GetBool("open")

		from, to, err := resolveRange(fromStr, toStr, thisMonth)
		if err != nil {
			return err
		}

		if repoPath == "" {
			repoPath, _ = os.Getwd()
		}
		repoPath = expandHome(repoPath)

		commits, err := report.ScanGit(repoPath, from, to)
		if err != nil {
			return err
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		store, err := db.Open(cfg)
		if err != nil {
			return err
		}
		defer store.Close()

		entries, err := collectEntries(store, project, from, to)
		if err != nil {
			return err
		}

		if len(commits) == 0 && len(entries) == 0 {
			ui.Blank()
			ui.Warn("no commits or sessions in that range — nothing to report")
			ui.Hint(fmt.Sprintf("repo: %s   range: %s → %s",
				repoPath, from.Format("2006-01-02"), to.Format("2006-01-02")))
			ui.Blank()
			return nil
		}

		if clientName == "" {
			clientName = displayName(project, repoPath)
		}
		if author == "" {
			author = gitUserName(repoPath)
		}

		files, added, removed := report.DiffStat(repoPath, from, to)

		rep := report.Report{
			Client:       clientName,
			From:         from,
			To:           to,
			Days:         report.Build(commits, entries),
			PreparedBy:   author,
			GeneratedAt:  time.Now(),
			ToolVersion:  displayVersion(Version),
			FilesChanged: files,
			LinesAdded:   added,
			LinesRemoved: removed,
		}

		if outPath == "" {
			home, _ := os.UserHomeDir()
			name := fmt.Sprintf("%s-calisma-raporu-%s-%s.html",
				slug(clientName), from.Format("20060102"), to.Format("20060102"))
			outPath = filepath.Join(home, name)
		}
		outPath = expandHome(outPath)

		f, err := os.Create(filepath.Clean(outPath))
		if err != nil {
			return fmt.Errorf("create report: %w", err)
		}
		if err := rep.Render(f); err != nil {
			f.Close()
			return err
		}
		f.Close()

		ui.Blank()
		ui.OK(fmt.Sprintf("report ready → %s", ui.StyleHighlight.Render(outPath)))
		ui.KV("range", from.Format("02.01.2006")+" – "+to.Format("02.01.2006"))
		ui.KV("commits", fmt.Sprintf("%d", rep.TotalCommits()))
		ui.KV("tracked", fmt.Sprintf("%s (%d sessions)", rep.TotalLabel(), rep.TotalEntries()))
		if rep.TotalEntries() == 0 {
			ui.Blank()
			ui.Warn("no tracked hours in this range — part 2 is empty")
			ui.Hint("record time with: btrack s \"task\" -p " + project)
		}
		ui.Blank()

		if openAfter {
			_ = openBrowser("file://" + outPath)
		}
		return nil
	},
}

// collectEntries pulls stopped sessions in [from, to] for a project.
func collectEntries(store db.Store, project string, from, to time.Time) ([]report.Entry, error) {
	var sessions []*db.Session
	var err error
	if project != "" {
		sessions, err = store.GetSessionsByProject(project, 10000)
	} else {
		sessions, err = store.GetRecentSessions(10000)
	}
	if err != nil {
		return nil, fmt.Errorf("load sessions: %w", err)
	}

	end := to.AddDate(0, 0, 1)
	var out []report.Entry
	for _, s := range sessions {
		if s.EndTime == nil {
			continue // active session: not billable yet
		}
		st := s.StartTime.Local()
		if st.Before(from) || !st.Before(end) {
			continue
		}
		desc := s.Message
		if strings.TrimSpace(desc) == "" {
			desc = s.TaskName
		}
		desc, flag := splitTrailingFlag(desc)
		out = append(out, report.Entry{
			Date:        st,
			Duration:    s.Duration(),
			Description: desc,
			Flag:        flag,
		})
	}
	return out, nil
}

// flagSuffix matches a bracketed note at the end of a description, e.g.
// "Mobil uygulama hata kontrolü [commit yok]". The report renders it as a
// chip beside the entry rather than as part of the sentence.
var flagSuffix = regexp.MustCompile(`\s*\[([^\]]{1,24})\]\s*$`)

func splitTrailingFlag(desc string) (string, string) {
	m := flagSuffix.FindStringSubmatch(desc)
	if m == nil {
		return desc, ""
	}
	return strings.TrimSpace(flagSuffix.ReplaceAllString(desc, "")), strings.TrimSpace(m[1])
}

// resolveRange turns the flags into a concrete [from, to] window.
func resolveRange(fromStr, toStr string, thisMonth bool) (time.Time, time.Time, error) {
	now := time.Now()
	if thisMonth {
		from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		return from, endOfDay(now), nil
	}
	if fromStr == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("need a range: --from 21.08 --to 16.09 (or -m for this month)")
	}
	from, err := parseReportDate(fromStr, now)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("--from: %w", err)
	}
	to := now
	if toStr != "" {
		to, err = parseReportDate(toStr, now)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("--to: %w", err)
		}
	}
	if to.Before(from) {
		return time.Time{}, time.Time{}, fmt.Errorf("--to is before --from")
	}
	return from, endOfDay(to), nil
}

func endOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 23, 59, 59, 0, t.Location())
}

// parseReportDate accepts 21.08, 21.08.2026, 2026-08-21 and 21/08.
func parseReportDate(s string, ref time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	layouts := []string{"02.01.2006", "2006-01-02", "02/01/2006", "02-01-2006"}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, s, ref.Location()); err == nil {
			return t, nil
		}
	}
	// Day-month only: assume the reference year.
	for _, l := range []string{"02.01", "02/01", "02-01"} {
		if t, err := time.ParseInLocation(l, s, ref.Location()); err == nil {
			return time.Date(ref.Year(), t.Month(), t.Day(), 0, 0, 0, 0, ref.Location()), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised date %q — try 21.08 or 2026-08-21", s)
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

// displayName produces a human client label from the project or repo folder.
func displayName(project, repoPath string) string {
	name := project
	if name == "" {
		name = filepath.Base(repoPath)
	}
	name = strings.NewReplacer("-", " ", "_", " ").Replace(name)
	words := strings.Fields(name)
	for i, w := range words {
		r := []rune(w)
		if len(r) > 0 {
			words[i] = strings.ToUpper(string(r[0])) + string(r[1:])
		}
	}
	return strings.Join(words, " ")
}

func slug(s string) string {
	repl := strings.NewReplacer(
		"ı", "i", "İ", "i", "ş", "s", "Ş", "s", "ğ", "g", "Ğ", "g",
		"ü", "u", "Ü", "u", "ö", "o", "Ö", "o", "ç", "c", "Ç", "c",
		" ", "-", "_", "-",
	)
	return strings.ToLower(repl.Replace(s))
}

func gitUserName(repoPath string) string {
	out, err := exec.Command("git", "-C", repoPath, "config", "user.name").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// displayVersion cleans a build-time version for print. Developer builds carry
// suffixes like "+dirty" or a pseudo-version with a commit hash; a client
// document should show "v0.6.8" or nothing at all, never build plumbing.
func displayVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == "dev" {
		return ""
	}
	if i := strings.IndexAny(v, "-+"); i > 0 {
		v = v[:i]
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return v
}

func init() {
	reportCmd.Flags().String("from", "", "range start (21.08 or 2026-08-21)")
	reportCmd.Flags().String("to", "", "range end (defaults to today)")
	reportCmd.Flags().BoolP("month", "m", false, "this calendar month")
	reportCmd.Flags().String("repo", "", "git repository to scan (default: current directory)")
	reportCmd.Flags().StringP("out", "o", "", "output path (default: ~/<client>-calisma-raporu-<range>.html)")
	reportCmd.Flags().String("client", "", "client name shown on the report")
	reportCmd.Flags().String("author", "", "who prepared the report (default: git user.name)")
	reportCmd.Flags().Bool("open", false, "open the report in your browser when done")
	rootCmd.AddCommand(reportCmd)
}
