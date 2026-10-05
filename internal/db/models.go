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

	// Source is where the session came from: SourceManual for sessions
	// recorded in btrack itself, otherwise the agent tool it was imported
	// from (see the ingest package).
	Source     string
	ExternalID string // session id in the source tool; empty for manual sessions
	Cwd        string // working directory the source tool recorded

	// ActiveSeconds is time a human was at the keyboard; AgentSeconds is
	// time the agent worked between human turns. Nil means unknown, which
	// is always the case for manual sessions.
	ActiveSeconds *int64
	AgentSeconds  *int64

	Model   string
	Usage   TokenUsage
	CostUSD *float64 // estimated; nil when unknown
}

// TokenUsage is summed over every model call in a session.
type TokenUsage struct {
	Input      int64
	Output     int64
	CacheRead  int64
	CacheWrite int64
}

const SourceManual = "manual"

func (s *Session) source() string {
	if s.Source == "" {
		return SourceManual
	}
	return s.Source
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

// Store persists sessions. The listing methods (active, last, recent, by
// date, by project, projects) see manual sessions only, so imported agent
// sessions never inflate the hand-kept timesheet that history, export and
// client reports are built from. GetSessionsInRange and GetSessionByID see
// every source.
type Store interface {
	CreateSession(s *Session) error
	UpdateSession(s *Session) error
	GetActiveSession() (*Session, error)
	GetLastSession() (*Session, error)
	GetRecentSessions(limit int) ([]*Session, error)
	GetSessionsForDate(date time.Time) ([]*Session, error)
	GetSessionByID(id int64) (*Session, error)
	GetProjects() ([]string, error)
	GetSessionsByProject(project string, limit int) ([]*Session, error)
	CreateLogEntry(e *LogEntry) error
	GetRecentLogs(sessionID int64, limit int) ([]*LogEntry, error)
	GetAllLogs(sessionID int64) ([]*LogEntry, error)
	UpsertExternalSession(s *Session) (created bool, err error)
	GetSessionsInRange(from, to time.Time) ([]*Session, error)
	Close() error
}
