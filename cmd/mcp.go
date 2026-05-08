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
	Short: "Run btrack as a Model Context Protocol server (stdio or HTTP)",
	Long: `Expose btrack as an MCP server so AI assistants (Claude Code, Cursor,
Gemini CLI, Claude Desktop) can read your tracked sessions and start, stop,
or switch tasks during a chat.

Two transports are supported:

  stdio (default)
      Launched per-client by the MCP client itself. Speaks JSON-RPC over
      stdin/stdout. Logs go to stderr only.

  http
      Long-lived Streamable HTTP server bound to localhost. Useful when
      the stdio launch isn't working (PATH issues, sandboxing, etc.) or
      when you want a single shared server that multiple AI clients
      connect to. Path is /mcp.

Usage:
  btrack mcp                     stdio
  btrack mcp --http              HTTP on 127.0.0.1:8765
  btrack mcp --http :9000        HTTP on 127.0.0.1:9000
  btrack mcp --http 0.0.0.0:9000 HTTP on all interfaces (use with care)

Register stdio with Claude Code:
  claude mcp add btrack -- btrack mcp

Register HTTP with Claude Code:
  claude mcp add --transport http btrack http://127.0.0.1:8765/mcp

Or in any MCP config file:
  {
    "mcpServers": {
      "btrack": { "command": "btrack", "args": ["mcp"] }
    }
  }

Tools exposed:
  btrack_status, btrack_start, btrack_stop, btrack_switch, btrack_resume,
  btrack_log_note, btrack_history, btrack_search, btrack_list_projects,
  btrack_get_session`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		httpFlag := cmd.Flags().Lookup("http")

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

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigCh
			cancel()
		}()

		deps := mcpserver.Deps{
			Client: daemon.NewClient(),
			Store:  store,
		}

		if httpFlag != nil && httpFlag.Changed {
			addr, _ := cmd.Flags().GetString("http")
			return mcpserver.RunHTTP(ctx, addr, deps)
		}
		return mcpserver.Run(ctx, deps)
	},
}

func init() {
	mcpCmd.Flags().String("http", "", "run as Streamable HTTP server on this address (default 127.0.0.1:8765 if no value)")
	mcpCmd.Flags().Lookup("http").NoOptDefVal = "127.0.0.1:8765"
	rootCmd.AddCommand(mcpCmd)
}
