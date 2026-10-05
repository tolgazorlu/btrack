// Hermes reader: parses Hermes's own SQLite session store (~/.hermes/state.db
// and ~/.hermes/profiles/*/state.db) into ingest.Session values.
package ingest

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// hermesUnixToTime converts a sessions/messages timestamp (unix epoch
// seconds, stored as REAL, fractional) into a time.Time.
func hermesUnixToTime(sec float64) time.Time {
	whole := int64(sec)
	frac := sec - float64(whole)
	return time.Unix(whole, int64(frac*float64(time.Second))).UTC()
}

// hermesInjectedPrefixes are content prefixes (after trimming whitespace)
// that mark a role='user' row as NOT a real typed prompt: it was injected
// by Hermes itself (async delivery notices, cron framing, compaction
// summaries, skill bodies, background-process notes, out-of-band steering
// messages, system notes). Anything else under role='user' is treated as a
// human prompt, including pasted multi-party chat transcripts that happen
// to start with a "[HH:MM, DD.MM.YYYY]" timestamp: those were typed/pasted
// by the human into the session, so they count.
var hermesInjectedPrefixes = []string{
	"[OUT-OF-BAND USER MESSAGE",
	"[ASYNC",
	"[System:",
	"[Note:",
	"[IMPORTANT:",
	"[CONTEXT COMPACTION",
	"[Cronjob",
	"[Your active task list",
}

// hermesSkipSources are Hermes's own source tags for sessions that are not
// a human-driven interactive session: cron jobs run unattended, and
// subagent is the generic child-session tag. Their rows are still read
// (as children, see below) to fold token usage into the parent, but they
// never become their own ingest.Session.
var hermesSkipSources = map[string]bool{
	"cron":     true,
	"subagent": true,
}

// HermesReader implements ingest.Reader for Hermes's own session store.
type HermesReader struct{}

func (HermesReader) Source() string { return "hermes" }

