package db

import "time"

type Session struct {
	ID        int64
	TaskName  string
	StartTime time.Time
	EndTime   *time.Time
	Message   string
	Tags      []string
	GitBranch string
	GitRepo   string
}

func (s *Session) Duration() time.Duration {
	if s.EndTime != nil {
		return s.EndTime.Sub(s.StartTime)
	}
	return time.Since(s.StartTime)
}

type LogEntry struct {
	ID        int64
	SessionID int64
	Note      string
	Timestamp time.Time
}

// Store is the persistence interface. Both SQLite and PostgreSQL implement it.
type Store interface {
	CreateSession(s *Session) error
	UpdateSession(s *Session) error
	GetActiveSession() (*Session, error)
	GetLastSession() (*Session, error)
	GetRecentSessions(limit int) ([]*Session, error)
	GetSessionsForDate(date time.Time) ([]*Session, error)
	CreateLogEntry(e *LogEntry) error
	GetRecentLogs(sessionID int64, limit int) ([]*LogEntry, error)
	GetAllLogs(sessionID int64) ([]*LogEntry, error)
	Close() error
}
