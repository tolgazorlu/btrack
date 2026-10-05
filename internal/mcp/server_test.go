package mcp

import (
	"bufio"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tolgazorlu/btrack/internal/db"
)

func newServer(t *testing.T, now time.Time) *Server {
	t.Helper()
	path := filepath.Join(t.TempDir(), "b.db")
	return &Server{Name: "btrack", Version: "test", Instructions: Instructions, Tools: Tools(Env{
		Open: func() (db.Store, error) { return db.NewSQLiteStore(path) },
		Home: t.TempDir(),
		Now:  func() time.Time { return now },
	})}
}

// exchange sends newline-delimited requests and returns responses by id.
func exchange(t *testing.T, s *Server, reqs ...string) map[float64]map[string]any {
	t.Helper()
	var out strings.Builder
	if err := s.Serve(strings.NewReader(strings.Join(reqs, "\n")+"\n"), &out); err != nil {
		t.Fatal(err)
	}
	got := map[float64]map[string]any{}
	sc := bufio.NewScanner(strings.NewReader(out.String()))
	for sc.Scan() {
		var r map[string]any
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatalf("bad response %q: %v", sc.Text(), err)
		}
		if id, ok := r["id"].(float64); ok {
			got[id] = r
		}
	}
	return got
}

func call(id int, tool, args string) string {
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": json.RawMessage(args)}})
	return string(b)
}

func result(t *testing.T, r map[string]any) (text string, isErr bool) {
	t.Helper()
	res, ok := r["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result: %v", r)
	}
	content := res["content"].([]any)[0].(map[string]any)
	e, _ := res["isError"].(bool)
	return content["text"].(string), e
}

func TestProtocol(t *testing.T) {
	s := newServer(t, time.Now())
	got := exchange(t, s,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":4,"method":"nope"}`,
		`not json`,
		`{"jsonrpc":"2.0","id":5,"method":"initialize","params":{"protocolVersion":"1999-01-01"}}`,
		call(6, "missing", `{}`),
	)
	init := got[1]["result"].(map[string]any)
	if init["protocolVersion"] != "2025-06-18" || init["instructions"] == "" {
		t.Errorf("initialize = %v", init)
	}
	if got[5]["result"].(map[string]any)["protocolVersion"] != SupportedVersions[0] {
		t.Error("unknown version not answered with the newest")
	}
	var names []string
	for _, tl := range got[2]["result"].(map[string]any)["tools"].([]any) {
		m := tl.(map[string]any)
		names = append(names, m["name"].(string))
		if m["inputSchema"].(map[string]any)["type"] != "object" {
			t.Errorf("%s: schema not an object", m["name"])
		}
	}
	if strings.Join(names, ",") != "recap,log_work,list_sessions,unlogged_days,sync,projects" {
		t.Errorf("tools = %v", names)
	}
	if got[3]["error"] != nil {
		t.Error("ping failed")
	}
	if got[4]["error"].(map[string]any)["code"].(float64) != -32601 {
		t.Error("unknown method not -32601")
	}
	if got[6]["error"] == nil {
		t.Error("unknown tool accepted")
	}
	if len(got) != 7 { // 6 ids + the parse error (id null, not counted) = 7 numbered replies
		t.Logf("responses: %d", len(got))
	}
}

func TestLogWorkThenRecapAndList(t *testing.T) {
	now := time.Date(2026, 10, 5, 18, 0, 0, 0, time.Local)
	s := newServer(t, now)
	got := exchange(t, s,
		call(1, "log_work", `{"task":"ship recap","project":"btrack","start":"2026-10-05 12:30","minutes":40,"message":"done #feature"}`),
		call(2, "log_work", `{"task":"overlap","project":"btrack","start":"12:45","end":"13:00"}`),
		call(3, "log_work", `{"task":"x","start":"12:00","bogus":1}`),
		call(4, "log_work", `{"task":"future","start":"19:00","minutes":30}`),
		call(5, "recap", `{"period":"day"}`),
		call(6, "list_sessions", `{"period":"week","source":"manual"}`),
		call(7, "unlogged_days", `{}`),
		call(8, "projects", `{}`),
	)
	if text, isErr := result(t, got[1]); isErr || !strings.Contains(text, `"minutes": 40`) {
		t.Fatalf("log_work: %s", text)
	}
	for id, want := range map[float64]string{2: "overlaps session #", 3: "unknown field", 4: "in the future", 7: "project is required"} {
		if text, isErr := result(t, got[id]); !isErr || !strings.Contains(text, want) {
			t.Errorf("call %v: want error %q, got %q (isError=%v)", id, want, text, isErr)
		}
	}
	var rc struct {
		Total struct {
			TimeSeconds int64 `json:"time_seconds"`
		} `json:"total"`
	}
	text, _ := result(t, got[5])
	json.Unmarshal([]byte(text), &rc)
	if rc.Total.TimeSeconds != 2400 {
		t.Errorf("recap total = %d, want 2400: %s", rc.Total.TimeSeconds, text)
	}
	if text, _ := result(t, got[6]); !strings.Contains(text, "ship recap") {
		t.Errorf("list_sessions: %s", text)
	}
	if text, _ := result(t, got[8]); !strings.Contains(text, `"name": "btrack"`) {
		t.Errorf("projects: %s", text)
	}
	if sc := got[5]["result"].(map[string]any)["structuredContent"]; sc == nil {
		t.Error("structuredContent missing")
	}
}
