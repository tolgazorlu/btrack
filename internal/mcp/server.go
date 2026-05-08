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

var Version = "dev"

func Run(ctx context.Context, deps Deps) error {
	server, err := buildServer(deps)
	if err != nil {
		return err
	}
	log.SetOutput(stderrOnly{})
	return server.Run(ctx, &mcp.StdioTransport{})
}

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

type stderrOnly struct{}

func (stderrOnly) Write(p []byte) (int, error) { return os.Stderr.Write(p) }

var _ io.Writer = stderrOnly{}
