package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Version is the implementation version reported in the MCP handshake.
// Set by the cmd/mcp.go wrapper to match the btrack binary version.
var Version = "dev"

// Run starts the MCP stdio server and blocks until the transport closes
// or the context is cancelled. All logging goes to stderr — stdout is
// reserved for the JSON-RPC protocol and must stay clean.
func Run(ctx context.Context, deps Deps) error {
	server, err := buildServer(deps)
	if err != nil {
		return err
	}
	// Route the SDK's internal logging away from stdout.
	log.SetOutput(stderrOnly{})
	return server.Run(ctx, &mcp.StdioTransport{})
}

// RunHTTP starts the MCP server over Streamable HTTP at the given address.
// Use this when you want a long-lived btrack MCP server that any HTTP-aware
// MCP client (or curl, for debugging) can talk to.
//
// addr accepts the same format as net.Listen ("host:port"). For local-only
// use, prefer "127.0.0.1:8765" — the helper NormalizeHTTPAddr applies that
// default to bare port specs like ":8765" so you don't accidentally bind
// to 0.0.0.0. The path is /mcp.
func RunHTTP(ctx context.Context, addr string, deps Deps) error {
	server, err := buildServer(deps)
	if err != nil {
		return err
	}
	addr = NormalizeHTTPAddr(addr)

	handler := mcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *mcp.Server { return server },
		nil,
	)

	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	mux.Handle("/mcp/", handler)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	fmt.Fprintf(os.Stderr, "btrack mcp http listening on %s (path: /mcp)\n", ln.Addr())

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// NormalizeHTTPAddr returns a sensible bind address. Bare ports (":8765" or
// "8765") get a localhost host prefixed so we never bind to 0.0.0.0 by
// accident; explicit hosts are returned as-is.
func NormalizeHTTPAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "127.0.0.1:8765"
	}
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr
	}
	if !strings.Contains(addr, ":") {
		return "127.0.0.1:" + addr
	}
	return addr
}

// buildServer wires the registered tools onto a fresh *mcp.Server.
func buildServer(deps Deps) (*mcp.Server, error) {
	if deps.Client == nil {
		return nil, fmt.Errorf("mcp: daemon client is required")
	}
	if deps.Store == nil {
		return nil, fmt.Errorf("mcp: db store is required")
	}
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
	return server, nil
}

// stderrOnly is an io.Writer that forwards everything to os.Stderr. Used to
// redirect any stray log output the SDK or downstream packages emit, so we
// never accidentally corrupt the stdio JSON-RPC stream on stdout.
type stderrOnly struct{}

func (stderrOnly) Write(p []byte) (int, error) { return os.Stderr.Write(p) }

// Verify io.Writer interface at compile time.
var _ io.Writer = stderrOnly{}
