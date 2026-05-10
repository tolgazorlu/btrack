package cmd

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/ui"
)

// Version is set at build time via ldflags:
//   -X github.com/tolgazorlu/btrack/cmd.Version=v1.2.3
// When unset (e.g. `go install ...@v1.2.3`), it falls back to the
// module version embedded in the binary by the Go toolchain.
var Version = "dev"

func init() {
	if Version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			if v := info.Main.Version; v != "" && v != "(devel)" {
				Version = v
			}
		}
	}
	rootCmd.AddCommand(versionCmd)
	rootCmd.Version = Version
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the btrack version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(ui.Indent +
			ui.StyleTitle.Render("btrack") + " " +
			ui.StyleHighlight.Render(Version))
	},
}
