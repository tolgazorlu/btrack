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
	db, err := sql.Open("sqlite", path+"?_journal=WAL&_timeout=5000&_fk=true")
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

func (s *SQLiteStore) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			task_name  TEXT    NOT NULL,
			start_time DATETIME NOT NULL,
			end_time   DATETIME,
			message    TEXT    DEFAULT '',
			tags       TEXT    DEFAULT '[]',
			git_branch TEXT    DEFAULT '',
			git_repo   TEXT    DEFAULT ''
		);
		CREATE TABLE IF NOT EXISTS log_entries (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id INTEGER NOT NULL,
			note       TEXT    NOT NULL,
			timestamp  DATETIME NOT NULL,
			parent_id  INTEGER REFERENCES log_entries(id) ON DELETE SET NULL,
			FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_sessions_end_time ON sessions(end_time);
		CREATE INDEX IF NOT EXISTS idx_log_entries_session ON log_entries(session_id);
	`)
	if err != nil {
		return err
	}
	// Add project column to existing databases (idempotent).
	rows, _ := s.db.Query(`PRAGMA table_info(sessions)`)
	hasProject := false
	if rows != nil {
		for rows.Next() {
			var cid int
			var name, typ string
			var notNull int
			var dflt sql.NullString
			var pk int
			_ = rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk)
			if name == "project" {
				hasProject = true
			}
		}
		rows.Close()
	}
	if !hasProject {
		_, err = s.db.Exec(`ALTER TABLE sessions ADD COLUMN project TEXT NOT NULL DEFAULT ''`)
		if err != nil {
			return err
		}
	}

	// Add parent_id to log_entries on existing databases that predate the initial schema fix (idempotent).
	logRows, _ := s.db.Query(`PRAGMA table_info(log_entries)`)
	hasParentID := false
	if logRows != nil {
		for logRows.Next() {
			var cid int
			var name, typ string
			var notNull int
			var dflt sql.NullString
			var pk int
			_ = logRows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk)
			if name == "parent_id" {
				hasParentID = true
			}
		}
		logRows.Close()
	}
	if !hasParentID {
		_, err = s.db.Exec(`ALTER TABLE log_entries ADD COLUMN parent_id INTEGER REFERENCES log_entries(id) ON DELETE SET NULL`)
	}
	return err
}

func (s *SQLiteStore) CreateSession(sess *Session) error {
	tagsJSON, _ := json.Marshal(sess.Tags)
	res, err := s.db.Exec(
		`INSERT INTO sessions (task_name, start_time, tags, git_branch, git_repo, project)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		sess.TaskName, sess.StartTime.UTC(), string(tagsJSON),
		sess.GitBranch, sess.GitRepo, sess.Project,
	)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	sess.ID, _ = res.LastInsertId()
	return nil
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
		`SELECT id, task_name, start_time, git_branch, git_repo, tags, project
		 FROM sessions WHERE end_time IS NULL ORDER BY start_time DESC LIMIT 1`,
	)
	return scanSession(row)
}

func (s *SQLiteStore) GetLastSession() (*Session, error) {
	row := s.db.QueryRow(
		`SELECT id, task_name, start_time, git_branch, git_repo, tags, project
		 FROM sessions ORDER BY start_time DESC LIMIT 1`,
	)
	return scanSession(row)
}

func (s *SQLiteStore) GetRecentSessions(limit int) ([]*Session, error) {
	rows, err := s.db.Query(
		`SELECT id, task_name, start_time, end_time, message, tags, git_branch, git_repo, project
		 FROM sessions ORDER BY start_time DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSessions(rows)
}

func (s *SQLiteStore) GetSessionByID(id int64) (*Session, error) {
	rows, err := s.db.Query(
		`SELECT id, task_name, start_time, end_time, message, tags, git_branch, git_repo, project
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

func (s *SQLiteStore) SearchSessions(query string) ([]*Session, error) {
	q := "%" + strings.ToLower(query) + "%"
	rows, err := s.db.Query(
		`SELECT id, task_name, start_time, end_time, message, tags, git_branch, git_repo, project
		 FROM sessions WHERE LOWER(task_name) LIKE ? OR LOWER(message) LIKE ?
		 ORDER BY start_time DESC LIMIT 200`, q, q,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSessions(rows)
}

func (s *SQLiteStore) GetSessionsForDate(date time.Time) ([]*Session, error) {
	y, m, d := date.Local().Date()
	from := time.Date(y, m, d, 0, 0, 0, 0, time.Local).UTC()
	to := from.Add(24 * time.Hour)
	rows, err := s.db.Query(
		`SELECT id, task_name, start_time, end_time, message, tags, git_branch, git_repo, project
		 FROM sessions WHERE start_time >= ? AND start_time < ?
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
		`SELECT DISTINCT project FROM sessions WHERE project != '' ORDER BY project ASC`,
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
		`SELECT id, task_name, start_time, end_time, message, tags, git_branch, git_repo, project
		 FROM sessions WHERE project=? ORDER BY start_time DESC LIMIT ?`, project, limit,
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

func scanSession(row *sql.Row) (*Session, error) {
	var sess Session
	var tagsJSON string
	var startStr string
	err := row.Scan(&sess.ID, &sess.TaskName, &startStr, &sess.GitBranch, &sess.GitRepo, &tagsJSON, &sess.Project)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sess.StartTime = parseTime(startStr)
	json.Unmarshal([]byte(tagsJSON), &sess.Tags)
	return &sess, nil
}

func scanSessions(rows *sql.Rows) ([]*Session, error) {
	var result []*Session
	for rows.Next() {
		var sess Session
		var tagsJSON, startStr string
		var endStr sql.NullString
		err := rows.Scan(&sess.ID, &sess.TaskName, &startStr, &endStr,
			&sess.Message, &tagsJSON, &sess.GitBranch, &sess.GitRepo, &sess.Project)
		if err != nil {
			return nil, err
		}
		sess.StartTime = parseTime(startStr)
		if endStr.Valid {
			t := parseTime(endStr.String)
			sess.EndTime = &t
		}
		json.Unmarshal([]byte(tagsJSON), &sess.Tags)
		result = append(result, &sess)
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
