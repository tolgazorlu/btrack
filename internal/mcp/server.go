package mcp

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Version is the implementation version reported in the MCP handshake.
// Set by the cmd/mcp.go wrapper to match the btrack binary version.
var Version = "dev"

// Run starts the MCP stdio server and blocks until the transport closes
// or the context is cancelled. All logging goes to stderr — stdout is
// reserved for the JSON-RPC protocol and must stay clean.
func Run(ctx context.Context, deps Deps) error {
	if deps.Client == nil {
		return fmt.Errorf("mcp: daemon client is required")
	}
	if deps.Store == nil {
		return fmt.Errorf("mcp: db store is required")
	}

	// Route the SDK's internal logging away from stdout.
	log.SetOutput(stderrOnly{})

	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "btrack",
			Version: Version,
		},
		nil,
	)

	for _, t := range Tools(deps) {
		t.Register(server)
	}

	return server.Run(ctx, &mcp.StdioTransport{})
}

// stderrOnly is an io.Writer that forwards everything to os.Stderr. Used to
// redirect any stray log output the SDK or downstream packages emit, so we
// never accidentally corrupt the stdio JSON-RPC stream on stdout.
type stderrOnly struct{}

func (stderrOnly) Write(p []byte) (int, error) { return os.Stderr.Write(p) }

// Verify io.Writer interface at compile time.
var _ io.Writer = stderrOnly{}
