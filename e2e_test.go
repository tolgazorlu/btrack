package main

// End-to-end checks on the built binary, the way a script or agent calls it.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "btrack")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return bin
}

func runBinary(t *testing.T, bin string, args ...string) (string, int) {
	t.Helper()
	home := t.TempDir()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "HOME="+home, "BTRACK_DB="+filepath.Join(home, "test.db"))
	out, err := cmd.CombinedOutput()
	if ee, ok := err.(*exec.ExitError); ok {
		return string(out), ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return string(out), 0
}

func TestCLIContract(t *testing.T) {
	bin := buildBinary(t)

	if out, code := runBinary(t, bin, "frobnicate"); code == 0 || !strings.Contains(out, "unknown command") {
		t.Errorf("unknown command must fail loudly: code %d out %q", code, out)
	}
	if out, code := runBinary(t, bin); code != 0 || !strings.Contains(out, "DAILY WORKFLOW") {
		t.Errorf("no-args help: code %d out %q", code, out)
	}
	for _, f := range []string{"--version", "-v"} {
		out, code := runBinary(t, bin, f)
		if code != 0 || strings.Contains(strings.TrimSpace(out), " ") || strings.TrimSpace(out) == "" {
			t.Errorf("%s must print a bare version: code %d out %q", f, code, out)
		}
	}
}
