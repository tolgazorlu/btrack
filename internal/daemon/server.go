package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/db"
)

type Server struct {
	mu           sync.Mutex
	store        db.Store
	listener     net.Listener
	state        *activeState
	lastActivity time.Time
	idleMinutes  int
	maxHours     int
}

const staleSessionThreshold = 6 * time.Hour

type activeState struct {
	session *db.Session
}

func NewServer(store db.Store) *Server {
	cfg, _ := config.Load()
	idleMinutes := 0
	maxHours := 0
	if cfg != nil {
		idleMinutes = cfg.Work.IdleMinutes
		maxHours = cfg.Work.MaxHours
	}
	return &Server{
		store:        store,
		state:        &activeState{},
		lastActivity: time.Now(),
		idleMinutes:  idleMinutes,
		maxHours:     maxHours,
	}
}

func (s *Server) Start() error {
	socketPath := config.SocketPath()
	if err := os.MkdirAll(filepath.Dir(socketPath), 0750); err != nil {
		return fmt.Errorf("create socket dir: %w", err)
	}

	_ = os.Remove(socketPath)

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("listen on socket: %w", err)
	}
	if err := os.Chmod(socketPath, 0600); err != nil {
		ln.Close()
		return fmt.Errorf("chmod socket: %w", err)
	}
	s.listener = ln

	if err := writePid(); err != nil {
		return err
	}

	if sess, err := s.store.GetActiveSession(); err == nil && sess != nil {
		s.state.session = sess
	}

	if s.idleMinutes > 0 {
		go s.idleWatcher()
	}
	if s.maxHours > 0 {
		go s.maxHoursWatcher()
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return nil
			}
			return fmt.Errorf("accept: %w", err)
		}
		go s.handleConn(conn)
	}
}

func (s *Server) Stop() {
	if s.listener != nil {
		_ = s.listener.Close()
	}
	_ = os.Remove(config.PidFile())
	_ = os.Remove(config.SocketPath())
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	data, err := io.ReadAll(conn)
	if err != nil {
		writeResponse(conn, Response{Success: false, Error: err.Error()})
		return
	}

	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		writeResponse(conn, Response{Success: false, Error: "invalid JSON"})
		return
	}

	resp := s.dispatch(req)
	writeResponse(conn, resp)
}

func (s *Server) dispatch(req Request) Response {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastActivity = time.Now()

	var resp Response
	switch req.Action {
	case ActionPing:
		resp = Response{Success: true}
	case ActionStart:
		resp = s.handleStart(req)
	case ActionStop:
		resp = s.handleStop(req)
	case ActionSwitch:
		resp = s.handleSwitch(req)
	case ActionLog:
		resp = s.handleLog(req)
	case ActionStatus:
		resp = s.handleStatus()
	case ActionResume:
		resp = s.handleResume()
	default:
		resp = Response{Success: false, Error: "unknown action: " + req.Action}
	}

	if resp.Warning == "" {
		resp.Warning = s.staleWarning()
	}
	return resp
}

func (s *Server) staleWarning() string {
	if s.state.session == nil {
		return ""
	}
	elapsed := time.Since(s.state.session.StartTime)
	if elapsed < staleSessionThreshold {
		return ""
	}
	return fmt.Sprintf(
		"session #%d %q has been running %s — `btrack x` to stop, or `btrack x --at <time>` to backdate",
		s.state.session.ID, s.state.session.TaskName, formatStaleDuration(elapsed),
	)
}

