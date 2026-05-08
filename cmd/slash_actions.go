package cmd

import (
	"sort"
	"strings"

	"github.com/tolgazorlu/btrack/internal/ui"
)

// slashAlias maps a /command name to the cobra command path it expands to.
// Every action here is reachable as "/name [args…]" inside the interactive
// console. Short single-letter aliases are included so muscle-memory works.
var slashAlias = map[string][]string{
	// ── session ──────────────────────────────────────────────────────────────
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

	// ── views ─────────────────────────────────────────────────────────────────
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

	// ── ai ────────────────────────────────────────────────────────────────────
	"standup":  {"standup"},
	"su":       {"standup"},
	"insights": {"ai", "ins"},
	"ins":      {"ai", "ins"},
	"chat":     {"ai"},
	"setup":    {"ai", "setup"},

	// ── system ────────────────────────────────────────────────────────────────
	"config":  {"config"},
	"export":  {"export"},
	"invoice": {"invoice"},
	"init":    {"init"},
}

// slashActionHints provides a human-readable one-liner for each canonical
// command shown in the autocomplete dropdown. Keys are the primary (longest)
// name; aliases deliberately share the same hint so the list is informative.
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

// expandSlashAction converts ["/s", "fix bug", "-p", "myapp"]
// into ["start", "fix bug", "-p", "myapp"].
//
// The leading "/" must already be stripped from args[0] by the caller.
// Returns (args, true) on a match, (nil, false) when unknown.
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

// slashSuggestions returns the slash-action aliases as ui.Suggestion rows for
// the console autocomplete dropdown. Meta-commands (/help, /exit, /clear,
// /tools, /mcp) are added separately in runConsole so they appear in the list.
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

// printSlashActions renders a help table of all known slash commands.
// Used by /help inside the console.
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
