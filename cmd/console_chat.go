package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tolgazorlu/btrack/internal/ai"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/db"
	"github.com/tolgazorlu/btrack/internal/ui"
)

// intentResp is the structured JSON the AI returns when routing input.
type intentResp struct {
	Exec []string `json:"exec,omitempty"` // command args to dispatch
	Chat string   `json:"chat,omitempty"` // plain-text reply
}

const intentSystemPrompt = `You are the command parser for btrack, a developer time tracker.
Given any user input, decide: execute a btrack command, or reply in chat.

Available commands (exact syntax):
  start <task-name> [-p <project>]   — start tracking a task
  stop [-m <message>]                — stop the current session
  switch <task-name> [-p <project>]  — stop current, start new task
  note <text>                        — add a note to the current session
  status                             — show live session view
  history                            — show session history
  stats [--days N]                   — productivity snapshot
  standup [--days N]                 — generate AI standup
  search <query>                     — search sessions
  projects                           — list all projects

Rules:
- Extract structured params from natural language (e.g. "with project X" → -p X)
- task-name must be a concise quoted label, strip filler words like "about", "for", "on"
- Respond with JSON ONLY — no prose, no markdown fences

Command intent: {"exec": ["command", "arg", "--flag", "value"]}
Chat/question:  {"chat": "concise plain-prose reply, max 100 words"}`

// runConsoleChat handles free-text and natural-language slash commands.
// It uses AI to decide whether to execute a btrack command or reply in chat.
func runConsoleChat(input string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.AI.ActiveKey() == "" {
		ui.Blank()
		ui.Hint("no AI key configured  ·  try `/setup` to add one (~30s)")
		ui.Blank()
		return nil
	}

	provider, err := ai.NewProvider(cfg)
	if err != nil {
		return err
	}

	store, err := db.Open(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	sessions, _ := store.GetRecentSessions(20)

	var sb strings.Builder
	sb.WriteString(intentSystemPrompt)
	if len(sessions) > 0 {
		sb.WriteString("\n\nRecent sessions (context only):\n")
		for _, s := range sessions {
			d := s.Duration()
			h, m := int(d.Hours()), int(d.Minutes())%60
			var dur string
			if h > 0 {
				dur = fmt.Sprintf("%dh%02dm", h, m)
			} else {
				dur = fmt.Sprintf("%dm", m)
			}
			sb.WriteString(fmt.Sprintf("- [%s] %s (%s)\n",
				s.StartTime.Local().Format("Mon 15:04"), s.TaskName, dur))
		}
	}
	sb.WriteString("\n\nUser input: " + input)

	ui.Blank()
	ui.Dim("✦ thinking…")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	raw, err := provider.Complete(ctx, sb.String())
	if err != nil {
		ui.Blank()
		ui.Warn("AI error: " + err.Error())
		ui.Blank()
		return nil
	}

	resp, chatText := parseIntentResp(raw)
	if resp != nil && len(resp.Exec) > 0 {
		ui.Blank()
		ui.Dim("→ " + strings.Join(resp.Exec, " "))
		return dispatch(resp.Exec)
	}

	ui.Blank()
	if chatText == "" {
		chatText = strings.TrimSpace(raw)
	}
	fmt.Fprintln(ui.Out, ui.Indent+ui.StyleSuccess.Render(" ai")+"  "+chatText)
	ui.Blank()
	return nil
}

// parseIntentResp decodes the AI's JSON response, stripping markdown fences.
// Returns (resp, chatText): resp is non-nil on valid JSON, chatText is the
// fallback plain-text reply when JSON parsing fails.
func parseIntentResp(raw string) (*intentResp, string) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		if i := strings.Index(raw, "\n"); i >= 0 {
			raw = raw[i+1:]
		}
		if i := strings.LastIndex(raw, "```"); i >= 0 {
			raw = strings.TrimSpace(raw[:i])
		}
	}
	var r intentResp
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return nil, strings.TrimSpace(raw)
	}
	return &r, r.Chat
}

// isKnownSubcommand reports whether name matches a top-level rootCmd
// subcommand, an alias, or "help".
func isKnownSubcommand(name string) bool {
	if name == "help" {
		return true
	}
	for _, c := range rootCmd.Commands() {
		if c.Name() == name {
			return true
		}
		for _, a := range c.Aliases {
			if a == name {
				return true
			}
		}
	}
	return false
}

// dispatchOrChat routes parsed args. Known cobra subcommands with clean
// CLI args dispatch directly; natural-language args and unknown inputs
// go through the AI intent router.
func dispatchOrChat(args []string, raw string) error {
	if len(args) == 0 {
		return nil
	}
	if isKnownSubcommand(args[0]) && !looksLikeNaturalLanguage(args[1:]) {
		return dispatch(args)
	}
	return runConsoleChat(raw)
}
