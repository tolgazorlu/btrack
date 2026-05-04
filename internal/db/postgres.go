package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
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
	return err
}

func (s *PostgresStore) CreateSession(sess *Session) error {
	tagsJSON, _ := json.Marshal(sess.Tags)
	return s.db.QueryRow(
		`INSERT INTO sessions (task_name, start_time, tags, git_branch, git_repo)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		sess.TaskName, sess.StartTime, string(tagsJSON), sess.GitBranch, sess.GitRepo,
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
		`SELECT id, task_name, start_time, git_branch, git_repo, tags
		 FROM sessions WHERE end_time IS NULL ORDER BY start_time DESC LIMIT 1`,
	)
	return scanSessionPG(row)
}

func (s *PostgresStore) GetLastSession() (*Session, error) {
	row := s.db.QueryRow(
		`SELECT id, task_name, start_time, git_branch, git_repo, tags
		 FROM sessions ORDER BY start_time DESC LIMIT 1`,
	)
	return scanSessionPG(row)
}

func (s *PostgresStore) GetRecentSessions(limit int) ([]*Session, error) {
	rows, err := s.db.Query(
		`SELECT id, task_name, start_time, end_time, message, tags, git_branch, git_repo
		 FROM sessions ORDER BY start_time DESC LIMIT $1`, limit,
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
		`SELECT id, task_name, start_time, end_time, message, tags, git_branch, git_repo
		 FROM sessions WHERE start_time >= $1 AND start_time < $2
		 ORDER BY start_time ASC`, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSessionsPG(rows)
}

func (s *PostgresStore) CreateLogEntry(e *LogEntry) error {
	return s.db.QueryRow(
		`INSERT INTO log_entries (session_id, note, timestamp) VALUES ($1, $2, $3) RETURNING id`,
		e.SessionID, e.Note, e.Timestamp,
	).Scan(&e.ID)
}

func (s *PostgresStore) GetRecentLogs(sessionID int64, limit int) ([]*LogEntry, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, note, timestamp FROM log_entries
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
		`SELECT id, session_id, note, timestamp FROM log_entries
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
	err := row.Scan(&sess.ID, &sess.TaskName, &sess.StartTime, &sess.GitBranch, &sess.GitRepo, &tagsJSON)
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
			&sess.Message, &tagsJSON, &sess.GitBranch, &sess.GitRepo)
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
		if err := rows.Scan(&e.ID, &e.SessionID, &e.Note, &e.Timestamp); err != nil {
			return nil, err
		}
		result = append(result, &e)
	}
	return result, rows.Err()
}
