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

	"status":   {"status"},
	"w":        {"status"},
	"history":  {"history"},
	"h":        {"history"},
	"stats":    {"stats"},
	"projects": {"projects"},
	"search":   {"search"},
	"tag":      {"tag"},

	"config":  {"config"},
	"export":  {"export"},
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
	"status":   "live session view",
	"w":        "live session view",
	"history":  "view session history",
	"h":        "view session history",
	"stats":    "productivity snapshot",
	"projects": "list all projects",
	"search":   "search sessions",
	"tag":      "filter history by tag",
	"config":   "view or change settings",
	"export":   "export sessions to CSV/JSON",
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
	ui.Hint(`examples:  /start "fix bug" -p myapp   ·   /stop -m "fixed it #bugfix"`)
	ui.Blank()
}
