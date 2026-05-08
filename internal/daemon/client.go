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

	"github.com/tolgazorlu/btrack/internal/config"
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

func (c *Client) Switch(payload SwitchPayload) (*SwitchData, error) {
	resp, err := c.Send(ActionSwitch, payload)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	var data SwitchData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("parse switch response: %w", err)
	}
	return &data, nil
}

func (c *Client) Ping() bool {
	resp, err := c.send(ActionPing, nil)
	return err == nil && resp.Success
}

func (c *Client) QuietStatus() *StatusData {
	req := Request{Action: ActionStatus}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil
	}

	conn, err := net.DialTimeout("unix", config.SocketPath(), 200*time.Millisecond)
	if err != nil {
		return nil
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(500 * time.Millisecond))

	if _, err := conn.Write(reqBytes); err != nil {
		return nil
	}
	if tc, ok := conn.(*net.UnixConn); ok {
		tc.CloseWrite()
	}

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

	var resp Response
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil || !resp.Success {
		return nil
	}

	var status StatusData
	if err := json.Unmarshal(resp.Data, &status); err != nil {
		return nil
	}
	return &status
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
