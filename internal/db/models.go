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
	Project   string
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
	ParentID  *int64
	Note      string
	Timestamp time.Time
}

type Store interface {
	CreateSession(s *Session) error
	UpdateSession(s *Session) error
	GetActiveSession() (*Session, error)
	GetLastSession() (*Session, error)
	GetRecentSessions(limit int) ([]*Session, error)
	GetSessionsForDate(date time.Time) ([]*Session, error)
	GetSessionByID(id int64) (*Session, error)
	SearchSessions(query string) ([]*Session, error)
	GetProjects() ([]string, error)
	GetSessionsByProject(project string, limit int) ([]*Session, error)
	CreateLogEntry(e *LogEntry) error
	GetRecentLogs(sessionID int64, limit int) ([]*LogEntry, error)
	GetAllLogs(sessionID int64) ([]*LogEntry, error)
	Close() error
}