func formatStaleDuration(d time.Duration) string {
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func (s *Server) handleStart(req Request) Response {
	var p StartPayload
	if err := json.Unmarshal(req.Payload, &p); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	if s.state.session != nil {
		return Response{
			Success: false,
			Error:   fmt.Sprintf("session already active: %q — run `btrack stop` first", s.state.session.TaskName),
		}
	}

	sess := &db.Session{
		TaskName:  p.TaskName,
		StartTime: time.Now(),
		GitBranch: p.GitBranch,
		GitRepo:   p.GitRepo,
		Project:   p.Project,
	}
	if err := s.store.CreateSession(sess); err != nil {
		return Response{Success: false, Error: err.Error()}
	}
	s.state.session = sess

	dto := sessionToDTO(sess)
	raw, _ := json.Marshal(dto)
	return Response{Success: true, Data: raw}
}

func (s *Server) handleStop(req Request) Response {
	if s.state.session == nil {
		return Response{Success: false, Error: "no active session — run `btrack start <task>` first"}
	}

	var p StopPayload
	if err := json.Unmarshal(req.Payload, &p); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	end := time.Now()
	if p.EndTime != "" {
		t, err := time.Parse(time.RFC3339, p.EndTime)
		if err != nil {
			return Response{Success: false, Error: fmt.Sprintf("invalid end_time: %v", err)}
		}
		if t.Before(s.state.session.StartTime) {
			return Response{Success: false, Error: fmt.Sprintf(
				"end time %s is before session start %s",
				t.Format(time.RFC3339), s.state.session.StartTime.Format(time.RFC3339),
			)}
		}
		if t.After(time.Now().Add(time.Minute)) {
			return Response{Success: false, Error: "end time cannot be in the future"}
		}
		end = t
	}
	s.state.session.EndTime = &end
	s.state.session.Message = p.Message
	s.state.session.Tags = extractTags(p.Message)

	if err := s.store.UpdateSession(s.state.session); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	dto := sessionToDTO(s.state.session)
	raw, _ := json.Marshal(dto)
	s.state.session = nil
	return Response{Success: true, Data: raw}
}

func (s *Server) handleSwitch(req Request) Response {
	var p SwitchPayload
	if err := json.Unmarshal(req.Payload, &p); err != nil {
		return Response{Success: false, Error: err.Error()}
	}
	if strings.TrimSpace(p.TaskName) == "" {
		return Response{Success: false, Error: "task_name is required"}
	}

	out := SwitchData{}

	if s.state.session != nil {
		now := time.Now()
		s.state.session.EndTime = &now
		s.state.session.Message = p.Message
		s.state.session.Tags = extractTags(p.Message)
		if err := s.store.UpdateSession(s.state.session); err != nil {
			return Response{Success: false, Error: err.Error()}
		}
		out.Stopped = sessionToDTO(s.state.session)
		s.state.session = nil
	}

	sess := &db.Session{
		TaskName:  p.TaskName,
		StartTime: time.Now(),
		GitBranch: p.GitBranch,
		GitRepo:   p.GitRepo,
		Project:   p.Project,
	}
	if err := s.store.CreateSession(sess); err != nil {
		return Response{Success: false, Error: err.Error()}
	}
	s.state.session = sess
	out.Started = sessionToDTO(sess)

	raw, _ := json.Marshal(out)
	return Response{Success: true, Data: raw}
}

func (s *Server) handleLog(req Request) Response {
	if s.state.session == nil {
		return Response{Success: false, Error: "no active session — run `btrack start <task>` first"}
	}

	var p LogPayload
	if err := json.Unmarshal(req.Payload, &p); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	var parentID *int64
	if p.ParentID > 0 {
		pid := p.ParentID
		parentID = &pid
	}

	entry := &db.LogEntry{
		SessionID: s.state.session.ID,
		ParentID:  parentID,
		Note:      p.Note,
		Timestamp: time.Now(),
	}
	if err := s.store.CreateLogEntry(entry); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	raw, _ := json.Marshal(map[string]interface{}{"note": p.Note, "id": entry.ID})
	return Response{Success: true, Data: raw}
}

func (s *Server) handleStatus() Response {
	status := StatusData{Active: s.state.session != nil}
	if s.state.session != nil {
		dto := sessionToDTO(s.state.session)
		status.Session = dto

		logs, err := s.store.GetRecentLogs(s.state.session.ID, 12)
		if err == nil {
			for _, l := range logs {
				dto := LogDTO{
					ID:        l.ID,
					Note:      l.Note,
					Timestamp: l.Timestamp.Format(time.RFC3339),
				}
				if l.ParentID != nil {
					dto.ParentID = *l.ParentID
				}
				status.RecentLog = append(status.RecentLog, dto)
			}
		}
	}
	raw, _ := json.Marshal(status)
	return Response{Success: true, Data: raw}
}

func (s *Server) handleResume() Response {
	if s.state.session != nil {
		return Response{Success: false, Error: "session already active — stop it first"}
	}

	sess, err := s.store.GetLastSession()
	if err != nil || sess == nil {
		return Response{Success: false, Error: "no previous session found — start your first one with: btrack start <task>"}
	}

	newSess := &db.Session{
		TaskName:  sess.TaskName,
		Project:   sess.Project,
		StartTime: time.Now(),
		GitBranch: sess.GitBranch,
		GitRepo:   sess.GitRepo,
	}
	if err := s.store.CreateSession(newSess); err != nil {
		return Response{Success: false, Error: err.Error()}
	}
	s.state.session = newSess

	dto := sessionToDTO(newSess)
	raw, _ := json.Marshal(dto)
	return Response{Success: true, Data: raw}
}

func writeResponse(conn net.Conn, resp Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[btrack daemon] marshal response: %v\n", err)
		return
	}
	if _, err := conn.Write(data); err != nil {
		fmt.Fprintf(os.Stderr, "[btrack daemon] write response: %v\n", err)
	}
}

