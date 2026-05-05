package daemon

import "encoding/json"

// Action constants for IPC protocol.
const (
	ActionStart  = "start"
	ActionStop   = "stop"
	ActionLog    = "log"
	ActionStatus = "status"
	ActionResume = "resume"
	ActionPing   = "ping"
)

type Request struct {
	Action  string          `json:"action"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type Response struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   string          `json:"error,omitempty"`
}

type StartPayload struct {
	TaskName  string `json:"task_name"`
	GitBranch string `json:"git_branch,omitempty"`
	GitRepo   string `json:"git_repo,omitempty"`
	Project   string `json:"project,omitempty"`
}

type StopPayload struct {
	Message string `json:"message"`
}

type LogPayload struct {
	Note     string `json:"note"`
	ParentID int64  `json:"parent_id,omitempty"` // 0 = top-level note
}

type StatusData struct {
	Active    bool        `json:"active"`
	Session   *SessionDTO `json:"session,omitempty"`
	RecentLog []LogDTO    `json:"recent_log,omitempty"`
}

type SessionDTO struct {
	ID        int64    `json:"id"`
	TaskName  string   `json:"task_name"`
	StartTime string   `json:"start_time"` // RFC3339
	Tags      []string `json:"tags,omitempty"`
	GitBranch string   `json:"git_branch,omitempty"`
	GitRepo   string   `json:"git_repo,omitempty"`
	Project   string   `json:"project,omitempty"`
}

type LogDTO struct {
	ID        int64  `json:"id"`
	ParentID  int64  `json:"parent_id,omitempty"` // 0 = top-level
	Note      string `json:"note"`
	Timestamp string `json:"timestamp"` // RFC3339
}
