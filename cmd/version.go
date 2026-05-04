package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tolgaozgun/btrack/internal/ui"
)

// Version is set at build time via ldflags: -X main.Version=v1.2.3
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the btrack version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%s %s\n",
			ui.StyleTitle.Render("btrack"),
			ui.StyleSubtle.Render(Version),
		)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.Version = Version
}