func writePid() error {
	pidFile := config.PidFile()
	if err := os.MkdirAll(filepath.Dir(pidFile), 0750); err != nil {
		return err
	}
	return os.WriteFile(filepath.Clean(pidFile), []byte(strconv.Itoa(os.Getpid())), 0600)
}

func sessionToDTO(s *db.Session) *SessionDTO {
	dto := &SessionDTO{
		ID:        s.ID,
		TaskName:  s.TaskName,
		StartTime: s.StartTime.Format(time.RFC3339),
		Message:   s.Message,
		Tags:      s.Tags,
		GitBranch: s.GitBranch,
		GitRepo:   s.GitRepo,
		Project:   s.Project,
	}
	if s.EndTime != nil {
		dto.EndTime = s.EndTime.Format(time.RFC3339)
	}
	return dto
}

func (s *Server) idleWatcher() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		if s.state.session != nil && time.Since(s.lastActivity) > time.Duration(s.idleMinutes)*time.Minute {
			s.autoStopIdle()
		}
		s.mu.Unlock()
	}
}

func (s *Server) autoStopIdle() {
	if s.state.session == nil {
		return
	}
	end := s.lastActivity
	if end.Before(s.state.session.StartTime) {
		end = s.state.session.StartTime
	}
	s.state.session.EndTime = &end
	s.state.session.Message = "auto-stopped: idle"
	s.state.session.Tags = append(s.state.session.Tags, "#idle")
	_ = s.store.UpdateSession(s.state.session)
	fmt.Fprintf(os.Stderr, "[btrack daemon] idle auto-stop: %q\n", s.state.session.TaskName)
	s.state.session = nil
}

func (s *Server) maxHoursWatcher() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		if s.state.session != nil {
			cap := time.Duration(s.maxHours) * time.Hour
			if time.Since(s.state.session.StartTime) > cap {
				s.autoStopMaxHours(cap)
			}
		}
		s.mu.Unlock()
	}
}

func (s *Server) autoStopMaxHours(cap time.Duration) {
	if s.state.session == nil {
		return
	}
	end := s.state.session.StartTime.Add(cap)
	s.state.session.EndTime = &end
	s.state.session.Message = fmt.Sprintf("auto-stopped: exceeded max duration (%dh)", s.maxHours)
	s.state.session.Tags = append(s.state.session.Tags, "#runaway")
	_ = s.store.UpdateSession(s.state.session)
	fmt.Fprintf(os.Stderr, "[btrack daemon] max-hours auto-stop: %q (cap %dh)\n",
		s.state.session.TaskName, s.maxHours)
	s.state.session = nil
}

func extractTags(msg string) []string {
	words := strings.Fields(msg)
	var tags []string
	for _, w := range words {
		if strings.HasPrefix(w, "#") {
			tags = append(tags, strings.ToLower(w))
		}
	}
	return tags
}
