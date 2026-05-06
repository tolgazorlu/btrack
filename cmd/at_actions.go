package cmd

import (
	"sort"
	"strings"

	"github.com/tolgazorlu/btrack/internal/ui"
)

// atAlias maps an @-quick-action to the cobra command path it expands to.
// Keep this short and curated — @-actions are the muscle-memory shortcuts
// you reach for inside the interactive console, not a full command list.
var atAlias = map[string][]string{
	"create-session": {"start"},
	"new":            {"start"},
	"start":          {"start"},
	"s":              {"start"},

	"stop": {"stop"},
	"x":    {"stop"},

	"resume": {"resume"},
	"r":      {"resume"},

	"switch": {"switch"},
	"sw":     {"switch"},

	"note": {"note"},
	"n":    {"note"},

	"break": {"break"},

	"status": {"status"},
	"w":      {"status"},

	"history": {"history"},
	"h":       {"history"},

	"day":      {"day"},
	"week":     {"week"},
	"stats":    {"stats"},
	"streak":   {"streak"},
	"projects": {"projects"},
	"shipped":  {"shipped"},
	"recap":    {"recap"},
	"search":   {"search"},
	"tag":      {"tag"},

	"ai":      {"ai"},
	"chat":    {"ai"},
	"summary": {"ai", "sum"},
	"sum":     {"ai", "sum"},
	"insights": {"ai", "ins"},
	"ins":      {"ai", "ins"},
	"setup":    {"ai", "setup"},

	"config": {"config"},
	"export": {"export"},
	"init":   {"init"},

	"clear": {"clear"},
	"cls":   {"clear"},
}

// expandAtAction converts ["@create-session", "fix bug", "-p", "myapp"]
// into ["start", "fix bug", "-p", "myapp"].
//
// Returns (args, true) on a match, (nil, false) when the action is unknown.
func expandAtAction(args []string) ([]string, bool) {
	if len(args) == 0 || !strings.HasPrefix(args[0], "@") {
		return nil, false
	}
	name := strings.TrimPrefix(args[0], "@")
	target, ok := atAlias[strings.ToLower(name)]
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(target)+len(args)-1)
	out = append(out, target...)
	out = append(out, args[1:]...)
	return out, true
}

// atSuggestions returns the @-aliases as ui.Suggestion rows for the
// console autocomplete dropdown.
func atSuggestions() []ui.Suggestion {
	out := make([]ui.Suggestion, 0, len(atAlias))
	for k, target := range atAlias {
		out = append(out, ui.Suggestion{
			Trigger: "@" + k,
			Hint:    "→ btrack " + strings.Join(target, " "),
		})
	}
	return out
}

// printAtActions renders a help table of all known @-quick-actions.
// Used by `/help` inside the console and by `btrack at` (future).
func printAtActions() {
	ui.Blank()
	ui.Section("@-quick actions")
	keys := make([]string, 0, len(atAlias))
	for k := range atAlias {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		target := strings.Join(atAlias[k], " ")
		ui.Cmd("@"+k, "→ btrack "+target)
	}
	ui.Blank()
	ui.Hint("examples:  @create-session \"fix bug\" -p myapp   ·   @stop -m \"shipped #refactor\"")
	ui.Blank()
}
