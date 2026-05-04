package cmd

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/ui"
)

const (
	repoURL     = "https://github.com/tolgazorlu/btrack"
	issueURL    = "https://github.com/tolgazorlu/btrack/issues/new"
	releaseURL  = "https://github.com/tolgazorlu/btrack/releases"
)

func openBrowser(url string) error {
	var err error
	switch runtime.GOOS {
	case "darwin":
		err = exec.Command("open", url).Start()
	case "windows":
		err = exec.Command("cmd", "/c", "start", url).Start()
	default:
		err = exec.Command("xdg-open", url).Start()
	}
	return err
}

var starCmd = &cobra.Command{
	Use:   "star",
	Short: "Open the btrack GitHub page",
	Long:  `Open the btrack repository on GitHub in your browser.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("\n  %s  %s\n\n",
			ui.StyleSuccess.Render("→"),
			ui.StyleHighlight.Render(repoURL),
		)
		if err := openBrowser(repoURL); err != nil {
			fmt.Printf("  %s\n\n", ui.StyleDimmed.Render("could not open browser — copy the URL above"))
		}
		return nil
	},
}

var issueCmd = &cobra.Command{
	Use:     "issue",
	Aliases: []string{"feedback", "report", "bug"},
	Short:   "Open a new GitHub issue (bug report or feedback)",
	Long: `Open a new issue on GitHub to report a bug or share feedback.

Aliases: feedback, report, bug

Examples:
  btrack issue
  btrack feedback
  btrack bug`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("\n  %s  %s\n\n",
			ui.StyleSuccess.Render("→"),
			ui.StyleHighlight.Render(issueURL),
		)
		if err := openBrowser(issueURL); err != nil {
			fmt.Printf("  %s\n\n", ui.StyleDimmed.Render("could not open browser — copy the URL above"))
		}
		return nil
	},
}

var releasesCmd = &cobra.Command{
	Use:   "releases",
	Short: "Open the btrack releases page",
	Long:  `Open the btrack releases page on GitHub to see the changelog.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("\n  %s  %s\n\n",
			ui.StyleSuccess.Render("→"),
			ui.StyleHighlight.Render(releaseURL),
		)
		if err := openBrowser(releaseURL); err != nil {
			fmt.Printf("  %s\n\n", ui.StyleDimmed.Render("could not open browser — copy the URL above"))
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(starCmd)
	rootCmd.AddCommand(issueCmd)
	rootCmd.AddCommand(releasesCmd)
}
