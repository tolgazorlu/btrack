package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/ui"
)

// checkWelcome prints a welcome or upgrade banner the first time a new version runs.
func checkWelcome() {
	versionFile := filepath.Join(config.ConfigDir(), ".version")
	data, _ := os.ReadFile(versionFile)
	seen := strings.TrimSpace(string(data))

	if seen == Version {
		return
	}

	isUpgrade := seen != "" && seen != "dev"
	printWelcome(isUpgrade, seen)
	os.WriteFile(versionFile, []byte(Version), 0644) //nolint:errcheck
}

func printWelcome(isUpgrade bool, prevVersion string) {
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#3B4261")).
		Padding(1, 3)

	c := func(command, desc string) string {
		return fmt.Sprintf("  %-40s%s",
			ui.StyleHighlight.Render(command),
			ui.StyleDimmed.Render(desc),
		)
	}
	sep := ui.StyleDimmed.Render(strings.Repeat("─", 52))

	var header string
	if isUpgrade {
		header = fmt.Sprintf("%s  %s  %s",
			ui.StyleTitle.Render("btrack"),
			ui.StyleSuccess.Render("updated to "+Version),
			ui.StyleDimmed.Render("(was "+prevVersion+")"),
		)
	} else {
		header = fmt.Sprintf("%s  %s",
			ui.StyleTitle.Render("btrack"),
			ui.StyleSuccess.Render(Version+" installed"),
		)
	}

	body := header + "\n" +
		ui.StyleDimmed.Render("  time tracker for developers") + "\n\n" +
		"  " + sep + "\n\n" +
		"  " + ui.StyleHighlight.Render("QUICK START") + "\n\n" +
		c(`btrack start "fix login bug"`, "start tracking a task") + "\n" +
		c(`btrack note "found the issue"`, "add a checkpoint note") + "\n" +
		c(`btrack stop -m "fixed it #bugfix"`, "stop and save") + "\n\n" +
		"  " + sep + "\n\n" +
		"  " + ui.StyleHighlight.Render("REVIEW") + "\n\n" +
		c("btrack day", "today's sessions as a tree") + "\n" +
		c("btrack day yesterday", "yesterday's sessions") + "\n" +
		c("btrack history", "all past sessions in a table") + "\n" +
		c("btrack status", "live view of active session") + "\n\n" +
		"  " + sep + "\n\n" +
		"  " + ui.StyleHighlight.Render("AI  ") +
		ui.StyleDimmed.Render("(optional — needs an API key)") + "\n\n" +
		c("btrack ai setup", "configure OpenAI / Claude / Gemini") + "\n" +
		c("btrack ai summarize", "standup summary from today's work") + "\n" +
		c("btrack ai insights", "weekly stats + AI analysis") + "\n\n" +
		"  " + sep + "\n\n" +
		"  " + ui.StyleHighlight.Render("CONFIG") + "\n\n" +
		c("btrack config hours 8", "set daily work target (default: 8h)") + "\n" +
		c("btrack config", "show all current settings") + "\n\n" +
		"  " + sep + "\n\n" +
		"  " + ui.StyleDimmed.Render("run  btrack --help  for the full command reference")

	fmt.Println()
	fmt.Println(border.Render(body))
	fmt.Println()
}
