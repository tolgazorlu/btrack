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

type HTTPStatus struct {
	Running bool
	PID     int
	Addr    string
	URL     string
}

func httpPidFile() string {
	return filepath.Join(config.DataDir(), "mcp-http.pid")
}

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

func StartHTTPBackground(addr string) (HTTPStatus, error) {
	addr = NormalizeHTTPAddr(addr)

	if cur := CurrentHTTPStatus(); cur.Running {
		return cur, nil
	}

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

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond); err == nil {
			_ = conn.Close()
			return CurrentHTTPStatus(), nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	cur := CurrentHTTPStatus()
	if !cur.Running {
		return HTTPStatus{}, errors.New("server did not start in time")
	}
	return cur, nil
}

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
