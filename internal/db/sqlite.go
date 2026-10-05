package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	_ = os.Chmod(path, 0600)
	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

// CreateSession inserts a session. A session with an EndTime already set is a
// completed record being backfilled (see `btrack import`), so every field is
// written; a live session leaves end_time NULL and fills it in on stop.
func (s *SQLiteStore) CreateSession(sess *Session) error {
	tagsJSON, _ := json.Marshal(sess.Tags)
	res, err := s.db.Exec(
		`INSERT INTO sessions (task_name, start_time, end_time, message, tags, git_branch, git_repo, project,
			source, external_id, cwd, active_seconds, agent_seconds, model,
			input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, cost_usd)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sess.TaskName, sess.StartTime.UTC(), nullTime(sess.EndTime), sess.Message,
		string(tagsJSON), sess.GitBranch, sess.GitRepo, sess.Project,
		sess.source(), nullString(sess.ExternalID), sess.Cwd, sess.ActiveSeconds, sess.AgentSeconds, sess.Model,
		sess.Usage.Input, sess.Usage.Output, sess.Usage.CacheRead, sess.Usage.CacheWrite, sess.CostUSD,
	)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	sess.ID, _ = res.LastInsertId()
	return nil
}

// UpsertExternalSession records a session imported from another tool, keyed
// by (Source, ExternalID). Re-syncing the same session refreshes what the
// tool may have grown since (end time, active/agent time, usage, cost) but
// never overwrites task name, project or message, which the user may have
// edited in btrack. Reports whether a new row was created.
func (s *SQLiteStore) UpsertExternalSession(sess *Session) (bool, error) {
	if sess.ExternalID == "" || sess.source() == SourceManual {
		return false, fmt.Errorf("upsert external session: source and external id are required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRow(`SELECT id FROM sessions WHERE source=? AND external_id=?`, sess.source(), sess.ExternalID).Scan(&id)
	switch {
	case err == sql.ErrNoRows:
		tagsJSON, _ := json.Marshal(sess.Tags)
		res, err := tx.Exec(
			`INSERT INTO sessions (task_name, start_time, end_time, message, tags, git_branch, git_repo, project,
				source, external_id, cwd, active_seconds, agent_seconds, model,
				input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, cost_usd)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			sess.TaskName, sess.StartTime.UTC(), nullTime(sess.EndTime), sess.Message,
			string(tagsJSON), sess.GitBranch, sess.GitRepo, sess.Project,
			sess.source(), sess.ExternalID, sess.Cwd, sess.ActiveSeconds, sess.AgentSeconds, sess.Model,
			sess.Usage.Input, sess.Usage.Output, sess.Usage.CacheRead, sess.Usage.CacheWrite, sess.CostUSD,
		)
		if err != nil {
			return false, fmt.Errorf("insert external session: %w", err)
		}
		sess.ID, _ = res.LastInsertId()
		return true, tx.Commit()
	case err != nil:
		return false, err
	}
	sess.ID = id
	_, err = tx.Exec(
		`UPDATE sessions SET start_time=?, end_time=?, cwd=?, active_seconds=?, agent_seconds=?, model=?,
			input_tokens=?, output_tokens=?, cache_read_tokens=?, cache_write_tokens=?, cost_usd=?
		 WHERE id=?`,
		sess.StartTime.UTC(), nullTime(sess.EndTime), sess.Cwd, sess.ActiveSeconds, sess.AgentSeconds, sess.Model,
		sess.Usage.Input, sess.Usage.Output, sess.Usage.CacheRead, sess.Usage.CacheWrite, sess.CostUSD, id,
	)
	if err != nil {
		return false, fmt.Errorf("update external session: %w", err)
	}
	return false, tx.Commit()
}

