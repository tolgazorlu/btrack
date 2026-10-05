// Codex reader: parses OpenAI Codex CLI/desktop rollout logs
// (~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl and, if present,
// ~/.codex/archived_sessions/**/rollout-*.jsonl) into ingest.Session values.
//
// Format notes (learned from real rollout files on this machine, all
// written by "Codex Desktop" / vscode source, cli_version 0.153-0.154):
//
//   - Every line is a JSON object {timestamp, type, payload, ...}. "type" is
//     one of: session_meta, turn_context, event_msg, response_item,
//     token_usage_record, world_state. Exactly one session_meta per file.
//   - session_meta.payload.id (== session_id) is the stable external id.
//   - turn_context.payload.model names the model in effect for the turns
//     that follow, until the next turn_context.
//   - event_msg.payload.type == "user_message" is a typed prompt. On this
//     machine's data that event is rare (only logged for certain entry
//     points); most real prompts instead show up as response_item entries
//     with role "user" (see below), so both are treated as Human sources.
//   - event_msg.payload.type == "item_completed" duplicates the content of
//     a response_item (wrapped as an "item"); it is skipped entirely to
//     avoid double-counting the same turn.
//   - event_msg.payload.type == "agent_message" is a status/error string
//     from the app shell (e.g. "OAuth session expired"), not real assistant
//     output; response_item already carries the real assistant text, so
//     agent_message is skipped.
//   - response_item.payload.role == "user": Codex attaches injected
//     material to the same role as real prompts (plugin lists, slash
//     command invocations, system reminders, background task
//     notifications, environment/app context), all wrapped starting with an
//     XML-ish tag such as <recommended_plugins>, <command-name>,
//     <system-reminder>, <task-notification>, <environment_context>,
//     <local-command-stdout>. A real typed prompt never starts that way
//     (checked against this machine's data), so: text that starts with
//     such a tag is skipped; anything else under role "user" is Human, even
//     when it also carries an automatically appended "# Files mentioned by
//     the user:" preamble around the actual request, since that preamble is
//     still part of what the user's turn produced.
//   - response_item.payload.role == "developer" is Codex's own injected app
//     context ("You are running inside the Codex (desktop) app...",
//     multi-agent mode instructions). Not Human, not Agent: skipped.
//   - response_item.payload.role == "assistant" (type "message"), plus
//     response_item types "reasoning", "function_call",
//     "function_call_output", "custom_tool_call", "custom_tool_call_output"
//     are all Agent: assistant text, tool calls, and tool results.
//   - Token usage: event_msg.payload.type == "token_count" carries
//     info.total_token_usage (cumulative since session start) AND
//     info.last_token_usage (the delta for the turn that just finished).
//     Using last_token_usage directly gives one ModelCall per turn with no
//     need to compute a delta. Real data shows the final token_count of a
//     session occasionally repeated verbatim (same last_token_usage values)
//     right before the session ends; consecutive identical last_token_usage
//     values are deduped (second one dropped) to avoid double-billing that
//     turn. Cached input is a subset of input per the task brief, so
//     Input = input_tokens - cached_input_tokens, CacheRead =
//     cached_input_tokens.
//   - Subagent/child turns: turn_context carries turn_id and root_turn_id;
//     on a child turn these would differ. No such turns were observed on
//     this machine (every turn_context has turn_id == root_turn_id), and
//     Codex does not write a separate rollout file for them: they live
//     inline in the same file as regular response_item/event_msg records.
//     Because this reader already turns one file into (at most) one
//     Session, a child turn's response_items and token_count usage are
//     automatically folded into the parent session's Events/Calls with no
//     special-casing needed; nothing is double-created or dropped.
package ingest

