package report

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// gitLogSep separates fields inside one log record; unlikely to occur in a subject.
const gitLogSep = "\x1f"

// gitRecSep separates records.
const gitRecSep = "\x1e"

var (
	prRefRe      = regexp.MustCompile(`(?i)(?:\(#|#|pull request #|pull/)(\d+)`)
	conventional = regexp.MustCompile(`^([a-z]+)(\([^)]*\))?!?:\s*(.+)$`)
	revertRe     = regexp.MustCompile(`(?i)^revert[\s:"]`)
)

// ScanGit reads the merged history of repoPath between from and to (inclusive)
// and returns one Commit per non-merge commit, oldest first.
func ScanGit(repoPath string, from, to time.Time) ([]Commit, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, fmt.Errorf("git not found in PATH")
	}
	// The record separator leads each record: git appends --name-only output
	// *after* the pretty format, so a trailing separator would push the file
	// list into the next record's first field.
	format := gitRecSep + strings.Join([]string{"%H", "%aI", "%s", "%b"}, gitLogSep)

	args := []string{
		"-C", repoPath, "log",
		"--no-merges",
		"--date=iso-strict",
		"--since=" + from.Format("2006-01-02T00:00:00"),
		"--until=" + to.AddDate(0, 0, 1).Format("2006-01-02T00:00:00"),
		"--pretty=format:" + format,
		"--name-only",
	}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("git log in %s: %w", repoPath, err)
	}

	var commits []Commit
	for _, rec := range strings.Split(string(out), gitRecSep) {
		rec = strings.TrimLeft(rec, "\n")
		if strings.TrimSpace(rec) == "" {
			continue
		}
		parts := strings.SplitN(rec, gitLogSep, 4)
		if len(parts) < 3 {
			continue
		}
		sha, dateStr, subject := parts[0], parts[1], parts[2]
		body, files := "", []string(nil)
		if len(parts) == 4 {
			body, files = splitBodyAndFiles(parts[3])
		}
		ts, err := time.Parse(time.RFC3339, dateStr)
		if err != nil {
			continue
		}
		kind, text := classify(subject)
		commits = append(commits, Commit{
			SHA:     sha,
			Date:    ts.Local(),
			Kind:    kind,
			Subject: text,
			PR:      findPR(subject, body),
			Areas:   areasFromFiles(files),
			Files:   files,
		})
	}

	// Squash workflows put the PR number in the subject; merge workflows put it
	// only on the merge commit, which --no-merges just dropped. Recover it by
	// mapping every merged commit back to the pull request that brought it in.
	if prs := mergedPRs(repoPath, from, to); len(prs) > 0 {
		for i := range commits {
			if commits[i].PR == 0 {
				if n, ok := prs[commits[i].SHA]; ok {
					commits[i].PR = n
				}
			}
		}
	}
	// git log is newest-first; the report reads oldest-first.
	for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
		commits[i], commits[j] = commits[j], commits[i]
	}
	return commits, nil
}

// splitBodyAndFiles separates the commit body from the --name-only file list.
// The body ends at the first blank line that is followed only by path-like lines.
func splitBodyAndFiles(rest string) (string, []string) {
	lines := strings.Split(rest, "\n")
	var body []string
	var files []string
	for _, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" {
			continue
		}
		// A path has no spaces and contains a separator or an extension.
		if !strings.Contains(trimmed, " ") && (strings.Contains(trimmed, "/") || filepath.Ext(trimmed) != "") {
			files = append(files, trimmed)
			continue
		}
		if len(files) == 0 {
			body = append(body, trimmed)
		}
	}
	return strings.Join(body, "\n"), files
}

// classify maps a conventional-commit subject onto a Kind plus the human text.
func classify(subject string) (Kind, string) {
	subject = strings.TrimSpace(subject)
	if revertRe.MatchString(subject) {
		return KindRevert, subject
	}
	m := conventional.FindStringSubmatch(subject)
	if m == nil {
		return KindChore, subject
	}
	text := m[3]
	switch m[1] {
	case "feat":
		return KindFeat, text
	case "fix", "perf", "hotfix":
		return KindFix, text
	case "revert":
		return KindRevert, text
	default:
		return KindChore, text
	}
}

// findPR pulls a pull-request number out of the subject or body.
func findPR(subject, body string) int {
	for _, s := range []string{subject, body} {
		if m := prRefRe.FindStringSubmatch(s); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil {
				return n
			}
		}
	}
	return 0
}

