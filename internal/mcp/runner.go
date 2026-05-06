package mcp

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tolgazorlu/btrack/internal/config"
)

// HTTPStatus reports whether a background MCP HTTP server is currently
// running, plus where to reach it. Zero value means "not running".
type HTTPStatus struct {
	Running bool
	PID     int
	Addr    string // host:port, e.g. "127.0.0.1:8765"
	URL     string // http://host:port/mcp
}

// httpPidFile holds the pid+addr of any background HTTP server started by
// `/mcp` from the console (or by `btrack mcp start` later). Lives next to
// the daemon's pid/socket files in DataDir.
func httpPidFile() string {
	return filepath.Join(config.DataDir(), "mcp-http.pid")
}

// CurrentHTTPStatus reads the pid file, probes the process, and returns
// the current state. Cleans up stale pid files automatically.
func CurrentHTTPStatus() HTTPStatus {
	data, err := os.ReadFile(httpPidFile())
	if err != nil {
		return HTTPStatus{}
	}
	parts := strings.SplitN(strings.TrimSpace(string(data)), "\n", 2)
	pid, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return HTTPStatus{}
	}
	if !processAlive(pid) {
		_ = os.Remove(httpPidFile())
		return HTTPStatus{}
	}
	addr := "127.0.0.1:8765"
	if len(parts) > 1 {
		addr = strings.TrimSpace(parts[1])
	}
	return HTTPStatus{
		Running: true,
		PID:     pid,
		Addr:    addr,
		URL:     "http://" + addr + "/mcp",
	}
}

// StartHTTPBackground forks `btrack mcp --http addr` as a detached child
// process and waits briefly for it to start listening. If a server is
// already recorded in the pid file and still alive, returns its status
// without spawning a new one.
func StartHTTPBackground(addr string) (HTTPStatus, error) {
	addr = NormalizeHTTPAddr(addr)

	if cur := CurrentHTTPStatus(); cur.Running {
		return cur, nil
	}

	// Fail fast if the port is already taken — beats spawning a child that
	// will exit a few hundred ms later with a confusing error.
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return HTTPStatus{}, fmt.Errorf("port %s in use: %w", addr, err)
	}
	_ = ln.Close()

	exe, err := os.Executable()
	if err != nil {
		return HTTPStatus{}, fmt.Errorf("locate btrack binary: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	cmd := exec.Command(exe, "mcp", "--http", addr)
	cmd.SysProcAttr = detachSysProcAttr()
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return HTTPStatus{}, fmt.Errorf("spawn: %w", err)
	}
	_ = cmd.Process.Release()

	if err := os.MkdirAll(filepath.Dir(httpPidFile()), 0750); err != nil {
		return HTTPStatus{}, fmt.Errorf("mkdir pid dir: %w", err)
	}
	pidContent := fmt.Sprintf("%d\n%s", cmd.Process.Pid, addr)
	if err := os.WriteFile(httpPidFile(), []byte(pidContent), 0600); err != nil {
		return HTTPStatus{}, fmt.Errorf("write pid file: %w", err)
	}

	// Poll the address until it accepts a connection (or we give up).
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond); err == nil {
			_ = conn.Close()
			return CurrentHTTPStatus(), nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Spawn succeeded but listener never came up — return whatever we have
	// so the caller can decide whether to surface it as an error.
	cur := CurrentHTTPStatus()
	if !cur.Running {
		return HTTPStatus{}, errors.New("server did not start in time")
	}
	return cur, nil
}

// StopHTTPBackground kills the recorded background HTTP server.
func StopHTTPBackground() error {
	cur := CurrentHTTPStatus()
	if !cur.Running {
		return errors.New("not running")
	}
	proc, err := os.FindProcess(cur.PID)
	if err != nil {
		return err
	}
	if err := proc.Kill(); err != nil {
		return err
	}
	_ = os.Remove(httpPidFile())
	return nil
}