import (
	"bufio"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// CodexReader implements ingest.Reader for OpenAI Codex CLI/desktop logs.
type CodexReader struct{}

func (CodexReader) Source() string { return "codex" }

// Discover returns every rollout-*.jsonl file under
// ~/.codex/sessions/**/ and ~/.codex/archived_sessions/**/, if those
// directories exist. A missing directory is not an error.
func (CodexReader) Discover(home string) ([]string, error) {
	var paths []string
	for _, root := range []string{
		filepath.Join(home, ".codex", "sessions"),
		filepath.Join(home, ".codex", "archived_sessions"),
	} {
		if _, err := os.Stat(root); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			name := d.Name()
			if strings.HasPrefix(name, "rollout-") && strings.HasSuffix(name, ".jsonl") {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// codexLine is the shape of every rollout-*.jsonl line.
type codexLine struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type codexSessionMeta struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
	Git       *struct {
		Branch string `json:"branch"`
	} `json:"git"`
}

type codexTurnContext struct {
	Model string `json:"model"`
}

type codexEventMsg struct {
	Type    string          `json:"type"`
	Message string          `json:"message"`
	Info    *codexTokenInfo `json:"info"`
}

type codexTokenInfo struct {
	LastTokenUsage *codexUsage `json:"last_token_usage"`
}

type codexUsage struct {
	InputTokens           int64 `json:"input_tokens"`
	CachedInputTokens     int64 `json:"cached_input_tokens"`
	CacheWriteInputTokens int64 `json:"cache_write_input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
}

type codexResponseItem struct {
	Type    string              `json:"type"`
	Role    string              `json:"role"`
	Content []codexContentBlock `json:"content"`
}

type codexContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// codexInjectedTag matches the leading XML-ish tag Codex uses to mark
// injected content under role "user": <recommended_plugins>,
// <command-name>, <system-reminder>, <task-notification>,
// <environment_context>, <local-command-stdout>, etc.
var codexInjectedTag = regexp.MustCompile(`^<[a-zA-Z][a-zA-Z0-9_-]*>`)

func codexResponseItemText(blocks []codexContentBlock) string {
	var sb strings.Builder
	for _, b := range blocks {
		if b.Type == "input_text" || b.Type == "output_text" || b.Type == "text" {
			sb.WriteString(b.Text)
		}
	}
	return sb.String()
}

func codexIsInjectedUserText(text string) bool {
	return codexInjectedTag.MatchString(strings.TrimSpace(text))
}

func codexSingleLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	return s
}

func codexTruncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// Read parses one rollout-*.jsonl file into zero or one Session. Malformed
// lines are skipped rather than failing the whole file.
func (CodexReader) Read(path string) ([]Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var (
		externalID   string
		cwd          string
		branch       string
		currentModel string
		firstHuman   string
		events       []Event
		calls        []ModelCall
		lastUsage    *codexUsage
	)

	for scanner.Scan() {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		var line codexLine
		if err := json.Unmarshal([]byte(raw), &line); err != nil {
			continue // malformed line: skip, don't fail the file
		}
		at, errTime := time.Parse(time.RFC3339Nano, line.Timestamp)
		hasTime := errTime == nil

		switch line.Type {
		case "session_meta":
			var meta codexSessionMeta
			if err := json.Unmarshal(line.Payload, &meta); err != nil {
				continue
			}
			if externalID == "" {
				externalID = meta.ID
				if externalID == "" {
					externalID = meta.SessionID
				}
				cwd = meta.Cwd
				if meta.Git != nil {
					branch = meta.Git.Branch
				}
			}

		case "turn_context":
			var tc codexTurnContext
			if err := json.Unmarshal(line.Payload, &tc); err != nil {
				continue
			}
			if tc.Model != "" {
				currentModel = tc.Model
			}

		case "event_msg":
			if !hasTime {
				continue
			}
			var em codexEventMsg
			if err := json.Unmarshal(line.Payload, &em); err != nil {
				continue
			}
			switch em.Type {
			case "user_message":
				text := em.Message
				if firstHuman == "" && strings.TrimSpace(text) != "" {
					firstHuman = text
				}
				events = append(events, Event{At: at, Kind: Human})
			case "token_count":
				if em.Info == nil || em.Info.LastTokenUsage == nil {
					continue
				}
				u := em.Info.LastTokenUsage
				if lastUsage != nil && *lastUsage == *u {
					// Same usage repeated verbatim: the final turn's
					// token_count is sometimes re-emitted at session end.
					continue
				}
				lastUsage = u
				input := u.InputTokens - u.CachedInputTokens
				if input < 0 {
					input = 0
				}
				calls = append(calls, ModelCall{
					Model:      currentModel,
					Input:      input,
					Output:     u.OutputTokens,
					CacheRead:  u.CachedInputTokens,
					CacheWrite: u.CacheWriteInputTokens,
				})
			// item_completed duplicates response_item content and
			// agent_message is app-shell status text, not real output or a
			// human prompt: both intentionally ignored.
			default:
			}

		case "response_item":
			if !hasTime {
				continue
			}
			var ri codexResponseItem
			if err := json.Unmarshal(line.Payload, &ri); err != nil {
				continue
			}
			switch {
			case ri.Type == "message" && ri.Role == "user":
				text := codexResponseItemText(ri.Content)
				if codexIsInjectedUserText(text) {
					continue
				}
				if firstHuman == "" && strings.TrimSpace(text) != "" {
					firstHuman = text
				}
				events = append(events, Event{At: at, Kind: Human})
			case ri.Type == "message" && ri.Role == "assistant":
				events = append(events, Event{At: at, Kind: Agent})
			case ri.Type == "message" && ri.Role == "developer":
				// Injected app/system context: neither Human nor Agent.
			case ri.Type == "reasoning",
				ri.Type == "function_call",
				ri.Type == "function_call_output",
				ri.Type == "custom_tool_call",
				ri.Type == "custom_tool_call_output":
				events = append(events, Event{At: at, Kind: Agent})
			default:
				// Unknown item type (future-proofing): ignore rather than
				// guess who produced it.
			}

		default:
			// turn_context already handled above; token_usage_record and
			// world_state carry no per-event timeline or usage info we
			// need.
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(events) == 0 || externalID == "" {
		return nil, nil
	}

	sort.SliceStable(events, func(i, j int) bool { return events[i].At.Before(events[j].At) })

	title := codexTruncateRunes(codexSingleLine(firstHuman), 80)

	return []Session{{
		Source:     "codex",
		ExternalID: externalID,
		Title:      title,
		Cwd:        cwd,
		GitBranch:  branch,
		Events:     events,
		Calls:      calls,
	}}, nil
}
