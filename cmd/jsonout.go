package cmd

import (
	"encoding/json"
	"os"
	"time"

	"github.com/tolgazorlu/btrack/internal/db"
)

// jsonOutput is the global --json flag: every command that prints data
// prints it as JSON instead, for scripts and AI agents.
var jsonOutput bool

// printJSON writes v as indented JSON to stdout.
func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// SessionJSON is the stable JSON shape of a session across commands.
type SessionJSON struct {
	ID              int64      `json:"id"`
	Task            string     `json:"task"`
	Project         string     `json:"project,omitempty"`
	Start           time.Time  `json:"start"`
	End             *time.Time `json:"end,omitempty"`
	DurationSeconds int64      `json:"duration_seconds"`
	Active          bool       `json:"active"`
	Message         string     `json:"message,omitempty"`
	Tags            []string   `json:"tags,omitempty"`
	GitBranch       string     `json:"git_branch,omitempty"`
	GitRepo         string     `json:"git_repo,omitempty"`
	Source          string     `json:"source"`
	ExternalID      string     `json:"external_id,omitempty"`
	ActiveSeconds   *int64     `json:"active_seconds,omitempty"`
	AgentSeconds    *int64     `json:"agent_seconds,omitempty"`
	Model           string     `json:"model,omitempty"`
	CostUSD         *float64   `json:"cost_usd,omitempty"`
	Notes           []NoteJSON `json:"notes,omitempty"`
}

// NoteJSON is a note on a session.
type NoteJSON struct {
	ID   int64     `json:"id"`
	Note string    `json:"note"`
	At   time.Time `json:"at"`
}

// toSessionJSON converts a session; notes may be nil.
func toSessionJSON(s *db.Session, notes []*db.LogEntry) SessionJSON {
	src := s.Source
	if src == "" {
		src = db.SourceManual
	}
	out := SessionJSON{
		ID: s.ID, Task: s.TaskName, Project: s.Project, Start: s.StartTime, End: s.EndTime,
		DurationSeconds: int64(s.Duration().Seconds()), Active: s.EndTime == nil,
		Message: s.Message, Tags: s.Tags, GitBranch: s.GitBranch, GitRepo: s.GitRepo,
		Source: src, ExternalID: s.ExternalID, ActiveSeconds: s.ActiveSeconds, AgentSeconds: s.AgentSeconds,
		Model: s.Model, CostUSD: s.CostUSD,
	}
	for _, n := range notes {
		out.Notes = append(out.Notes, NoteJSON{ID: n.ID, Note: n.Note, At: n.Timestamp})
	}
	return out
}
