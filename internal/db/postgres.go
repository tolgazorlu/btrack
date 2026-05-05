package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// PostgresStore mirrors SQLiteStore but targets PostgreSQL.
// Use when database.type = "postgres" in config.
type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	s := &PostgresStore{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *PostgresStore) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			id         BIGSERIAL PRIMARY KEY,
			task_name  TEXT        NOT NULL,
			start_time TIMESTAMPTZ NOT NULL,
			end_time   TIMESTAMPTZ,
			message    TEXT        DEFAULT '',
			tags       JSONB       DEFAULT '[]',
			git_branch TEXT        DEFAULT '',
			git_repo   TEXT        DEFAULT ''
		);
		CREATE TABLE IF NOT EXISTS log_entries (
			id         BIGSERIAL PRIMARY KEY,
			session_id BIGINT      NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			note       TEXT        NOT NULL,
			timestamp  TIMESTAMPTZ NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_sessions_end_time  ON sessions(end_time);
		CREATE INDEX IF NOT EXISTS idx_log_session        ON log_entries(session_id);
	`)
	if err != nil {
		return err
	}
	// Add project column to existing databases (idempotent).
	_, err = s.db.Exec(`
		DO $$ BEGIN
			ALTER TABLE sessions ADD COLUMN project TEXT NOT NULL DEFAULT '';
		EXCEPTION WHEN duplicate_column THEN NULL;
		END $$;
	`)
	if err != nil {
		return err
	}
	// Add parent_id column to log_entries (idempotent).
	_, err = s.db.Exec(`
		DO $$ BEGIN
			ALTER TABLE log_entries ADD COLUMN parent_id BIGINT REFERENCES log_entries(id) ON DELETE SET NULL;
		EXCEPTION WHEN duplicate_column THEN NULL;
		END $$;
	`)
	return err
}

func (s *PostgresStore) CreateSession(sess *Session) error {
	tagsJSON, _ := json.Marshal(sess.Tags)
	return s.db.QueryRow(
		`INSERT INTO sessions (task_name, start_time, tags, git_branch, git_repo, project)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		sess.TaskName, sess.StartTime, string(tagsJSON), sess.GitBranch, sess.GitRepo, sess.Project,
	).Scan(&sess.ID)
}

func (s *PostgresStore) UpdateSession(sess *Session) error {
	tagsJSON, _ := json.Marshal(sess.Tags)
	_, err := s.db.Exec(
		`UPDATE sessions SET end_time=$1, message=$2, tags=$3 WHERE id=$4`,
		nullTime(sess.EndTime), sess.Message, string(tagsJSON), sess.ID,
	)
	return err
}

func (s *PostgresStore) GetActiveSession() (*Session, error) {
	row := s.db.QueryRow(
		`SELECT id, task_name, start_time, git_branch, git_repo, tags, project
		 FROM sessions WHERE end_time IS NULL ORDER BY start_time DESC LIMIT 1`,
	)
	return scanSessionPG(row)
}

func (s *PostgresStore) GetLastSession() (*Session, error) {
	row := s.db.QueryRow(
		`SELECT id, task_name, start_time, git_branch, git_repo, tags, project
		 FROM sessions ORDER BY start_time DESC LIMIT 1`,
	)
	return scanSessionPG(row)
}

