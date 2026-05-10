package notify

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// Notify fires a best-effort native OS notification. Errors are swallowed:
// callers should not rely on it being delivered (no daemon, no permission,
// no notification service all silently no-op).
func Notify(title, body string) {
	go func() {
		switch runtime.GOOS {
		case "darwin":
			script := fmt.Sprintf(
				`display notification %s with title %s`,
				escapeAppleScript(body), escapeAppleScript(title),
			)
			_ = exec.Command("osascript", "-e", script).Run()
		case "linux":
			_ = exec.Command("notify-send", title, body).Run()
		case "windows":
			ps := fmt.Sprintf(
				`[reflection.assembly]::loadwithpartialname('System.Windows.Forms') | Out-Null; `+
					`[System.Windows.Forms.MessageBox]::Show(%q, %q) | Out-Null`,
				body, title,
			)
			_ = exec.Command("powershell", "-NoProfile", "-Command", ps).Run()
		}
	}()
}

// Bell plays a short attention sound. Falls back to the terminal bell (BEL,
// 0x07) if no platform sound player is available.
func Bell() {
	go func() {
		switch runtime.GOOS {
		case "darwin":
			if err := exec.Command("afplay", "/System/Library/Sounds/Glass.aiff").Run(); err == nil {
				return
			}
		case "linux":
			candidates := []string{
				"/usr/share/sounds/freedesktop/stereo/complete.oga",
				"/usr/share/sounds/freedesktop/stereo/bell.oga",
			}
			for _, path := range candidates {
				if _, err := os.Stat(path); err != nil {
					continue
				}
				if err := exec.Command("paplay", path).Run(); err == nil {
					return
				}
				if err := exec.Command("aplay", "-q", path).Run(); err == nil {
					return
				}
			}
		}
		fmt.Fprint(os.Stderr, "\a")
	}()
}

func escapeAppleScript(s string) string {
	out := make([]byte, 0, len(s)+2)
	out = append(out, '"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '"' || c == '\\' {
			out = append(out, '\\')
		}
		out = append(out, c)
	}
	out = append(out, '"')
	return string(out)
}
