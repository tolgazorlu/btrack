// Package mcp serves btrack to AI agents over the Model Context Protocol
// (JSON-RPC 2.0, newline-delimited, on stdin/stdout). It is a small,
// dependency-free implementation of the parts agents use: initialize,
// ping, tools/list and tools/call.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// SupportedVersions are the protocol revisions this server speaks, newest
// first. A client asking for one of them gets it; anything else gets the
// newest.
var SupportedVersions = []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}

// Tool is one callable tool.
type Tool struct {
	Name        string         `json:"name"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	Annotations map[string]any `json:"annotations,omitempty"`
	// Call runs the tool. A returned error becomes a tool result with
	// isError set (the model sees it and can correct itself), not a
	// protocol error.
	Call func(args json.RawMessage) (any, error) `json:"-"`
}

// Server dispatches requests to tools.
type Server struct {
	Name, Version string
	Instructions  string
	Tools         []Tool

	mu  sync.Mutex
	out *json.Encoder
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// Serve reads requests from r until EOF and writes responses to w.
func (s *Server) Serve(r io.Reader, w io.Writer) error {
	s.out = json.NewEncoder(w)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			s.send(response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{-32700, "parse error"}})
			continue
		}
		if len(req.ID) == 0 {
			continue // a notification (e.g. notifications/initialized): no reply
		}
		result, rerr := s.handle(req)
		s.send(response{JSONRPC: "2.0", ID: req.ID, Result: result, Error: rerr})
	}
	return sc.Err()
}

func (s *Server) send(r response) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.out.Encode(r)
}

func (s *Server) handle(req request) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &p)
		version := SupportedVersions[0]
		for _, v := range SupportedVersions {
			if v == p.ProtocolVersion {
				version = v
			}
		}
		return map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": s.Name, "version": s.Version},
			"instructions":    s.Instructions,
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": s.Tools}, nil
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, &rpcError{-32602, "invalid params"}
		}
		for _, t := range s.Tools {
			if t.Name == p.Name {
				return s.call(t, p.Arguments), nil
			}
		}
		return nil, &rpcError{-32602, fmt.Sprintf("unknown tool %q", p.Name)}
	}
	return nil, &rpcError{-32601, "method not found: " + req.Method}
}

func (s *Server) call(t Tool, args json.RawMessage) (out map[string]any) {
	defer func() {
		if r := recover(); r != nil {
			out = errorResult(fmt.Errorf("internal error: %v", r))
		}
	}()
	if len(args) == 0 || string(args) == "null" {
		args = json.RawMessage("{}")
	}
	v, err := t.Call(args)
	if err != nil {
		return errorResult(err)
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return errorResult(err)
	}
	res := map[string]any{"content": []map[string]any{{"type": "text", "text": string(b)}}}
	// structuredContent must be a JSON object.
	var obj map[string]any
	if json.Unmarshal(b, &obj) == nil {
		res["structuredContent"] = obj
	}
	return res
}

func errorResult(err error) map[string]any {
	return map[string]any{
		"isError": true,
		"content": []map[string]any{{"type": "text", "text": err.Error()}},
	}
}
