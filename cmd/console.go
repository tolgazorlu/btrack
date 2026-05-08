package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/shlex"
	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/daemon"
	"github.com/tolgazorlu/btrack/internal/db"
	mcpserver "github.com/tolgazorlu/btrack/internal/mcp"
	"github.com/tolgazorlu/btrack/internal/ui"
)

// runConsole is the interactive REPL invoked when the user runs `btrack`
// with no args. It renders the Gemini-style banner, captures one line of
// input at a time via Bubble Tea, dispatches it through cobra, and loops.
//
// Input syntax:
//   - bare command:   s "fix bug" -p myapp
//   - /slash action:  /start "fix bug" -p myapp
//   - slash meta:     /help, /exit, /clear
//   - free text:      treated as AI chat (when configured)
func runConsole() error {
	tagline := "time tracker for developers"

	// Banner shows only on the very first iteration; subsequent prompts
	// just render the input box so the screen doesn't reflow on every Enter.
	hint := ""
	first := true
	suggestions := slashSuggestions()
	for {
		model := ui.NewConsoleModel(tagline, Version, hint, first).
			WithSuggestions(suggestions)
		first = false
		p := tea.NewProgram(model)
		final, err := p.Run()
		if err != nil {
			return err
		}
		c, _ := final.(ui.ConsoleModel)
		if c.Aborted() && c.Value() == "" {
			ui.Blank()
			ui.Hint("bye — see you next session")
			ui.Blank()
			return nil
		}

		input := strings.TrimSpace(c.Value())
		if input == "" {
			continue
		}

		// Slash commands: /help, /exit, /clear.
		if strings.HasPrefix(input, "/") {
			done, h := handleSlash(input)
			hint = h
			if done {
				return nil
			}
			continue
		}

		args, err := shlex.Split(input)
		if err != nil {
			hint = "parse error: " + err.Error()
			continue
		}
		if len(args) == 0 {
			continue
		}

		// Dispatch: known command → cobra, unknown → AI chat.
		execErr := dispatchOrChat(args, input)
		if execErr != nil {
			hint = ui.StyleError.Render(" error ") + " " + execErr.Error()
			continue
		}
		hint = ""
	}
}

// dispatch invokes the rootCmd with the given args, isolated from os.Args.
func dispatch(args []string) error {
	// Always keep the rootCmd usable across iterations: clone-by-state.
	rootCmd.SetArgs(args)
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true

	defer func() {
		// Recover from any panic inside a subcommand so the REPL keeps running.
		if r := recover(); r != nil {
			fmt.Fprintln(ui.Out, ui.StyleError.Render(" panic ")+" "+fmt.Sprint(r))
		}
	}()

	if err := rootCmd.Execute(); err != nil {
		var ce *cobra.Command
		_ = ce
		if errors.Is(err, ErrSilent) {
			return nil
		}
		return err
	}
	return nil
}

// ErrSilent — sentinel for subcommands that handled their own messaging.
var ErrSilent = errors.New("silent")

// handleSlash processes /-prefixed commands. Returns (quit, hint).
// Meta-commands are handled inline; everything else is looked up in
// slashAlias and dispatched through cobra.
func handleSlash(input string) (bool, string) {
	raw := strings.TrimPrefix(input, "/")
	parts, err := shlex.Split(raw)
	if err != nil || len(parts) == 0 {
		return false, ""
	}
	name := strings.ToLower(parts[0])
	switch name {
	case "exit", "quit", "q":
		ui.Blank()
		ui.Hint("bye — see you next session")
		ui.Blank()
		return true, ""
	case "help", "?":
		_ = rootCmd.Help()
		printSlashActions()
		return false, ""
	case "clear", "cls":
		fmt.Fprint(ui.Out, "\033[H\033[2J")
		return false, ""
	case "tools":
		printToolCatalog()
		return false, ""
	case "mcp":
		handleMCPSlash(input)
		return false, ""
	default:
		expanded, ok := expandSlashAction(parts)
		if !ok {
			return false, "unknown command: /" + name + "  ·  /help to list"
		}
		if dispErr := dispatch(expanded); dispErr != nil {
			return false, ui.StyleError.Render(" error ") + " " + dispErr.Error()
		}
		return false, ""
	}
}

// printToolCatalog renders the registered MCP tools as a one-per-line list.
// Same surface AI clients see — handy for confirming what the daemon and
// store can do via the MCP bridge.
func printToolCatalog() {
	ui.Blank()
	ui.Section("mcp tools")
	for _, t := range mcpserver.Tools(mcpserver.Deps{}) {
		ui.Cmd(t.Name, t.Description)
	}
	ui.Blank()
	ui.Hint("expose to AI: `btrack mcp` (stdio MCP server)")
	ui.Blank()
}

// invokeMCPTool runs a registered MCP tool with JSON args against a freshly
// constructed Deps. Debug-only: gated behind BTRACK_DEBUG.
//
// Usage: @tool <name> [json-args]
func invokeMCPTool(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: /tools <name> [json-args]")
	}
	name := args[0]
	rawArgs := json.RawMessage(`{}`)
	if len(args) > 1 {
		joined := strings.TrimSpace(strings.Join(args[1:], " "))
		if joined != "" {
			rawArgs = json.RawMessage(joined)
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return "", err
	}
	store, err := db.Open(cfg)
	if err != nil {
		return "", err
	}
	defer store.Close()

	deps := mcpserver.Deps{Client: daemon.NewClient(), Store: store}
	for _, t := range mcpserver.Tools(deps) {
		if t.Name == name {
			result, err := t.Invoke(context.Background(), rawArgs)
			if err != nil {
				return "", err
			}
			body, mErr := json.MarshalIndent(result, "", "  ")
			if mErr != nil {
				return "", mErr
			}
			return string(body), nil
		}
	}
	return "", fmt.Errorf("unknown tool %q (try /tools)", name)
}
