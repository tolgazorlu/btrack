package daemon

import "encoding/json"

const (
	ActionStart  = "start"
	ActionStop   = "stop"
	ActionSwitch = "switch"
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
	Warning string          `json:"warning,omitempty"`
}

type StartPayload struct {
	TaskName  string `json:"task_name"`
	GitBranch string `json:"git_branch,omitempty"`
	GitRepo   string `json:"git_repo,omitempty"`
	Project   string `json:"project,omitempty"`
}

type StopPayload struct {
	Message string `json:"message"`
	EndTime string `json:"end_time,omitempty"`
}

type SwitchPayload struct {
	TaskName  string `json:"task_name"`
	Message   string `json:"message,omitempty"`
	GitBranch string `json:"git_branch,omitempty"`
	GitRepo   string `json:"git_repo,omitempty"`
	Project   string `json:"project,omitempty"`
}

type SwitchData struct {
	Stopped *SessionDTO `json:"stopped,omitempty"`
	Started *SessionDTO `json:"started"`
}

type LogPayload struct {
	Note     string `json:"note"`
	ParentID int64  `json:"parent_id,omitempty"`
}

type StatusData struct {
	Active    bool        `json:"active"`
	Session   *SessionDTO `json:"session,omitempty"`
	RecentLog []LogDTO    `json:"recent_log,omitempty"`
}

type SessionDTO struct {
	ID        int64    `json:"id"`
	TaskName  string   `json:"task_name"`
	StartTime string   `json:"start_time"`
	EndTime   string   `json:"end_time,omitempty"`
	Message   string   `json:"message,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	GitBranch string   `json:"git_branch,omitempty"`
	GitRepo   string   `json:"git_repo,omitempty"`
	Project   string   `json:"project,omitempty"`
}

type LogDTO struct {
	ID        int64  `json:"id"`
	ParentID  int64  `json:"parent_id,omitempty"`
	Note      string `json:"note"`
	Timestamp string `json:"timestamp"`
}
