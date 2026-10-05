// Claude Code reader: parses the JSONL transcripts Claude Code writes under
// ~/.claude/projects/<escaped-cwd>/<session-uuid>.jsonl into ingest.Session
// values. One JSON object per line; record "type" includes "user",
// "assistant", "system", "summary", "custom-title", "ai-title",
// "file-history-snapshot" and more that we don't care about.
package ingest

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ClaudeCodeReader implements ingest.Reader for Claude Code's own JSONL
// session transcripts.
type ClaudeCodeReader struct{}

func (ClaudeCodeReader) Source() string { return "claude-code" }

// Discover returns every <session-uuid>.jsonl directly under a project
// directory in ~/.claude/projects. It does NOT recurse into
// <session>/subagents/*.jsonl: those are sidechain (Task-tool) transcripts
// for a parent session, not standalone sessions (see claudecodeParseLines
// for how isSidechain is handled defensively too). A missing
// ~/.claude/projects is not an error.
func (ClaudeCodeReader) Discover(home string) ([]string, error) {
	root := filepath.Join(home, ".claude", "projects")
	projectDirs, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var paths []string
	for _, pd := range projectDirs {
		if !pd.IsDir() {
			continue
		}
		projDir := filepath.Join(root, pd.Name())
		entries, err := os.ReadDir(projDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if strings.HasSuffix(e.Name(), ".jsonl") {
				paths = append(paths, filepath.Join(projDir, e.Name()))
			}
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// claudeCodeRecord is the union of the JSONL line shapes we care about.
// Fields we don't use (version, permissionMode, origin, ...) are left out;
// unknown fields are ignored by encoding/json.
type claudeCodeRecord struct {
	Type        string `json:"type"`
	Timestamp   string `json:"timestamp"`
	SessionID   string `json:"sessionId"`
	Cwd         string `json:"cwd"`
	GitBranch   string `json:"gitBranch"`
	IsSidechain bool   `json:"isSidechain"`
	IsMeta      bool   `json:"isMeta"`
	AgentID     string `json:"agentId"`

	// summary/title records
	Summary     string `json:"summary"`
	CustomTitle string `json:"customTitle"`
	AITitle     string `json:"aiTitle"`

	Message *claudeCodeMessage `json:"message"`
}

type claudeCodeMessage struct {
	Role    string           `json:"role"`
	ID      string           `json:"id"`
	Model   string           `json:"model"`
	Content json.RawMessage  `json:"content"`
	Usage   *claudeCodeUsage `json:"usage"`
}

type claudeCodeUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
}

// claudeCodeContentBlock covers both the string and structured-block forms
// message.content can take.
type claudeCodeContentBlock struct {
	Type string `json:"type"`
}

// Read parses one Claude Code session transcript file. Malformed lines are
// skipped, not fatal: one bad line must not sink the whole session.
func (r ClaudeCodeReader) Read(path string) ([]Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []Event
	var firstHuman string
	var summaryTitle, customTitle, aiTitle string
	var cwd, branch, sessionID string
	seenCalls := map[string]bool{}
	var calls []ModelCall

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var rec claudeCodeRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			// Malformed line: skip it, keep reading the rest of the file.
			continue
		}
		if rec.SessionID != "" && sessionID == "" {
			sessionID = rec.SessionID
		}
		if rec.Cwd != "" && cwd == "" {
			cwd = rec.Cwd
		}
		if rec.GitBranch != "" && branch == "" {
			branch = rec.GitBranch
		}

		// Subagent / sidechain turns are not a standalone session; their
		// wall time is inside the parent's. We don't have a cheap way to
		// attribute their usage back to the parent from inside this file
		// (the parent lives in a sibling top-level .jsonl, correlated only
		// by a tool_use id we don't track here), so we skip them entirely
		// rather than double count or guess.
		if rec.IsSidechain || rec.AgentID != "" {
			continue
		}

		switch rec.Type {
		case "summary":
			if summaryTitle == "" {
				summaryTitle = rec.Summary
			}
		case "custom-title":
			customTitle = rec.CustomTitle
		case "ai-title":
			if aiTitle == "" {
				aiTitle = rec.AITitle
			}
		case "user":
			at, ok := claudeCodeParseTime(rec.Timestamp)
			if !ok || rec.Message == nil {
				continue
			}
			// isMeta marks content Claude Code injected itself (slash
			// command caveats, skill bodies, compaction summaries, hook
			// output, AGENTS.md/CLAUDE.md context) rather than something
			// the human typed: never Human.
			if rec.IsMeta {
				events = append(events, Event{At: at, Kind: Agent})
				continue
			}
			text, isToolResult := claudeCodeUserText(rec.Message.Content)
			if isToolResult {
				// A tool_result block: the agent's tool ran and the
				// result is arriving, not a human prompt.
				events = append(events, Event{At: at, Kind: Agent})
				continue
			}
			if strings.TrimSpace(text) == "" {
				continue
			}
			if firstHuman == "" {
				firstHuman = text
			}
			events = append(events, Event{At: at, Kind: Human})
		case "assistant":
			at, ok := claudeCodeParseTime(rec.Timestamp)
			if !ok || rec.Message == nil {
				continue
			}
			events = append(events, Event{At: at, Kind: Agent})
			// Claude Code streams one JSONL line per content block of the
			// same response; every line repeats the identical cumulative
			// usage for that message. Dedupe by message.id so we count a
			// response's tokens exactly once.
			if rec.Message.Usage == nil || rec.Message.ID == "" {
				continue
			}
			if seenCalls[rec.Message.ID] {
				continue
			}
			seenCalls[rec.Message.ID] = true
			u := rec.Message.Usage
			calls = append(calls, ModelCall{
				Model:      rec.Message.Model,
				Input:      u.InputTokens,
				Output:     u.OutputTokens,
				CacheRead:  u.CacheReadInputTokens,
				CacheWrite: u.CacheCreationInputTokens,
			})
		default:
			// system, mode, queue-operation, file-history-snapshot,
			// attachment, last-prompt and anything else: not a timestamped
			// human/agent timeline event we track.
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, nil
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].At.Before(events[j].At) })

	if sessionID == "" {
		sessionID = claudeCodeStem(path)
	}

	title := customTitle
	if title == "" {
		title = summaryTitle
	}
	if title == "" {
		title = aiTitle
	}
	if title == "" {
		title = firstHuman
	}
	title = claudeCodeSingleLine(title)
	title = claudeCodeTruncateRunes(title, 80)

	return []Session{{
		Source:     "claude-code",
		ExternalID: sessionID,
		Title:      title,
		Cwd:        cwd,
		GitBranch:  branch,
		Events:     events,
		Calls:      calls,
	}}, nil
}

