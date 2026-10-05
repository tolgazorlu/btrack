package cmd

import (
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/db"
	"github.com/tolgazorlu/btrack/internal/mcp"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Serve btrack to AI agents over MCP (stdio)",
	Long: `Run btrack as a Model Context Protocol server on stdin/stdout, so an AI
agent can read and write your ledger directly.

Tools: recap, log_work, list_sessions, unlogged_days, sync, projects.

Register it with your agent, for example:
  claude mcp add btrack -- btrack mcp
  codex mcp add btrack -- btrack mcp

or in any MCP client config:
  {"mcpServers": {"btrack": {"command": "btrack", "args": ["mcp"]}}}

The database is opened per tool call, so the server never locks out the
CLI or the timer daemon.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		home, _ := os.UserHomeDir()
		srv := &mcp.Server{
			Name:         "btrack",
			Version:      displayVersion(Version),
			Instructions: mcp.Instructions,
			Tools: mcp.Tools(mcp.Env{
				Open: func() (db.Store, error) {
					cfg, err := config.Load()
					if err != nil {
						return nil, err
					}
					return db.Open(cfg)
				},
				Home: home,
				Now:  time.Now,
			}),
		}
		return srv.Serve(os.Stdin, os.Stdout)
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
