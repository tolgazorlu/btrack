package cmd

import (
	"sort"
	"strings"

	"github.com/tolgazorlu/btrack/internal/ui"
)

var slashAlias = map[string][]string{
	"start":  {"start"},
	"s":      {"start"},
	"stop":   {"stop"},
	"x":      {"stop"},
	"switch": {"switch"},
	"sw":     {"switch"},
	"resume": {"resume"},
	"r":      {"resume"},
	"note":   {"note"},
	"n":      {"note"},
	"break":  {"break"},

	"status":   {"status"},
	"w":        {"status"},
	"history":  {"history"},
	"h":        {"history"},
	"stats":    {"stats"},
	"streak":   {"streak"},
	"projects": {"projects"},
	"shipped":  {"shipped"},
	"search":   {"search"},
	"tag":      {"tag"},

	"standup":  {"standup"},
	"su":       {"standup"},
	"insights": {"ai", "ins"},
	"ins":      {"ai", "ins"},
	"chat":     {"ai"},
	"setup":    {"ai", "setup"},

	"config":  {"config"},
	"export":  {"export"},
	"invoice": {"invoice"},
	"init":    {"init"},
}

var slashActionHints = map[string]string{
	"start":    "start a new tracking session",
	"s":        "start a new tracking session",
	"stop":     "stop the active session",
	"x":        "stop the active session",
	"switch":   "stop current and start a new task",
	"sw":       "stop current and start a new task",
	"resume":   "resume the last stopped session",
	"r":        "resume the last stopped session",
	"note":     "add a checkpoint note",
	"n":        "add a checkpoint note",
	"break":    "log a break session",
	"status":   "live session view",
	"w":        "live session view",
	"history":  "view session history",
	"h":        "view session history",
	"stats":    "productivity snapshot",
	"streak":   "working-day streak",
	"projects": "list all projects",
	"shipped":  "compare sessions vs git commits",
	"search":   "search sessions",
	"tag":      "filter history by tag",
	"standup":  "generate AI standup",
	"su":       "generate AI standup",
	"insights": "AI productivity analysis",
	"ins":      "AI productivity analysis",
	"chat":     "open AI chat",
	"setup":    "configure AI provider key",
	"config":   "view or change settings",
	"export":   "export sessions to CSV/JSON",
	"invoice":  "generate billing invoice",
	"init":     "create .btrack project file",
}

func expandSlashAction(args []string) ([]string, bool) {
	if len(args) == 0 {
		return nil, false
	}
	name := strings.ToLower(args[0])
	target, ok := slashAlias[name]
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(target)+len(args)-1)
	out = append(out, target...)
	out = append(out, args[1:]...)
	return out, true
}

func slashSuggestions() []ui.Suggestion {
	out := make([]ui.Suggestion, 0, len(slashAlias))
	for k, target := range slashAlias {
		hint, ok := slashActionHints[k]
		if !ok {
			hint = "→ btrack " + strings.Join(target, " ")
		}
		out = append(out, ui.Suggestion{
			Trigger: "/" + k,
			Hint:    hint,
		})
	}
	return out
}

func printSlashActions() {
	ui.Blank()
	ui.Section("/ commands")
	keys := make([]string, 0, len(slashAlias))
	for k := range slashAlias {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		hint := slashActionHints[k]
		if hint == "" {
			hint = "→ btrack " + strings.Join(slashAlias[k], " ")
		}
		ui.Cmd("/"+k, hint)
	}
	ui.Blank()
	ui.Hint(`examples:  /start "fix bug" -p myapp   ·   /stop -m "shipped #refactor"   ·   /su`)
	ui.Blank()
}
