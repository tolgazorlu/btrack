package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var welcomeCmd = &cobra.Command{
	Use:   "welcome",
	Short: "Show the welcome screen and quick-start guide",
	Long: `Print the btrack welcome screen — the same banner you saw on first run.
Useful when you want a one-page reminder of the most-used commands.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		printWelcome(false, "")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(welcomeCmd)
}

// welcomeSuppressed skips the auto-banner when stdout must stay clean
// (prompt, completion) or when the command immediately opens a fullscreen TUI.
func welcomeSuppressed(cmd *cobra.Command) bool {
	if cmd == nil || cmd == rootCmd {
		return true
	}
	path := cmd.CommandPath()
	if strings.Contains(path, " prompt") || strings.Contains(path, " completion") {
		return true
	}
	if cmd == welcomeCmd {
		return true // explicit invocation handles its own output
	}
	switch cmd {
	case startCmd, statusCmd, pomoCmd, aiCmd, aiSetupCmd:
		return true
	default:
		return false
	}
}

// checkWelcome auto-prints the banner the first time a new version runs.
func checkWelcome() {
	versionFile := filepath.Join(config.ConfigDir(), ".version")
	data, _ := os.ReadFile(filepath.Clean(versionFile))
	seen := strings.TrimSpace(string(data))

	if seen == Version {
		return
	}

	isUpgrade := seen != "" && seen != "dev"
	printWelcome(isUpgrade, seen)
	_ = os.WriteFile(filepath.Clean(versionFile), []byte(Version), 0600)
}

// printWelcome renders the welcome / upgrade banner.
// First-run = isUpgrade false. Upgrade = isUpgrade true with prevVersion set.
func printWelcome(isUpgrade bool, prevVersion string) {
	ui.Blank()

	// Title bar — single line, no box.
	titleLine := ui.Indent + ui.StyleTitle.Render("btrack")
	switch {
	case isUpgrade:
		titleLine += "  " + ui.StyleSuccess.Render("updated → "+Version)
		titleLine += "  " + ui.StyleDimmed.Render("(was "+prevVersion+")")
	case Version == "dev":
		titleLine += "  " + ui.StyleDimmed.Render("dev build")
	default:
		titleLine += "  " + ui.StyleSuccess.Render(Version+" installed")
	}
	fmt.Println(titleLine)
	fmt.Println(ui.Indent + ui.StyleDimmed.Render("time tracker for developers"))
	ui.Rule()
	ui.Blank()

	ui.Section("daily")
	ui.Cmd(`btrack s "fix login bug"`, "start a session")
	ui.Cmd(`btrack n "found the issue"`, "add a note")
	ui.Cmd(`btrack x -m "fixed it #bugfix"`, "stop with a message")
	ui.Cmd(`btrack r`, "resume last session")
	ui.Blank()

	ui.Section("review")
	ui.Cmd("btrack w", "live status (alias of `status`)")
	ui.Cmd("btrack d", "today as a tree")
	ui.Cmd("btrack h -w", "this week")
	ui.Cmd("btrack stats", "quick snapshot")
	ui.Cmd("btrack shipped", "what landed in git during your sessions")
	ui.Blank()

	ui.Section("ai · optional, needs an API key")
	ui.Cmd("btrack ai setup", "configure OpenAI / Claude / Gemini")
	ui.Cmd("btrack ai sum", "standup summary from today")
	ui.Cmd("btrack ai ins", "weekly stats + AI analysis")
	ui.Blank()

	ui.Section("setup")
	ui.Cmd("btrack init", "create a .btrack project file here")
	ui.Cmd("btrack config hours 8", "set daily target")
	ui.Cmd("btrack config", "show all settings")
	ui.Blank()

	ui.Rule()
	ui.Hint("run  btrack --help  for the full reference")
	ui.Blank()
}
