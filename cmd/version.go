package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/ui"
)

// Version is set at build time via ldflags: -X main.Version=v1.2.3
var Version = "beta"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the btrack version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(ui.Indent +
			ui.StyleTitle.Render("btrack") + " " +
			ui.StyleHighlight.Render(Version))
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.Version = Version
}
