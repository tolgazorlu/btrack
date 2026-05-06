package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/daemon"
	"github.com/tolgazorlu/btrack/internal/db"
	mcpserver "github.com/tolgazorlu/btrack/internal/mcp"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run btrack as a Model Context Protocol stdio server",
	Long: `Expose btrack as an MCP server over stdio so AI assistants
(Claude Code, Cursor, Gemini CLI, Claude Desktop) can read your tracked
sessions and start/stop/switch tasks during a chat.

This command speaks JSON-RPC over stdin/stdout and is meant to be launched
by an MCP client, not run interactively. Logs go to stderr only.

Register with Claude Code:
  claude mcp add btrack <path-to-btrack-binary> -- mcp

Register with Cursor or Gemini CLI: add an entry like
  {
    "mcpServers": {
      "btrack": {
        "command": "<path-to-btrack-binary>",
        "args": ["mcp"]
      }
    }
  }

Tools exposed:
  btrack_status, btrack_start, btrack_stop, btrack_switch, btrack_resume,
  btrack_log_note, btrack_history, btrack_search, btrack_list_projects,
  btrack_get_session`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		store, err := db.Open(cfg)
		if err != nil {
			return err
		}
		defer store.Close()

		mcpserver.Version = Version

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Forward Ctrl-C / SIGTERM to the server's context so it can shut down
		// cleanly. MCP clients normally close stdio to end the session, which
		// the SDK already handles, so this is just belt-and-suspenders.
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigCh
			cancel()
		}()

		return mcpserver.Run(ctx, mcpserver.Deps{
			Client: daemon.NewClient(),
			Store:  store,
		})
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
