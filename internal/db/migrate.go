package db

import (
	"database/sql"
	"fmt"
)

// migration is one forward-only schema step. Steps run in version order,
// each inside its own transaction, and are recorded in schema_migrations so
// they never run twice.
type migration struct {
	version int
	name    string
	up      func(tx *sql.Tx) error
}

var migrations = []migration{
	{1, "baseline sessions and log entries", migrateBaseline},
	{2, "agent ledger columns on sessions", migrateAgentLedger},
}

// SchemaVersion is the highest migration version this binary knows about.
func SchemaVersion() int { return migrations[len(migrations)-1].version }

func (s *SQLiteStore) migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		name       TEXT     NOT NULL,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	applied := map[int]bool{}
	rows, err := s.db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("read schema_migrations: %w", err)
	}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		applied[v] = true
	}
	rows.Close()

	for _, m := range migrations {
		if applied[m.version] {
			continue
		}
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		if err := m.up(tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d (%s): %w", m.version, m.name, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version, name) VALUES (?, ?)`, m.version, m.name); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// migrateBaseline creates the original schema. It is idempotent because
// databases created before schema_migrations existed already have some or
// all of it; those columns were added ad hoc by older releases.
func migrateBaseline(tx *sql.Tx) error {
	if _, err := tx.Exec(`
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
			FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_sessions_end_time ON sessions(end_time);
		CREATE INDEX IF NOT EXISTS idx_log_entries_session ON log_entries(session_id);
	`); err != nil {
		return err
	}
	if err := addColumnIfMissing(tx, "sessions", "project", `TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	return addColumnIfMissing(tx, "log_entries", "parent_id", `INTEGER REFERENCES log_entries(id) ON DELETE SET NULL`)
}

// migrateAgentLedger adds what an imported AI agent session carries: where
// it came from, its id in that tool (for idempotent re-sync), how much of
// it was a human at the keyboard versus the agent working alone, and what
// it cost.
func migrateAgentLedger(tx *sql.Tx) error {
	cols := []struct{ name, def string }{
		{"source", `TEXT NOT NULL DEFAULT 'manual'`},
		{"external_id", `TEXT`},
		{"cwd", `TEXT NOT NULL DEFAULT ''`},
		{"active_seconds", `INTEGER`},
		{"agent_seconds", `INTEGER`},
		{"model", `TEXT NOT NULL DEFAULT ''`},
		{"input_tokens", `INTEGER NOT NULL DEFAULT 0`},
		{"output_tokens", `INTEGER NOT NULL DEFAULT 0`},
		{"cache_read_tokens", `INTEGER NOT NULL DEFAULT 0`},
		{"cache_write_tokens", `INTEGER NOT NULL DEFAULT 0`},
		{"cost_usd", `REAL`},
	}
	for _, c := range cols {
		if err := addColumnIfMissing(tx, "sessions", c.name, c.def); err != nil {
			return err
		}
	}
	_, err := tx.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_sessions_source_external
			ON sessions(source, external_id) WHERE external_id IS NOT NULL;
		CREATE INDEX IF NOT EXISTS idx_sessions_start_time ON sessions(start_time);
	`)
	return err
}

func addColumnIfMissing(tx *sql.Tx, table, column, def string) error {
	has, err := hasColumn(tx, table, column)
	if err != nil || has {
		return err
	}
	_, err = tx.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, column, def))
	return err
}

func hasColumn(tx *sql.Tx, table, column string) (bool, error) {
	rows, err := tx.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid, notNull, pk int
			name, typ        string
			dflt             sql.NullString
		)
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}