// claudeCodeParseTime parses the RFC3339 timestamp Claude Code records on
// every line. An empty or malformed timestamp means the record cannot be
// anchored on the timeline, so the caller skips it.
func claudeCodeParseTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// claudeCodeUserText extracts the human-readable text from a user
// message's content, which is either a plain string or a list of content
// blocks. It reports isToolResult=true if any block is a tool_result,
// since a tool_result-bearing user record is the agent's tool finishing,
// never a human prompt.
func claudeCodeUserText(raw json.RawMessage) (text string, isToolResult bool) {
	if len(raw) == 0 {
		return "", false
	}
	// Plain string content: a real typed (or pasted) prompt, or an
	// injected string like "<local-command-caveat>...": the isMeta flag on
	// the record already filters those out before we get here.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, false
	}

	var blocks []json.RawMessage
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return "", false
	}
	var sb strings.Builder
	for _, b := range blocks {
		var block claudeCodeContentBlock
		if err := json.Unmarshal(b, &block); err != nil {
			continue
		}
		if block.Type == "tool_result" {
			isToolResult = true
			continue
		}
		if block.Type == "text" {
			var tb struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(b, &tb); err == nil {
				if sb.Len() > 0 {
					sb.WriteByte('\n')
				}
				sb.WriteString(tb.Text)
			}
		}
	}
	return sb.String(), isToolResult
}

// claudeCodeStem returns the session id fallback for a path whose
// SessionID field, for whatever reason, came back empty: the file's base
// name without extension. ExternalID must stay stable across re-reads of
// a growing file, and a file's own name never changes.
func claudeCodeStem(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func claudeCodeSingleLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	return s
}

func claudeCodeTruncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
