package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var shellCmd = &cobra.Command{
	Use:   "shell <zsh|bash|fish>",
	Short: "Print shell prompt integration snippet",
	Long: `Print the ready-to-paste snippet for your shell.

  btrack shell zsh    — Zsh (RPROMPT)
  btrack shell bash   — Bash (PS1)
  btrack shell fish   — Fish (right prompt)`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"zsh", "bash", "fish"},
	RunE: func(cmd *cobra.Command, args []string) error {
		shell := args[0]

		ui.Blank()
		switch shell {
		case "zsh":
			fmt.Println(ui.StyleHighlight.Render("  # Add to ~/.zshrc"))
			fmt.Println(ui.StyleDimmed.Render(`  btrack_prompt() { btrack prompt 2>/dev/null; }
  RPROMPT='$(btrack_prompt)'`))
			fmt.Println()
			ui.Hint("session appears on the right side of your prompt")
			ui.Hint("or replace RPROMPT with PROMPT to put it on the left")
		case "bash":
			fmt.Println(ui.StyleHighlight.Render("  # Add to ~/.bashrc"))
			fmt.Println(ui.StyleDimmed.Render(`  btrack_prompt() {
    local s=$(btrack prompt 2>/dev/null)
    [ -n "$s" ] && echo " $s"
  }
  PS1='\u@\h \w$(btrack_prompt) \$ '`))
			fmt.Println()
			ui.Hint("session is appended before the $ in your prompt")
		case "fish":
			fmt.Println(ui.StyleHighlight.Render("  # Add to ~/.config/fish/functions/fish_right_prompt.fish"))
			fmt.Println(ui.StyleDimmed.Render(`  function fish_right_prompt
    btrack prompt 2>/dev/null
  end`))
			fmt.Println()
			ui.Hint("session appears on the right side of your prompt")
		default:
			return fmt.Errorf("unknown shell %q — choose zsh, bash, or fish", shell)
		}

		ui.Blank()
		ui.Hint("use 'btrack prompt --format starship' for Starship integration")
		ui.Blank()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(shellCmd)
}
