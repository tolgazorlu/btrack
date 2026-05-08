package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tolgazorlu/btrack/internal/daemon"
	"github.com/tolgazorlu/btrack/internal/db"
)

type Deps struct {
	Client *daemon.Client
	Store  db.Store
}

type Tool struct {
	Name        string
	Description string
	Invoke func(ctx context.Context, raw json.RawMessage) (any, error)
	Register func(s *mcp.Server)
}

func makeTool[In, Out any](name, desc string, h func(context.Context, In) (Out, error)) Tool {
	invoke := func(ctx context.Context, raw json.RawMessage) (any, error) {
		var in In
		if len(raw) > 0 && string(raw) != "null" {
			if err := json.Unmarshal(raw, &in); err != nil {
				return nil, fmt.Errorf("decode args: %w", err)
			}
		}
		return h(ctx, in)
	}
	register := func(s *mcp.Server) {
		mcp.AddTool(
			s,
			&mcp.Tool{Name: name, Description: desc},
			func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
				out, err := h(ctx, in)
				if err != nil {
					var zero Out
					return &mcp.CallToolResult{
						IsError: true,
						Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
					}, zero, nil
				}
				body, mErr := json.Marshal(out)
				if mErr != nil {
					return nil, out, mErr
				}
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: string(body)}},
				}, out, nil
			},
		)
	}
	return Tool{Name: name, Description: desc, Invoke: invoke, Register: register}
}

func Tools(d Deps) []Tool {
	return []Tool{
		makeTool("btrack_status", "Get the active session, its tags, and the most recent log notes attached to it. Returns active=false when no session is running.", d.statusHandler),
		makeTool("btrack_start", "Start a new tracking session for a task. Fails if a session is already active. Captures the current git branch and repo automatically.", d.startHandler),
		makeTool("btrack_stop", "Stop the currently active session, optionally with a closing message. #tags inside the message are extracted.", d.stopHandler),
		makeTool("btrack_switch", "Atomically stop the active session and start a new one. Useful when context-switching between tasks.", d.switchHandler),
		makeTool("btrack_resume", "Resume the most recently stopped task by starting a fresh session with the same name and git context.", d.resumeHandler),
		makeTool("btrack_log_note", "Add a checkpoint note to the active session. Optional parent_id makes the note a sub-note under another note.", d.logNoteHandler),
		makeTool("btrack_history", "List tracked sessions in a time window. window can be: today, yesterday, week, month, date:YYYY-MM-DD, or last_n:N (default today). Optional project filter.", d.historyHandler),
		makeTool("btrack_search", "Full-text search across session task names and closing messages.", d.searchHandler),
		makeTool("btrack_list_projects", "List all known projects with their cumulative tracked time.", d.listProjectsHandler),
		makeTool("btrack_get_session", "Get a single session by ID, including every log note attached to it.", d.getSessionHandler),
	}
}

type StatusOut struct {
	Active     bool          `json:"active"`
	Session    *SessionView  `json:"session,omitempty"`
	RecentLog  []LogEntryDTO `json:"recent_log,omitempty"`
}

type StartIn struct {
	TaskName string `json:"task_name" jsonschema:"the task you are starting work on"`
	Project  string `json:"project,omitempty" jsonschema:"optional project name to associate the session with"`
}
type StartOut struct {
	Session SessionView `json:"session"`
}

type StopIn struct {
	Message string `json:"message,omitempty" jsonschema:"closing message; #tags inside are extracted"`
}
type StopOut struct {
	Session SessionView `json:"session"`
}

type SwitchIn struct {
	TaskName string `json:"task_name" jsonschema:"name of the new task to start"`
	Message  string `json:"message,omitempty" jsonschema:"closing message for the session being stopped"`
	Project  string `json:"project,omitempty" jsonschema:"optional project for the new session"`
}
type SwitchOut struct {
	Stopped *SessionView `json:"stopped,omitempty"`
	Started SessionView  `json:"started"`
}

type ResumeOut struct {
	Session SessionView `json:"session"`
}

type LogNoteIn struct {
	Note     string `json:"note" jsonschema:"the note text to attach to the active session"`
	ParentID int64  `json:"parent_id,omitempty" jsonschema:"optional parent note id; non-zero turns this into a sub-note"`
}
type LogNoteOut struct {
	NoteID int64  `json:"note_id"`
	Note   string `json:"note"`
}

