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

// Commit is one of the user's own commits in a repo btrack has seen them
// work in. SessionID links it to the session that produced it, if any.
type Commit struct {
	ID         int64
	Project    string
	RepoPath   string
	SHA        string
	AuthoredAt time.Time
	Subject    string
	Files      int
	Insertions int
	Deletions  int
	SessionID  *int64
}

// Segment is a stretch of continuous work inside an imported session.
type Segment struct {
	SessionID     int64
	Start, End    time.Time
	ActiveSeconds int64
	AgentSeconds  int64
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
	// UpsertCommit records a commit once per (project, sha); a re-scan
	// keeps the existing row and its link.
	UpsertCommit(c *Commit) (created bool, err error)
	GetCommitsInRange(from, to time.Time) ([]*Commit, error)
	LinkCommit(commitID int64, sessionID *int64) error
	// ReplaceSegments sets a session's segments, dropping any previous set.
	ReplaceSegments(sessionID int64, segs []Segment) error
	// GetSegmentsInRange returns segments overlapping [from, to).
	GetSegmentsInRange(from, to time.Time) ([]Segment, error)
	Close() error
}
