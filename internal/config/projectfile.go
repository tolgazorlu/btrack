package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ProjectFile holds local overrides loaded from a .btrack file in a project
// directory. Every field is optional — zero values mean "use global default".
type ProjectFile struct {
	Project     string   // default project assignment for sessions started here
	TaskPrefix  string   // text prepended to every task name
	Description string   // short note on what this project is (used by AI features)
	DailyHours  int      // override global daily_hours (0 = use global)
	BillingRate float64  // hourly rate used by `btrack invoice` (0 = use global)
	DefaultTags []string // tags auto-applied to every session stopped in this dir
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
		// Don't strip inline comments here — values like default_tags or
		// task_prefix may legitimately contain '#'. Comments must be on
		// their own line (lines starting with '#').
		switch key {
		case "task_prefix":
			pf.TaskPrefix = val
		case "project":
			pf.Project = val
		case "description":
			pf.Description = val
		case "daily_hours":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				pf.DailyHours = n
			}
		case "billing_rate":
			if r, err := strconv.ParseFloat(val, 64); err == nil && r >= 0 {
				pf.BillingRate = r
			}
		case "default_tags":
			pf.DefaultTags = ParseTagList(val)
		}
	}
	return pf, scanner.Err()
}

// ParseTagList accepts comma- or whitespace-separated tags, with or without a
// leading '#', and returns canonical "#tag" form, lowercased and deduplicated.
func ParseTagList(raw string) []string {
	seen := map[string]bool{}
	var out []string
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t'
	}) {
		t := strings.ToLower(strings.TrimSpace(part))
		if t == "" {
			continue
		}
		if !strings.HasPrefix(t, "#") {
			t = "#" + t
		}
		if seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// Render serialises a ProjectFile to the on-disk .btrack format. Keys are
// emitted in a stable, human-friendly order. Zero-value fields are omitted.
func (pf *ProjectFile) Render() string {
	var sb strings.Builder
	sb.WriteString("# btrack project config — https://github.com/tolgazorlu/btrack\n")
	if pf.Project != "" {
		sb.WriteString(fmt.Sprintf("project       = %s\n", pf.Project))
	}
	if pf.TaskPrefix != "" {
		sb.WriteString(fmt.Sprintf("task_prefix   = %s\n", pf.TaskPrefix))
	}
	if pf.Description != "" {
		sb.WriteString(fmt.Sprintf("description   = %s\n", pf.Description))
	}
	if pf.DailyHours > 0 {
		sb.WriteString(fmt.Sprintf("daily_hours   = %d\n", pf.DailyHours))
	}
	if pf.BillingRate > 0 {
		sb.WriteString(fmt.Sprintf("billing_rate  = %.2f\n", pf.BillingRate))
	}
	if len(pf.DefaultTags) > 0 {
		tags := append([]string{}, pf.DefaultTags...)
		sort.Strings(tags)
		sb.WriteString(fmt.Sprintf("default_tags  = %s\n", strings.Join(tags, ", ")))
	}
	return sb.String()
}