type HistoryIn struct {
	Window  string `json:"window,omitempty" jsonschema:"today | yesterday | week | month | date:YYYY-MM-DD | last_n:N (default today)"`
	Project string `json:"project,omitempty" jsonschema:"optional project filter"`
}
type HistoryOut struct {
	Window   string        `json:"window"`
	Count    int           `json:"count"`
	Sessions []SessionView `json:"sessions"`
}

type SearchIn struct {
	Query string `json:"query" jsonschema:"text to search for in task names and messages"`
}
type SearchOut struct {
	Query    string        `json:"query"`
	Count    int           `json:"count"`
	Sessions []SessionView `json:"sessions"`
}

type ProjectsOut struct {
	Projects []ProjectStat `json:"projects"`
}

type ProjectStat struct {
	Name           string  `json:"name"`
	SessionCount   int     `json:"session_count"`
	TotalHours     float64 `json:"total_hours"`
	TotalMinutes   int64   `json:"total_minutes"`
}

type GetSessionIn struct {
	ID int64 `json:"id" jsonschema:"the session id to fetch"`
}
type GetSessionOut struct {
	Session SessionView   `json:"session"`
	Notes   []LogEntryDTO `json:"notes"`
}

type SessionView struct {
	ID            int64    `json:"id"`
	TaskName      string   `json:"task_name"`
	StartTime     string   `json:"start_time"`
	EndTime       string   `json:"end_time,omitempty"`
	DurationMins  int64    `json:"duration_minutes"`
	Active        bool     `json:"active"`
	Message       string   `json:"message,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	GitBranch     string   `json:"git_branch,omitempty"`
	GitRepo       string   `json:"git_repo,omitempty"`
	Project       string   `json:"project,omitempty"`
}

type LogEntryDTO struct {
	ID        int64  `json:"id"`
	ParentID  int64  `json:"parent_id,omitempty"`
	Note      string `json:"note"`
	Timestamp string `json:"timestamp"`
}

type emptyArgs struct{}

func (d Deps) statusHandler(ctx context.Context, _ emptyArgs) (StatusOut, error) {
	resp, err := d.Client.Send(daemon.ActionStatus, nil)
	if err != nil {
		return StatusOut{}, err
	}
	if !resp.Success {
		return StatusOut{}, fmt.Errorf("%s", resp.Error)
	}
	var sd daemon.StatusData
	if err := json.Unmarshal(resp.Data, &sd); err != nil {
		return StatusOut{}, err
	}
	out := StatusOut{Active: sd.Active}
	if sd.Session != nil {
		v := sessionDTOToView(*sd.Session, true)
		out.Session = &v
	}
	for _, l := range sd.RecentLog {
		out.RecentLog = append(out.RecentLog, LogEntryDTO{
			ID: l.ID, ParentID: l.ParentID, Note: l.Note, Timestamp: l.Timestamp,
		})
	}
	return out, nil
}

func (d Deps) startHandler(ctx context.Context, in StartIn) (StartOut, error) {
	if strings.TrimSpace(in.TaskName) == "" {
		return StartOut{}, fmt.Errorf("task_name is required")
	}
	branch, repo := gitContext()
	resp, err := d.Client.Send(daemon.ActionStart, daemon.StartPayload{
		TaskName:  in.TaskName,
		GitBranch: branch,
		GitRepo:   repo,
		Project:   in.Project,
	})
	if err != nil {
		return StartOut{}, err
	}
	if !resp.Success {
		return StartOut{}, fmt.Errorf("%s", resp.Error)
	}
	var s daemon.SessionDTO
	if err := json.Unmarshal(resp.Data, &s); err != nil {
		return StartOut{}, err
	}
	return StartOut{Session: sessionDTOToView(s, true)}, nil
}

func (d Deps) stopHandler(ctx context.Context, in StopIn) (StopOut, error) {
	resp, err := d.Client.Send(daemon.ActionStop, daemon.StopPayload{Message: in.Message})
	if err != nil {
		return StopOut{}, err
	}
	if !resp.Success {
		return StopOut{}, fmt.Errorf("%s", resp.Error)
	}
	var s daemon.SessionDTO
	if err := json.Unmarshal(resp.Data, &s); err != nil {
		return StopOut{}, err
	}
	return StopOut{Session: sessionDTOToView(s, false)}, nil
}

func (d Deps) switchHandler(ctx context.Context, in SwitchIn) (SwitchOut, error) {
	if strings.TrimSpace(in.TaskName) == "" {
		return SwitchOut{}, fmt.Errorf("task_name is required")
	}
	branch, repo := gitContext()
	data, err := d.Client.Switch(daemon.SwitchPayload{
		TaskName:  in.TaskName,
		Message:   in.Message,
		GitBranch: branch,
		GitRepo:   repo,
		Project:   in.Project,
	})
	if err != nil {
		return SwitchOut{}, err
	}
	out := SwitchOut{Started: sessionDTOToView(*data.Started, true)}
	if data.Stopped != nil {
		v := sessionDTOToView(*data.Stopped, false)
		out.Stopped = &v
	}
	return out, nil
}

func (d Deps) resumeHandler(ctx context.Context, _ emptyArgs) (ResumeOut, error) {
	resp, err := d.Client.Send(daemon.ActionResume, nil)
	if err != nil {
		return ResumeOut{}, err
	}
	if !resp.Success {
		return ResumeOut{}, fmt.Errorf("%s", resp.Error)
	}
	var s daemon.SessionDTO
	if err := json.Unmarshal(resp.Data, &s); err != nil {
		return ResumeOut{}, err
	}
	return ResumeOut{Session: sessionDTOToView(s, true)}, nil
}

func (d Deps) logNoteHandler(ctx context.Context, in LogNoteIn) (LogNoteOut, error) {
	if strings.TrimSpace(in.Note) == "" {
		return LogNoteOut{}, fmt.Errorf("note is required")
	}
	resp, err := d.Client.Send(daemon.ActionLog, daemon.LogPayload{
		Note: in.Note, ParentID: in.ParentID,
	})
	if err != nil {
		return LogNoteOut{}, err
	}
	if !resp.Success {
		return LogNoteOut{}, fmt.Errorf("%s", resp.Error)
	}
	var raw struct {
		ID   int64  `json:"id"`
		Note string `json:"note"`
	}
	_ = json.Unmarshal(resp.Data, &raw)
	return LogNoteOut{NoteID: raw.ID, Note: raw.Note}, nil
}

func (d Deps) historyHandler(ctx context.Context, in HistoryIn) (HistoryOut, error) {
	window := strings.TrimSpace(in.Window)
	if window == "" {
		window = "today"
	}

	sessions, err := d.fetchWindow(window)
	if err != nil {
		return HistoryOut{}, err
	}
	if in.Project != "" {
		sessions = filterByProject(sessions, in.Project)
	}

	out := HistoryOut{Window: window, Count: len(sessions)}
	for _, s := range sessions {
		out.Sessions = append(out.Sessions, sessionToView(s))
	}
	return out, nil
}

func (d Deps) searchHandler(ctx context.Context, in SearchIn) (SearchOut, error) {
	q := strings.TrimSpace(in.Query)
	if q == "" {
		return SearchOut{}, fmt.Errorf("query is required")
	}
	sessions, err := d.Store.SearchSessions(q)
	if err != nil {
		return SearchOut{}, err
	}
	out := SearchOut{Query: q, Count: len(sessions)}
	for _, s := range sessions {
		out.Sessions = append(out.Sessions, sessionToView(s))
	}
	return out, nil
}

func (d Deps) listProjectsHandler(ctx context.Context, _ emptyArgs) (ProjectsOut, error) {
	names, err := d.Store.GetProjects()
	if err != nil {
		return ProjectsOut{}, err
	}
	out := ProjectsOut{}
	for _, name := range names {
		ps, err := d.Store.GetSessionsByProject(name, 10000)
		if err != nil {
			return ProjectsOut{}, err
		}
		var total time.Duration
		for _, s := range ps {
			total += s.Duration()
		}
		out.Projects = append(out.Projects, ProjectStat{
			Name:         name,
			SessionCount: len(ps),
			TotalHours:   total.Hours(),
			TotalMinutes: int64(total.Minutes()),
		})
	}
	return out, nil
}

func (d Deps) getSessionHandler(ctx context.Context, in GetSessionIn) (GetSessionOut, error) {
	if in.ID <= 0 {
		return GetSessionOut{}, fmt.Errorf("id must be > 0")
	}
	sess, err := d.Store.GetSessionByID(in.ID)
	if err != nil {
		return GetSessionOut{}, err
	}
	if sess == nil {
		return GetSessionOut{}, fmt.Errorf("session %d not found", in.ID)
	}
	notes, err := d.Store.GetAllLogs(in.ID)
	if err != nil {
		return GetSessionOut{}, err
	}
	out := GetSessionOut{Session: sessionToView(sess)}
	for _, n := range notes {
		dto := LogEntryDTO{ID: n.ID, Note: n.Note, Timestamp: n.Timestamp.Format(time.RFC3339)}
		if n.ParentID != nil {
			dto.ParentID = *n.ParentID
		}
		out.Notes = append(out.Notes, dto)
	}
	return out, nil
}

func (d Deps) fetchWindow(window string) ([]*db.Session, error) {
	now := time.Now()

	switch {
	case window == "today":
		return d.Store.GetSessionsForDate(now)
	case window == "yesterday":
		return d.Store.GetSessionsForDate(now.AddDate(0, 0, -1))
	case window == "week":
		return d.fetchSince(now.AddDate(0, 0, -7))
	case window == "month":
		return d.fetchSince(now.AddDate(0, -1, 0))
	case strings.HasPrefix(window, "date:"):
		raw := strings.TrimPrefix(window, "date:")
		t, err := time.ParseInLocation("2006-01-02", raw, time.Local)
		if err != nil {
			return nil, fmt.Errorf("invalid date %q: %w", raw, err)
		}
		return d.Store.GetSessionsForDate(t)
	case strings.HasPrefix(window, "last_n:"):
		raw := strings.TrimPrefix(window, "last_n:")
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid last_n value %q", raw)
		}
		return d.Store.GetRecentSessions(n)
	default:
		return nil, fmt.Errorf("unknown window %q (use today | yesterday | week | month | date:YYYY-MM-DD | last_n:N)", window)
	}
}

func (d Deps) fetchSince(since time.Time) ([]*db.Session, error) {
	all, err := d.Store.GetRecentSessions(1000)
	if err != nil {
		return nil, err
	}
	out := make([]*db.Session, 0, len(all))
	for _, s := range all {
		if !s.StartTime.Before(since) {
			out = append(out, s)
		}
	}
	return out, nil
}

func filterByProject(sessions []*db.Session, project string) []*db.Session {
	out := make([]*db.Session, 0, len(sessions))
	for _, s := range sessions {
		if strings.EqualFold(s.Project, project) {
			out = append(out, s)
		}
	}
	return out
}

func sessionToView(s *db.Session) SessionView {
	v := SessionView{
		ID:           s.ID,
		TaskName:     s.TaskName,
		StartTime:    s.StartTime.Format(time.RFC3339),
		DurationMins: int64(s.Duration().Minutes()),
		Active:       s.EndTime == nil,
		Message:      s.Message,
		Tags:         s.Tags,
		GitBranch:    s.GitBranch,
		GitRepo:      s.GitRepo,
		Project:      s.Project,
	}
	if s.EndTime != nil {
		v.EndTime = s.EndTime.Format(time.RFC3339)
	}
	return v
}

func sessionDTOToView(s daemon.SessionDTO, active bool) SessionView {
	v := SessionView{
		ID:        s.ID,
		TaskName:  s.TaskName,
		StartTime: s.StartTime,
		EndTime:   s.EndTime,
		Message:   s.Message,
		Active:    active,
		Tags:      s.Tags,
		GitBranch: s.GitBranch,
		GitRepo:   s.GitRepo,
		Project:   s.Project,
	}
	if t, err := time.Parse(time.RFC3339, s.StartTime); err == nil {
		if active {
			v.DurationMins = int64(time.Since(t).Minutes())
		} else if s.EndTime != "" {
			if e, err := time.Parse(time.RFC3339, s.EndTime); err == nil {
				v.DurationMins = int64(e.Sub(t).Minutes())
			}
		}
	}
	return v
}

func gitContext() (branch, repo string) {
	if out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		branch = strings.TrimSpace(string(out))
	}
	if out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output(); err == nil {
		repo = filepath.Base(strings.TrimSpace(string(out)))
	}
	return
}
