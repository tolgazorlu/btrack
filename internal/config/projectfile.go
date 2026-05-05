package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ProjectFile holds local overrides loaded from a .btrack file in a project directory.
type ProjectFile struct {
	TaskPrefix string // prepend to every task name started in this dir
	Project    string // default project assignment
	DailyHours int    // override global daily_hours (0 = use global)
}

// FindProjectFile walks up from dir looking for a .btrack file, stopping at the
// filesystem root. Returns nil, nil when no file is found — that is not an error.
func FindProjectFile(dir string) (*ProjectFile, error) {
	current := dir
	for {
		path := filepath.Join(current, ".btrack")
		if _, err := os.Stat(path); err == nil {
			return parseProjectFile(path)
		}
		parent := filepath.Dir(current)
		if parent == current {
			break // reached filesystem root
		}
		current = parent
	}
	return nil, nil
}

func parseProjectFile(path string) (*ProjectFile, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	pf := &ProjectFile{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		// Strip inline comments: "value # comment" → "value"
		if idx := strings.Index(val, " #"); idx >= 0 {
			val = strings.TrimSpace(val[:idx])
		}
		switch key {
		case "task_prefix":
			pf.TaskPrefix = val
		case "project":
			pf.Project = val
		case "daily_hours":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				pf.DailyHours = n
			}
		}
	}
	return pf, scanner.Err()
}
