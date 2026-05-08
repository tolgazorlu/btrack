package cmd

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/shlex"
	mcpserver "github.com/tolgazorlu/btrack/internal/mcp"
	"github.com/tolgazorlu/btrack/internal/ui"
)

func runConsole() error {
	tagline := "time tracker for developers"

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

		execErr := dispatchOrChat(args, input)
		if execErr != nil {
			hint = ui.StyleError.Render(" error ") + " " + execErr.Error()
			continue
		}
		hint = ""
	}
}

func dispatch(args []string) error {
	rootCmd.SetArgs(args)
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true

	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(ui.Out, ui.StyleError.Render(" panic ")+" "+fmt.Sprint(r))
		}
	}()

	if err := rootCmd.Execute(); err != nil {
		if errors.Is(err, ErrSilent) {
			return nil
		}
		return err
	}
	return nil
}

var ErrSilent = errors.New("silent")

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
		if looksLikeNaturalLanguage(expanded[1:]) {
			if err := runConsoleChat(input); err != nil {
				return false, ui.StyleError.Render(" error ") + " " + err.Error()
			}
			return false, ""
		}
		if dispErr := dispatch(expanded); dispErr != nil {
			return false, ui.StyleError.Render(" error ") + " " + dispErr.Error()
		}
		return false, ""
	}
}

func looksLikeNaturalLanguage(args []string) bool {
	nlWords := map[string]bool{
		"with": true, "about": true, "for": true, "in": true,
		"to": true, "and": true, "project": true, "task": true,
		"named": true, "called": true, "on": true,
	}
	for _, a := range args {
		if !strings.HasPrefix(a, "-") && nlWords[strings.ToLower(a)] {
			return true
		}
	}
	return false
}

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
