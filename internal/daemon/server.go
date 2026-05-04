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
	"time"

	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/db"
)

type Server struct {
	store    db.Store
	listener net.Listener
	state    *activeState
}

type activeState struct {
	session *db.Session
}

func NewServer(store db.Store) *Server {
	return &Server{store: store, state: &activeState{}}
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
	s.listener = ln

	if err := writePid(); err != nil {
		return err
	}

	// Restore any in-progress session from db.
	if sess, err := s.store.GetActiveSession(); err == nil && sess != nil {
		s.state.session = sess
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
	switch req.Action {
	case ActionPing:
		return Response{Success: true}
	case ActionStart:
		return s.handleStart(req)
	case ActionStop:
		return s.handleStop(req)
	case ActionLog:
		return s.handleLog(req)
	case ActionStatus:
		return s.handleStatus()
	case ActionResume:
		return s.handleResume()
	default:
		return Response{Success: false, Error: "unknown action: " + req.Action}
	}
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

	now := time.Now()
	s.state.session.EndTime = &now
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

func (s *Server) handleLog(req Request) Response {
	if s.state.session == nil {
		return Response{Success: false, Error: "no active session — run `btrack start <task>` first"}
	}

	var p LogPayload
	if err := json.Unmarshal(req.Payload, &p); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	entry := &db.LogEntry{
		SessionID: s.state.session.ID,
		Note:      p.Note,
		Timestamp: time.Now(),
	}
	if err := s.store.CreateLogEntry(entry); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	raw, _ := json.Marshal(map[string]string{"note": p.Note})
	return Response{Success: true, Data: raw}
}

func (s *Server) handleStatus() Response {
	status := StatusData{Active: s.state.session != nil}
	if s.state.session != nil {
		dto := sessionToDTO(s.state.session)
		status.Session = dto

		logs, err := s.store.GetRecentLogs(s.state.session.ID, 5)
		if err == nil {
			for _, l := range logs {
				status.RecentLog = append(status.RecentLog, LogDTO{
					Note:      l.Note,
					Timestamp: l.Timestamp.Format(time.RFC3339),
				})
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
		return Response{Success: false, Error: "no previous session to resume"}
	}

	// Create a new session copying the task name.
	newSess := &db.Session{
		TaskName:  sess.TaskName,
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
	data, _ := json.Marshal(resp)
	_, _ = conn.Write(data)
}

func writePid() error {
	pidFile := config.PidFile()
	if err := os.MkdirAll(filepath.Dir(pidFile), 0750); err != nil {
		return err
	}
	return os.WriteFile(filepath.Clean(pidFile), []byte(strconv.Itoa(os.Getpid())), 0600)
}

func sessionToDTO(s *db.Session) *SessionDTO {
	return &SessionDTO{
		ID:        s.ID,
		TaskName:  s.TaskName,
		StartTime: s.StartTime.Format(time.RFC3339),
		Tags:      s.Tags,
		GitBranch: s.GitBranch,
		GitRepo:   s.GitRepo,
	}
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
