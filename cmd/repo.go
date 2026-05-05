package cmd

import (
	"fmt"

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
		sep := ui.StyleDimmed.Render("  ─────────────────────────────────")
		fmt.Println()
		fmt.Printf("  %s  %s\n", ui.StyleTitle.Render("btrack"), ui.StyleHighlight.Render("project links"))
		fmt.Println(sep)
		fmt.Printf("  %s  %s\n", ui.StyleDimmed.Render("repo      "), ui.StyleSubtle.Render(repoURL))
		fmt.Printf("  %s  %s\n", ui.StyleDimmed.Render("issues    "), ui.StyleSubtle.Render(issueURL))
		fmt.Printf("  %s  %s\n", ui.StyleDimmed.Render("releases  "), ui.StyleSubtle.Render(releaseURL))
		fmt.Println(sep)
		fmt.Printf("  %s\n\n", ui.StyleDimmed.Render("btrack repo star | issue | releases  to open in browser"))
		return nil
	},
}

var repoStarCmd = &cobra.Command{
	Use:   "star",
	Short: "Open the btrack GitHub repository",
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

var repoIssueCmd = &cobra.Command{
	Use:     "issue",
	Aliases: []string{"feedback", "bug"},
	Short:   "Open a new issue (bug report or feedback)",
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

var repoReleasesCmd = &cobra.Command{
	Use:   "releases",
	Short: "Open the releases / changelog page",
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
	repoCmd.AddCommand(repoStarCmd, repoIssueCmd, repoReleasesCmd)
	rootCmd.AddCommand(repoCmd)
}