// areasFromFiles derives readable area names from changed paths, e.g.
// "app/(dashboard)/siparisler/page.tsx" → "siparisler".
func areasFromFiles(files []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range files {
		a := areaFromPath(f)
		if a == "" || seen[a] {
			continue
		}
		seen[a] = true
		out = append(out, a)
		if len(out) == 4 {
			break
		}
	}
	return out
}

// skipSegments are structural folders that say nothing about *what* changed.
var skipSegments = map[string]bool{
	"src": true, "app": true, "pages": true, "lib": true, "components": true,
	"panel": true, "dashboard": true, "admin": true, "sidebar": true,
	"": true, ".": true,
}

// areaFromPath names the part of the product a file belongs to, reading the
// path from the deepest directory outwards: the folder closest to the file is
// the most specific thing it can be named after. Next.js route groups —
// "(panel)", "(sidebar)" — are organisational, not functional, so they are
// unwrapped and then skipped like any other structural folder.
//
//	src/app/(panel)/panel/(sidebar)/siparisler/Edit.tsx  → "siparisler"
//	src/backend/siparis/queries.ts                       → "backend/siparis"
func areaFromPath(p string) string {
	segs := strings.Split(filepath.ToSlash(p), "/")
	if len(segs) == 1 {
		return strings.TrimSuffix(segs[0], filepath.Ext(segs[0]))
	}
	dirs := segs[:len(segs)-1]

	var meaningful []string
	for _, s := range dirs {
		s = strings.Trim(s, "()[]")
		if skipSegments[strings.ToLower(s)] || strings.HasPrefix(s, "_") {
			continue
		}
		meaningful = append(meaningful, s)
	}
	if len(meaningful) == 0 {
		// Everything was structural: fall back to the file's own name.
		return strings.TrimSuffix(segs[len(segs)-1], filepath.Ext(segs[len(segs)-1]))
	}
	// Keep at most the two deepest segments: enough to disambiguate
	// "siparisler/rezervasyonlar" without printing the whole tree.
	if len(meaningful) > 2 {
		meaningful = meaningful[len(meaningful)-2:]
	}
	return strings.Join(meaningful, "/")
}

// mergedPRs maps each commit SHA onto the pull request that merged it, by
// walking the merge commits in the window and listing each one's second-parent
// range. Repositories that squash-merge have no merge commits and yield an
// empty map, which is harmless.
func mergedPRs(repoPath string, from, to time.Time) map[string]int {
	out, err := exec.Command("git", "-C", repoPath, "log",
		"--merges",
		"--since="+from.Format("2006-01-02T00:00:00"),
		"--until="+to.AddDate(0, 0, 1).Format("2006-01-02T00:00:00"),
		"--pretty=format:%H"+gitLogSep+"%s",
	).Output()
	if err != nil {
		return nil
	}

	prs := map[string]int{}
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(line, gitLogSep, 2)
		if len(parts) != 2 {
			continue
		}
		mergeSHA, subject := parts[0], parts[1]
		num := findPR(subject, "")
		if num == 0 {
			continue
		}
		// Commits introduced by this merge: everything on the second parent
		// that the first parent did not already contain.
		listed, err := exec.Command("git", "-C", repoPath, "log",
			"--no-merges", "--pretty=format:%H",
			mergeSHA+"^1.."+mergeSHA+"^2").Output()
		if err != nil {
			continue
		}
		for _, sha := range strings.Fields(string(listed)) {
			// The earliest merge wins: a commit re-merged later still belongs
			// to the PR that first delivered it.
			if _, seen := prs[sha]; !seen {
				prs[sha] = num
			}
		}
	}
	return prs
}

// DiffStat returns the net files/insertions/deletions between the first commit
// in the window and HEAD.
func DiffStat(repoPath string, from, to time.Time) (files, added, removed int) {
	base, err := exec.Command("git", "-C", repoPath, "log",
		"--until="+from.Format("2006-01-02T00:00:00"),
		"-1", "--pretty=format:%H").Output()
	if err != nil || strings.TrimSpace(string(base)) == "" {
		return 0, 0, 0
	}
	out, err := exec.Command("git", "-C", repoPath, "diff", "--shortstat",
		strings.TrimSpace(string(base)), "HEAD").Output()
	if err != nil {
		return 0, 0, 0
	}
	line := string(out)
	files = firstInt(line, `(\d+) files? changed`)
	added = firstInt(line, `(\d+) insertions?`)
	removed = firstInt(line, `(\d+) deletions?`)
	return files, added, removed
}

func firstInt(s, pattern string) int {
	m := regexp.MustCompile(pattern).FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}
