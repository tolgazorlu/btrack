package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/tolgazorlu/btrack/internal/ai"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/db"
	"github.com/tolgazorlu/btrack/internal/ui"
)

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

// runConsoleChat treats the raw input as an AI chat prompt: builds a
// short recent-sessions context, calls the configured provider, and
// prints the response inline so the next REPL iteration can repaint.
//
// If no AI provider is configured, prints a one-line setup hint and
// returns nil (the REPL keeps running).
func runConsoleChat(input string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.AI.ActiveKey() == "" {
		ui.Blank()
		ui.Hint("no AI key configured  ·  try `@setup` to add one (~30s)")
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
	sb.WriteString("You are an assistant inside the `btrack` developer time tracker. ")
	sb.WriteString("Be concise (under 120 words). Reply in plain prose, no markdown headings.\n\n")
	if len(sessions) > 0 {
		sb.WriteString("Recent sessions:\n")
		for _, s := range sessions {
			d := s.Duration()
			h := int(d.Hours())
			m := int(d.Minutes()) % 60
			var dur string
			if h > 0 {
				dur = fmt.Sprintf("%dh%02dm", h, m)
			} else {
				dur = fmt.Sprintf("%dm", m)
			}
			sb.WriteString(fmt.Sprintf("- [%s] %s (%s)\n",
				s.StartTime.Local().Format("Mon 15:04"),
				s.TaskName, dur))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("User: " + input + "\n")

	ui.Blank()
	ui.Dim("✦ asking " + provider.Name() + "…")

	resp, err := provider.Complete(context.Background(), sb.String())
	if err != nil {
		ui.Blank()
		ui.Warn("AI error: " + err.Error())
		ui.Blank()
		return nil
	}

	ui.Blank()
	fmt.Fprintln(ui.Out, ui.Indent+ui.StyleSuccess.Render(" ai")+"  "+strings.TrimSpace(resp))
	ui.Blank()
	return nil
}

// dispatchOrChat routes parsed args. If the first arg names a known
// cobra subcommand, dispatch normally; otherwise treat the entire raw
// input as an AI chat prompt.
func dispatchOrChat(args []string, raw string) error {
	if len(args) == 0 {
		return nil
	}
	if isKnownSubcommand(args[0]) {
		return dispatch(args)
	}
	return runConsoleChat(raw)
}