func (s *PostgresStore) GetRecentSessions(limit int) ([]*Session, error) {
	rows, err := s.db.Query(
		`SELECT id, task_name, start_time, end_time, message, tags, git_branch, git_repo, project
		 FROM sessions ORDER BY start_time DESC LIMIT $1`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSessionsPG(rows)
}

func (s *PostgresStore) GetSessionByID(id int64) (*Session, error) {
	rows, err := s.db.Query(
		`SELECT id, task_name, start_time, end_time, message, tags, git_branch, git_repo, project
		 FROM sessions WHERE id=$1 LIMIT 1`, id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sessions, err := scanSessionsPG(rows)
	if err != nil || len(sessions) == 0 {
		return nil, err
	}
	return sessions[0], nil
}

func (s *PostgresStore) SearchSessions(query string) ([]*Session, error) {
	q := "%" + strings.ToLower(query) + "%"
	rows, err := s.db.Query(
		`SELECT id, task_name, start_time, end_time, message, tags, git_branch, git_repo, project
		 FROM sessions WHERE LOWER(task_name) LIKE $1 OR LOWER(message) LIKE $1
		 ORDER BY start_time DESC LIMIT 200`, q,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSessionsPG(rows)
}

func (s *PostgresStore) GetSessionsForDate(date time.Time) ([]*Session, error) {
	y, m, d := date.Local().Date()
	from := time.Date(y, m, d, 0, 0, 0, 0, time.Local).UTC()
	to := from.Add(24 * time.Hour)
	rows, err := s.db.Query(
		`SELECT id, task_name, start_time, end_time, message, tags, git_branch, git_repo, project
		 FROM sessions WHERE start_time >= $1 AND start_time < $2
		 ORDER BY start_time ASC`, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSessionsPG(rows)
}

func (s *PostgresStore) GetProjects() ([]string, error) {
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

func (s *PostgresStore) GetSessionsByProject(project string, limit int) ([]*Session, error) {
	rows, err := s.db.Query(
		`SELECT id, task_name, start_time, end_time, message, tags, git_branch, git_repo, project
		 FROM sessions WHERE project=$1 ORDER BY start_time DESC LIMIT $2`, project, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSessionsPG(rows)
}

func (s *PostgresStore) CreateLogEntry(e *LogEntry) error {
	var parentID interface{}
	if e.ParentID != nil {
		parentID = *e.ParentID
	}
	return s.db.QueryRow(
		`INSERT INTO log_entries (session_id, note, timestamp, parent_id) VALUES ($1, $2, $3, $4) RETURNING id`,
		e.SessionID, e.Note, e.Timestamp, parentID,
	).Scan(&e.ID)
}

func (s *PostgresStore) GetRecentLogs(sessionID int64, limit int) ([]*LogEntry, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, note, timestamp, parent_id FROM log_entries
		 WHERE session_id=$1 ORDER BY timestamp DESC LIMIT $2`, sessionID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLogsPG(rows)
}

func (s *PostgresStore) GetAllLogs(sessionID int64) ([]*LogEntry, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, note, timestamp, parent_id FROM log_entries
		 WHERE session_id=$1 ORDER BY timestamp ASC`, sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLogsPG(rows)
}

func (s *PostgresStore) Close() error { return s.db.Close() }

func scanSessionPG(row *sql.Row) (*Session, error) {
	var sess Session
	var tagsJSON []byte
	err := row.Scan(&sess.ID, &sess.TaskName, &sess.StartTime, &sess.GitBranch, &sess.GitRepo, &tagsJSON, &sess.Project)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	json.Unmarshal(tagsJSON, &sess.Tags)
	return &sess, nil
}

func scanSessionsPG(rows *sql.Rows) ([]*Session, error) {
	var result []*Session
	for rows.Next() {
		var sess Session
		var tagsJSON []byte
		var endTime sql.NullTime
		err := rows.Scan(&sess.ID, &sess.TaskName, &sess.StartTime, &endTime,
			&sess.Message, &tagsJSON, &sess.GitBranch, &sess.GitRepo, &sess.Project)
		if err != nil {
			return nil, err
		}
		if endTime.Valid {
			sess.EndTime = &endTime.Time
		}
		json.Unmarshal(tagsJSON, &sess.Tags)
		result = append(result, &sess)
	}
	return result, rows.Err()
}

func scanLogsPG(rows *sql.Rows) ([]*LogEntry, error) {
	var result []*LogEntry
	for rows.Next() {
		var e LogEntry
		var parentID sql.NullInt64
		if err := rows.Scan(&e.ID, &e.SessionID, &e.Note, &e.Timestamp, &parentID); err != nil {
			return nil, err
		}
		if parentID.Valid {
			v := parentID.Int64
			e.ParentID = &v
		}
		result = append(result, &e)
	}
	return result, rows.Err()
}
