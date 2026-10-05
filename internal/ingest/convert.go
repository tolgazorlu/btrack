package ingest

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/db"
	"github.com/tolgazorlu/btrack/internal/pricing"
)

// Readers returns every built-in source reader.
func Readers() []Reader {
	return []Reader{ClaudeCodeReader{}, CodexReader{}, HermesReader{}}
}

// ProjectResolver maps a working directory to a btrack project name:
//
//  1. a .btrack file in the directory or a parent wins;
//  2. otherwise the git repository's name: the origin remote's repo name,
//     else the main checkout's directory name (so worktrees resolve to
//     their repository, not to a throwaway directory name);
//  3. otherwise the directory's own name, if it still exists.
//
// The home directory, empty paths and directories that no longer exist
// outside any repo resolve to "" (unassigned): a session started there
// could be about anything. ResolveSession then falls back to the paths
// the agent touched.
type ProjectResolver struct {
	Home  string
	cache map[string]string
}

func (r *ProjectResolver) Resolve(cwd string) string {
	if cwd == "" {
		return ""
	}
	cwd = filepath.Clean(cwd)
	if cwd == filepath.Clean(r.Home) {
		return ""
	}
	if r.cache == nil {
		r.cache = map[string]string{}
	}
	if p, ok := r.cache[cwd]; ok {
		return p
	}
	p := r.resolve(cwd)
	r.cache[cwd] = p
	return p
}

// ResolveSession resolves the session's own cwd, falling back to the
// project most of its path hints point into. A hinted project needs at
// least 3 hits and 40% of all resolvable hints, so one stray file read in
// another repo does not claim the session.
func (r *ProjectResolver) ResolveSession(s Session) string {
	if p := r.Resolve(s.Cwd); p != "" {
		return p
	}
	counts := map[string]int{}
	total := 0
	for _, h := range s.PathHints {
		p := r.Resolve(r.existingRepoDir(h))
		if p == "" {
			continue
		}
		counts[p]++
		total++
	}
	best, bestN := "", 0
	for p, n := range counts {
		if n > bestN || (n == bestN && p < best) {
			best, bestN = p, n
		}
	}
	if bestN >= 3 && bestN*10 >= total*4 {
		return best
	}
	return ""
}

// existingRepoDir returns the git checkout containing path, or "" when
// path is not inside one. Hints only count when they land in a repo, so
// paths into tool caches or config directories are ignored.
func (r *ProjectResolver) existingRepoDir(path string) string {
	home := filepath.Clean(r.Home)
	for dir := filepath.Clean(path); dir != home && dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
	}
	return ""
}

func (r *ProjectResolver) resolve(cwd string) string {
	if pf, err := config.FindProjectFile(cwd); err == nil && pf != nil && pf.Project != "" {
		return pf.Project
	}
	if repo := r.existingRepoDir(cwd); repo != "" {
		return repoName(repo)
	}
	// Outside any repo. Throwaway workspaces that agent apps create for
	// themselves are not projects. Anything else is named after itself,
	// even if it has since been moved or deleted.
	if rel, err := filepath.Rel(filepath.Clean(r.Home), cwd); err == nil && isThrowaway(filepath.ToSlash(rel)) {
		return ""
	}
	return filepath.Base(cwd)
}

// throwawayRoots are home-relative directories where agent apps create
// per-chat scratch workspaces.
var throwawayRoots = []string{
	"Library/",         // macOS app support, e.g. Claude desktop scratch-workspaces
	"AppData/",         // Windows equivalent
	"Documents/Codex/", // Codex desktop per-chat folders
}

func isThrowaway(rel string) bool {
	if strings.HasPrefix(rel, ".") { // ~/.cache, ~/.codex, ~/.hermes/cache/scratch ...
		return true
	}
	for _, root := range throwawayRoots {
		if strings.HasPrefix(rel, root) {
			return true
		}
	}
	return false
}