// Discover returns ~/.hermes/state.db (the default profile) plus
// ~/.hermes/profiles/*/state.db for every named profile. A missing
// .hermes or profiles directory is not an error.
func (HermesReader) Discover(home string) ([]string, error) {
	var paths []string

	root := filepath.Join(home, ".hermes", "state.db")
	if _, err := os.Stat(root); err == nil {
		paths = append(paths, root)
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	profilesDir := filepath.Join(home, ".hermes", "profiles")
	entries, err := os.ReadDir(profilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return paths, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(profilesDir, e.Name(), "state.db")
		if _, err := os.Stat(p); err == nil {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// hermesProfile derives the profile name from a discovered path:
// .../profiles/<name>/state.db -> <name>; anything else (the root
// ~/.hermes/state.db) -> "default".
func hermesProfile(path string) string {
	dir := filepath.Dir(path)
	if filepath.Base(filepath.Dir(dir)) == "profiles" {
		return filepath.Base(dir)
	}
	return "default"
}

// hermesIsInjected reports whether a role='user' message is Hermes-injected
// framing rather than something the human typed.
func hermesIsInjected(content string) bool {
	trimmed := strings.TrimSpace(content)
	for _, p := range hermesInjectedPrefixes {
		if strings.HasPrefix(trimmed, p) {
			return true
		}
	}
	return false
}

func hermesSingleLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	return s
}

func hermesTruncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

type hermesSessionRow struct {
	id               string
	parentSessionID  sql.NullString
	source           string
	startedAt        float64
	cwd              sql.NullString
	gitBranch        sql.NullString
	gitRepoRoot      sql.NullString
	model            sql.NullString
	title            sql.NullString
	inputTokens      sql.NullInt64
	outputTokens     sql.NullInt64
	cacheReadTokens  sql.NullInt64
	cacheWriteTokens sql.NullInt64
	estimatedCostUSD sql.NullFloat64
	actualCostUSD    sql.NullFloat64
}

// Read parses one Hermes state.db. It opens the database read-only
// (mode=ro) and never writes: the Hermes app may have it open.
func (r HermesReader) Read(path string) ([]Session, error) {
	dsn := fmt.Sprintf("file:%s?mode=ro&_pragma=busy_timeout(3000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	profile := hermesProfile(path)

	rows, err := db.Query(`
		SELECT id, parent_session_id, source, started_at, cwd, git_branch,
		       git_repo_root, model, title, input_tokens, output_tokens,
		       cache_read_tokens, cache_write_tokens, estimated_cost_usd,
		       actual_cost_usd
		FROM sessions
		WHERE parent_session_id IS NULL
		   -- A compression continuation is the same conversation carried
		   -- into a fresh session after its parent ended with
		   -- end_reason='compression': its time is real, not a subagent's.
		   OR parent_session_id IN (SELECT id FROM sessions WHERE end_reason = 'compression')
	`)
	if err != nil {
		return nil, err
	}
	var parents []hermesSessionRow
	for rows.Next() {
		var s hermesSessionRow
		if err := rows.Scan(&s.id, &s.parentSessionID, &s.source, &s.startedAt,
			&s.cwd, &s.gitBranch, &s.gitRepoRoot, &s.model, &s.title,
			&s.inputTokens, &s.outputTokens, &s.cacheReadTokens,
			&s.cacheWriteTokens, &s.estimatedCostUSD, &s.actualCostUSD); err != nil {
			// A malformed row should not sink the whole file.
			continue
		}
		if hermesSkipSources[s.source] {
			continue
		}
		parents = append(parents, s)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	var out []Session
	for _, s := range parents {
		sess, err := r.readOne(db, profile, s)
		if err != nil {
			// Skip this session, keep going for the rest of the file.
			continue
		}
		if sess == nil {
			continue
		}
		out = append(out, *sess)
	}
	return out, nil
}

func (r HermesReader) readOne(db *sql.DB, profile string, s hermesSessionRow) (*Session, error) {
	msgRows, err := db.Query(`
		SELECT role, content, tool_calls, timestamp
		FROM messages
		WHERE session_id = ?
		ORDER BY timestamp ASC, id ASC
	`, s.id)
	if err != nil {
		return nil, err
	}
	defer msgRows.Close()

	var events []Event
	var hints []string
	var firstHuman string
	for msgRows.Next() {
		var role string
		var content, toolCalls sql.NullString
		var ts sql.NullFloat64
		if err := msgRows.Scan(&role, &content, &toolCalls, &ts); err != nil {
			continue
		}
		if !ts.Valid {
			// No real timestamp to anchor this event: skip it.
			continue
		}
		at := hermesUnixToTime(ts.Float64)
		switch role {
		case "user":
			if !content.Valid || strings.TrimSpace(content.String) == "" {
				continue
			}
			if hermesIsInjected(content.String) {
				continue
			}
			if firstHuman == "" {
				firstHuman = content.String
			}
			events = append(events, Event{At: at, Kind: Human})
		case "assistant", "tool":
			events = append(events, Event{At: at, Kind: Agent})
			// Tool arguments and tool results (which report the working
			// directory) say which repo the agent was actually in.
			hints = collectPathHints(hints, toolCalls.String)
			if role == "tool" {
				hints = collectPathHints(hints, content.String)
			}
		default:
			// Unknown role (future-proofing): treat conservatively as
			// agent activity rather than drop the timestamp entirely.
			events = append(events, Event{At: at, Kind: Agent})
		}
	}
	if err := msgRows.Err(); err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, nil
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].At.Before(events[j].At) })

	calls, err := r.collectCalls(db, s)
	if err != nil {
		return nil, err
	}

	title := ""
	if s.title.Valid && strings.TrimSpace(s.title.String) != "" {
		title = strings.TrimSpace(s.title.String)
	} else {
		title = hermesSingleLine(firstHuman)
	}
	title = hermesTruncateRunes(title, 80)

	cwd := ""
	if s.gitRepoRoot.Valid && s.gitRepoRoot.String != "" {
		cwd = s.gitRepoRoot.String
	} else if s.cwd.Valid {
		cwd = s.cwd.String
	}

	var costUSD *float64
	if s.actualCostUSD.Valid {
		v := s.actualCostUSD.Float64
		costUSD = &v
	} else if s.estimatedCostUSD.Valid {
		v := s.estimatedCostUSD.Float64
		costUSD = &v
	}

	branch := ""
	if s.gitBranch.Valid {
		branch = s.gitBranch.String
	}

	return &Session{
		Source:     "hermes",
		ExternalID: profile + "/" + s.id,
		Title:      title,
		Cwd:        cwd,
		GitBranch:  branch,
		PathHints:  hints,
		Events:     events,
		Calls:      calls,
		CostUSD:    costUSD,
	}, nil
}

// collectCalls builds the session's ModelCall list. Usage is tracked per
// model in session_model_usage (already a cumulative total per
// session+model, so no per-record dedupe is needed: we read it once and
// sum by model). Child sessions (subagents launched from this one, found
// via parent_session_id) are cheap to attribute here via a SQL join, so
// their usage is folded into the same per-model totals rather than
// dropped. If a session has no session_model_usage rows at all (older
// data), we fall back to the session's own cumulative token columns as a
// single ModelCall.
func (r HermesReader) collectCalls(db *sql.DB, s hermesSessionRow) ([]ModelCall, error) {
	rows, err := db.Query(`
		SELECT model, SUM(input_tokens), SUM(output_tokens),
		       SUM(cache_read_tokens), SUM(cache_write_tokens)
		FROM session_model_usage
		WHERE session_id = ?
		   OR session_id IN (SELECT c.id FROM sessions c JOIN sessions p ON p.id = c.parent_session_id
		                     WHERE c.parent_session_id = ? AND COALESCE(p.end_reason, '') <> 'compression')
		GROUP BY model
		HAVING model <> ''
		ORDER BY model
	`, s.id, s.id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var calls []ModelCall
	for rows.Next() {
		var c ModelCall
		if err := rows.Scan(&c.Model, &c.Input, &c.Output, &c.CacheRead, &c.CacheWrite); err != nil {
			continue
		}
		calls = append(calls, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(calls) > 0 {
		return calls, nil
	}

	// Fallback: session-level cumulative totals as one ModelCall.
	if !s.model.Valid || s.model.String == "" {
		return nil, nil
	}
	c := ModelCall{Model: s.model.String}
	if s.inputTokens.Valid {
		c.Input = s.inputTokens.Int64
	}
	if s.outputTokens.Valid {
		c.Output = s.outputTokens.Int64
	}
	if s.cacheReadTokens.Valid {
		c.CacheRead = s.cacheReadTokens.Int64
	}
	if s.cacheWriteTokens.Valid {
		c.CacheWrite = s.cacheWriteTokens.Int64
	}
	if c.Input == 0 && c.Output == 0 && c.CacheRead == 0 && c.CacheWrite == 0 {
		return nil, nil
	}
	return []ModelCall{c}, nil
}
