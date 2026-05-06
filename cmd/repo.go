package cmd

import (
	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Open btrack project links in your browser",
	Long: `Open btrack project pages in your browser.

  btrack repo            show available links
  btrack repo star       open the GitHub repository
  btrack repo issue      open a new issue (bug / feedback)
  btrack repo releases   open the releases / changelog page`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Header("links", "")
		ui.KV("repo", ui.StyleHighlight.Render(repoURL))
		ui.KV("issues", ui.StyleHighlight.Render(issueURL))
		ui.KV("releases", ui.StyleHighlight.Render(releaseURL))
		ui.Footer("btrack repo star | issue | releases  to open in browser")
		return nil
	},
}

func openLink(url string) {
	ui.Blank()
	ui.Sign(ui.StyleSuccess.Render(ui.Sym.Arrow), ui.StyleHighlight.Render(url))
	if err := openBrowser(url); err != nil {
		ui.Hint("could not open browser — copy the URL above")
	}
	ui.Blank()
}

var repoStarCmd = &cobra.Command{
	Use:   "star",
	Short: "Open the btrack GitHub repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		openLink(repoURL)
		return nil
	},
}

var repoIssueCmd = &cobra.Command{
	Use:     "issue",
	Aliases: []string{"feedback", "bug"},
	Short:   "Open a new issue (bug report or feedback)",
	RunE: func(cmd *cobra.Command, args []string) error {
		openLink(issueURL)
		return nil
	},
}

var repoReleasesCmd = &cobra.Command{
	Use:   "releases",
	Short: "Open the releases / changelog page",
	RunE: func(cmd *cobra.Command, args []string) error {
		openLink(releaseURL)
		return nil
	},
}

func init() {
	repoCmd.AddCommand(repoStarCmd, repoIssueCmd, repoReleasesCmd)
	rootCmd.AddCommand(repoCmd)
}