// repoName names the repository checked out at dir: the origin remote's
// repo name when there is one, else the main checkout's directory name.
func repoName(dir string) string {
	gitDir := filepath.Join(dir, ".git")
	mainDir := dir
	if fi, err := os.Stat(gitDir); err == nil && !fi.IsDir() {
		// A linked worktree: .git is a file "gitdir: <main>/.git/worktrees/<name>".
		if b, err := os.ReadFile(gitDir); err == nil {
			line := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(b)), "gitdir:"))
			if i := strings.Index(line, string(filepath.Separator)+"worktrees"+string(filepath.Separator)); i > 0 {
				gitDir = line[:i]
				mainDir = filepath.Dir(gitDir)
				if !strings.HasSuffix(gitDir, string(filepath.Separator)+".git") {
					mainDir = strings.TrimSuffix(gitDir, ".git") // bare repo
				}
			}
		}
	}
	if name := originRepoName(filepath.Join(gitDir, "config")); name != "" {
		return name
	}
	return filepath.Base(mainDir)
}

// originRepoName reads the origin URL from a git config file and returns
// its last path segment without ".git".
func originRepoName(configPath string) string {
	b, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}
	inOrigin := false
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			inOrigin = line == `[remote "origin"]`
			continue
		}
		if !inOrigin || !strings.HasPrefix(line, "url") {
			continue
		}
		if i := strings.Index(line, "="); i >= 0 {
			url := strings.TrimSuffix(strings.TrimSpace(line[i+1:]), "/")
			url = strings.TrimSuffix(url, ".git")
			if j := strings.LastIndexAny(url, "/:"); j >= 0 {
				url = url[j+1:]
			}
			return url
		}
	}
	return ""
}

// Converted is a parsed session ready to store, plus what the caller needs
// to report about it.
type Converted struct {
	Session      *db.Session
	Split        Split
	UnknownModel string // first model the pricing table could not price
}

// ToDB maps a parsed session to a db.Session: time split, cost and
// project. Sessions with no human prompt return nil (nothing a person did).
func ToDB(s Session, resolver *ProjectResolver, opt SplitOptions) *Converted {
	if len(s.Events) == 0 || HumanTurns(s.Events) == 0 {
		return nil
	}
	sort.SliceStable(s.Events, func(i, j int) bool { return s.Events[i].At.Before(s.Events[j].At) })
	sp := ComputeSplit(s.Events, opt)
	// Wall time includes the first prompt's reading/typing allowance, so
	// the stored session starts that much before the first timestamp.
	end := s.End()
	start := end.Add(-sp.Wall)
	active := int64(sp.Active.Seconds())
	agent := int64(sp.Agent.Seconds())

	out := &Converted{Split: sp}
	sess := &db.Session{
		TaskName:      title(s),
		StartTime:     start,
		EndTime:       &end,
		GitBranch:     s.GitBranch,
		Project:       resolver.ResolveSession(s),
		Source:        s.Source,
		ExternalID:    s.ExternalID,
		Cwd:           s.Cwd,
		ActiveSeconds: &active,
		AgentSeconds:  &agent,
	}

	var cost float64
	priced := true
	models := map[string]int64{}
	for _, c := range s.Calls {
		sess.Usage.Input += c.Input
		sess.Usage.Output += c.Output
		sess.Usage.CacheRead += c.CacheRead
		sess.Usage.CacheWrite += c.CacheWrite
		models[c.Model] += c.Output + c.Input
		usd, known := pricing.Estimate(c.Model, pricing.Tokens{
			Input: c.Input, Output: c.Output, CacheRead: c.CacheRead, CacheWrite: c.CacheWrite,
		})
		if !known {
			priced = false
			if out.UnknownModel == "" {
				out.UnknownModel = c.Model
			}
			continue
		}
		cost += usd
	}
	sess.Model = dominantModel(models)
	switch {
	case s.CostUSD != nil:
		sess.CostUSD = s.CostUSD
	case len(s.Calls) > 0 && priced:
		sess.CostUSD = &cost
	}
	out.Session = sess
	return out
}

func dominantModel(m map[string]int64) string {
	best, bestN := "", int64(-1)
	for k, v := range m {
		if k != "" && (v > bestN || (v == bestN && k < best)) {
			best, bestN = k, v
		}
	}
	return best
}

func title(s Session) string {
	t := strings.Join(strings.Fields(s.Title), " ")
	if t == "" {
		t = "(untitled " + s.Source + " session)"
	}
	if utf8.RuneCountInString(t) > 80 {
		r := []rune(t)
		t = string(r[:79]) + "…"
	}
	return t
}
