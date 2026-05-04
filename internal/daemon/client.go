package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
"strconv"
	"strings"
	"time"

	"github.com/tolgaozgun/btrack/internal/config"
)

type Client struct{}

func NewClient() *Client { return &Client{} }

func (c *Client) Send(action string, payload any) (*Response, error) {
	if err := c.ensureDaemon(); err != nil {
		return nil, err
	}
	return c.send(action, payload)
}

func (c *Client) send(action string, payload any) (*Response, error) {
	var raw json.RawMessage
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		raw = b
	}

	req := Request{Action: action, Payload: raw}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialTimeout("unix", config.SocketPath(), 3*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to daemon: %w", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))

	if _, err := conn.Write(reqBytes); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	// Signal EOF so server knows the full message arrived.
	if tc, ok := conn.(*net.UnixConn); ok {
		tc.CloseWrite()
	}

	var resp Response
	buf := new(bytes.Buffer)
	tmp := make([]byte, 4096)
	for {
		n, err := conn.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if err != nil {
			break
		}
	}
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &resp, nil
}

func (c *Client) Ping() bool {
	resp, err := c.send(ActionPing, nil)
	return err == nil && resp.Success
}

func (c *Client) ensureDaemon() error {
	if c.isDaemonRunning() {
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find executable: %w", err)
	}
	exe, _ = filepath.EvalSymlinks(exe)

	cmd := exec.Command(exe, "daemon", "start")
	cmd.SysProcAttr = sysProcAttr()
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	cmd.Process.Release()

	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		if c.isDaemonRunning() {
			return nil
		}
	}
	return fmt.Errorf("daemon did not start in time")
}

func (c *Client) isDaemonRunning() bool {
	pidFile := config.PidFile()
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return isProcessAlive(proc)
}