// GetSessionsInRange returns sessions whose start falls in [from, to).
func (s *SQLiteStore) GetSessionsInRange(from, to time.Time) ([]*Session, error) {
	rows, err := s.db.Query(
		`SELECT `+sessionCols+`
		 FROM sessions WHERE start_time >= ? AND start_time < ?
		 ORDER BY start_time ASC`, from.UTC(), to.UTC(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSessions(rows)
}

func (s *SQLiteStore) UpdateSession(sess *Session) error {
	tagsJSON, _ := json.Marshal(sess.Tags)
	_, err := s.db.Exec(
		`UPDATE sessions SET task_name=?, start_time=?, end_time=?, message=?, tags=?, project=? WHERE id=?`,
		sess.TaskName, sess.StartTime.UTC(), nullTime(sess.EndTime), sess.Message, string(tagsJSON), sess.Project, sess.ID,
	)
	return err
}

func (s *SQLiteStore) GetActiveSession() (*Session, error) {
	row := s.db.QueryRow(
		`SELECT ` + sessionCols + `
		 FROM sessions WHERE end_time IS NULL AND source='manual' ORDER BY start_time DESC LIMIT 1`,
	)
	return scanSession(row)
}

func (s *SQLiteStore) GetLastSession() (*Session, error) {
	row := s.db.QueryRow(
		`SELECT ` + sessionCols + `
		 FROM sessions WHERE source='manual' ORDER BY start_time DESC LIMIT 1`,
	)
	return scanSession(row)
}

func (s *SQLiteStore) GetRecentSessions(limit int) ([]*Session, error) {
	rows, err := s.db.Query(
		`SELECT `+sessionCols+`
		 FROM sessions WHERE source='manual' ORDER BY start_time DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSessions(rows)
}

func (s *SQLiteStore) GetSessionByID(id int64) (*Session, error) {
	rows, err := s.db.Query(
		`SELECT `+sessionCols+`
		 FROM sessions WHERE id=? LIMIT 1`, id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sessions, err := scanSessions(rows)
	if err != nil || len(sessions) == 0 {
		return nil, err
	}
	return sessions[0], nil
}

func (s *SQLiteStore) GetSessionsForDate(date time.Time) ([]*Session, error) {
	y, m, d := date.Local().Date()
	from := time.Date(y, m, d, 0, 0, 0, 0, time.Local).UTC()
	to := from.Add(24 * time.Hour)
	rows, err := s.db.Query(
		`SELECT `+sessionCols+`
		 FROM sessions WHERE start_time >= ? AND start_time < ? AND source='manual'
		 ORDER BY start_time ASC`, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSessions(rows)
}

func (s *SQLiteStore) GetProjects() ([]string, error) {
	rows, err := s.db.Query(
		`SELECT DISTINCT project FROM sessions WHERE project != '' AND source='manual' ORDER BY project ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var projects []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (s *SQLiteStore) GetSessionsByProject(project string, limit int) ([]*Session, error) {
	rows, err := s.db.Query(
		`SELECT `+sessionCols+`
		 FROM sessions WHERE project=? AND source='manual' ORDER BY start_time DESC LIMIT ?`, project, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSessions(rows)
}

func (s *SQLiteStore) CreateLogEntry(e *LogEntry) error {
	var parentID interface{}
	if e.ParentID != nil {
		parentID = *e.ParentID
	}
	res, err := s.db.Exec(
		`INSERT INTO log_entries (session_id, note, timestamp, parent_id) VALUES (?, ?, ?, ?)`,
		e.SessionID, e.Note, e.Timestamp.UTC(), parentID,
	)
	if err != nil {
		return fmt.Errorf("create log entry: %w", err)
	}
	e.ID, _ = res.LastInsertId()
	return nil
}

func (s *SQLiteStore) GetRecentLogs(sessionID int64, limit int) ([]*LogEntry, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, note, timestamp, parent_id FROM log_entries
		 WHERE session_id=? ORDER BY timestamp DESC LIMIT ?`, sessionID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLogs(rows)
}

func (s *SQLiteStore) GetAllLogs(sessionID int64) ([]*LogEntry, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, note, timestamp, parent_id FROM log_entries
		 WHERE session_id=? ORDER BY timestamp ASC`, sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLogs(rows)
}

func (s *SQLiteStore) Close() error { return s.db.Close() }

const sessionCols = `id, task_name, start_time, end_time, message, tags, git_branch, git_repo, project,
	source, external_id, cwd, active_seconds, agent_seconds, model,
	input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, cost_usd`

type scanner interface{ Scan(dest ...any) error }

func scanOne(sc scanner) (*Session, error) {
	var (
		sess               Session
		tagsJSON, startStr string
		endStr, externalID sql.NullString
		active, agent      sql.NullInt64
		cost               sql.NullFloat64
	)
	err := sc.Scan(&sess.ID, &sess.TaskName, &startStr, &endStr, &sess.Message, &tagsJSON,
		&sess.GitBranch, &sess.GitRepo, &sess.Project,
		&sess.Source, &externalID, &sess.Cwd, &active, &agent, &sess.Model,
		&sess.Usage.Input, &sess.Usage.Output, &sess.Usage.CacheRead, &sess.Usage.CacheWrite, &cost)
	if err != nil {
		return nil, err
	}
	sess.StartTime = parseTime(startStr)
	if endStr.Valid {
		t := parseTime(endStr.String)
		sess.EndTime = &t
	}
	sess.ExternalID = externalID.String
	if active.Valid {
		sess.ActiveSeconds = &active.Int64
	}
	if agent.Valid {
		sess.AgentSeconds = &agent.Int64
	}
	if cost.Valid {
		sess.CostUSD = &cost.Float64
	}
	_ = json.Unmarshal([]byte(tagsJSON), &sess.Tags)
	return &sess, nil
}

func scanSession(row *sql.Row) (*Session, error) {
	sess, err := scanOne(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return sess, err
}

func scanSessions(rows *sql.Rows) ([]*Session, error) {
	var result []*Session
	for rows.Next() {
		sess, err := scanOne(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, sess)
	}
	return result, rows.Err()
}

func scanLogs(rows *sql.Rows) ([]*LogEntry, error) {
	var result []*LogEntry
	for rows.Next() {
		var e LogEntry
		var tsStr string
		var parentID sql.NullInt64
		if err := rows.Scan(&e.ID, &e.SessionID, &e.Note, &tsStr, &parentID); err != nil {
			return nil, err
		}
		e.Timestamp = parseTime(tsStr)
		if parentID.Valid {
			v := parentID.Int64
			e.ParentID = &v
		}
		result = append(result, &e)
	}
	return result, rows.Err()
}

func parseTime(s string) time.Time {
	s = strings.Replace(s, " ", "T", 1)
	if !strings.HasSuffix(s, "Z") && !strings.Contains(s, "+") {
		s += "Z"
	}
	t, _ := time.Parse(time.RFC3339, s)
	return t.Local()
}

func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC()
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
