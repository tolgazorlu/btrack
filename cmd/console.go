package cmd

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/shlex"
	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/ui"
)

// runConsole is the interactive REPL invoked when the user runs `btrack`
// with no args. It renders the Gemini-style banner, captures one line of
// input at a time via Bubble Tea, dispatches it through cobra, and loops.
//
// Input syntax:
//   - bare command:   s "fix bug" -p myapp
//   - @-quick action: @create-session "fix bug" -p myapp
//   - slash command:  /help, /exit, /clear
//   - free text:      treated as AI chat (when configured)
func runConsole() error {
	tagline := "time tracker for developers"

	// Print the banner once, then loop one input at a time.
	hint := ""
	for {
		model := ui.NewConsoleModel(tagline, Version, hint)
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

		// @-actions are dispatched in console.go's @-handler (added in a
		// follow-up commit). For now, anything starting with @ falls
		// through to a friendly hint.
		if strings.HasPrefix(args[0], "@") {
			hint = "@-actions arrive in v2 — try `s \"task\"` for now"
			continue
		}

		// Dispatch via cobra. Reset args, run, surface any error to the user.
		if err := dispatch(args); err != nil {
			hint = ui.StyleError.Render(" error ") + " " + err.Error()
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

// handleSlash processes /-prefixed meta commands. Returns (quit, hint).
func handleSlash(input string) (bool, string) {
	cmd := strings.ToLower(strings.TrimPrefix(input, "/"))
	switch strings.SplitN(cmd, " ", 2)[0] {
	case "exit", "quit", "q":
		ui.Blank()
		ui.Hint("bye — see you next session")
		ui.Blank()
		return true, ""
	case "help", "?":
		_ = rootCmd.Help()
		return false, ""
	case "clear", "cls":
		// Bubble Tea handles its own draw; the next iteration paints fresh.
		fmt.Fprint(ui.Out, "\033[H\033[2J")
		return false, ""
	default:
		return false, "unknown slash command: /" + cmd
	}
}
