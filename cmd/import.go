package cmd

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/db"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var importCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import past work records as completed sessions",
	Long: `Backfill sessions you tracked somewhere else — a stopwatch app, a
spreadsheet, or a hand-written log.

The file is tab- or comma-separated, one record per line:

  date <TAB> duration <TAB> description

  2026-08-21    1.00.46    İstek anlama + rezervasyon düzenleme + test
  24.08.2026    24.09.27   Desen sıralama pattern + test + deploy
  2026-09-16    9dk39sn    Filtre panelinin açık başlaması

Dates accept 2026-08-21, 21.08.2026 or 21.08. Durations accept the
stopwatch forms you already write by hand:

  1.00.46      1 hour 0 min 46 sec      (h.m.s)
  24.09.27     24 min 9.27 sec → 24m09s (m.s.cs, the trailing pair is
               read as seconds when the leading value exceeds 23)
  1:09:20      1 hour 9 min 20 sec
  45m29s       45 min 29 sec
  0.75h        45 min

Each record becomes a stopped session starting at 09:00 on its date,
laid end to end so the day's sessions never overlap.

Usage:
  btrack import hours.tsv -p yalcinkaya
  btrack import hours.tsv -p yalcinkaya --dry-run
  btrack import hours.tsv -p yalcinkaya --start 10:00

Run with --dry-run first: it prints exactly what would be created and
writes nothing.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, _ := cmd.Flags().GetString("project")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		startAt, _ := cmd.Flags().GetString("start")
		tagsFlag, _ := cmd.Flags().GetString("tags")

		dayStart, err := parseClock(startAt)
		if err != nil {
			return fmt.Errorf("--start: %w", err)
		}

		records, err := readImportFile(expandHome(args[0]))
		if err != nil {
			return err
		}
		if len(records) == 0 {
			return fmt.Errorf("no usable records in %s", args[0])
		}

		var tags []string
		for _, t := range strings.Fields(tagsFlag) {
			tags = append(tags, strings.TrimPrefix(t, "#"))
		}

		// Lay each day's records end to end from the day's start time.
		cursor := map[string]time.Time{}
		type planned struct {
			start time.Time
			end   time.Time
			rec   importRecord
		}
		var plan []planned
		var total time.Duration

		for _, r := range records {
			key := r.Date.Format("2006-01-02")
			at, ok := cursor[key]
			if !ok {
				at = time.Date(r.Date.Year(), r.Date.Month(), r.Date.Day(),
					0, 0, 0, 0, time.Local).Add(dayStart)
			}
			end := at.Add(r.Duration)
			cursor[key] = end
			plan = append(plan, planned{start: at, end: end, rec: r})
			total += r.Duration
		}

		if dryRun {
			ui.Blank()
			ui.Section(fmt.Sprintf("dry run · %d records · %s", len(plan), formatDur(total)))
			for _, p := range plan {
				ui.KV(p.start.Format("02.01.2006 15:04"),
					fmt.Sprintf("%-11s %s", formatDur(p.rec.Duration), truncate(p.rec.Description, 58)))
			}
			ui.Blank()
			ui.Hint("nothing was written — drop --dry-run to import")
			ui.Blank()
			return nil
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		store, err := db.Open(cfg)
		if err != nil {
			return err
		}
		defer store.Close()

		created := 0
		for _, p := range plan {
			end := p.end
			sess := &db.Session{
				TaskName:  truncate(p.rec.Description, 80),
				StartTime: p.start,
				EndTime:   &end,
				Message:   p.rec.Description,
				Tags:      tags,
				Project:   project,
			}
			if err := store.CreateSession(sess); err != nil {
				return fmt.Errorf("create session for %s: %w",
					p.start.Format("2006-01-02"), err)
			}
			created++
		}

		ui.Blank()
		ui.OK(fmt.Sprintf("imported %d sessions · %s", created, formatDur(total)))
		if project != "" {
			ui.KV("project", "@"+project)
		}
		ui.KV("range", plan[0].start.Format("02.01.2006")+" – "+plan[len(plan)-1].start.Format("02.01.2006"))
		ui.Blank()
		ui.Hint("build the client report with: btrack report " + project + " --from ... --to ...")
		ui.Blank()
		return nil
	},
}

type importRecord struct {
	Date        time.Time
	Duration    time.Duration
	Description string
}

// readImportFile parses a tab- or comma-separated work log.
func readImportFile(path string) ([]importRecord, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var out []importRecord
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields, err := splitRecord(line)
		if err != nil || len(fields) < 3 {
			return nil, fmt.Errorf("line %d: need date, duration and description — got %q", lineNo, line)
		}
		date, err := parseReportDate(fields[0], time.Now())
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}
		dur, err := ParseStopwatch(fields[1])
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}
		desc := strings.TrimSpace(strings.Join(fields[2:], " "))
		out = append(out, importRecord{Date: date, Duration: dur, Description: desc})
	}
	return out, scanner.Err()
}

// splitRecord splits on tabs when present, otherwise on commas (CSV-quoted).
func splitRecord(line string) ([]string, error) {
	if strings.Contains(line, "\t") {
		parts := strings.Split(line, "\t")
		var out []string
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				out = append(out, s)
			}
		}
		return out, nil
	}
	r := csv.NewReader(strings.NewReader(line))
	r.LazyQuotes = true
	return r.Read()
}

var (
	clockRe = regexp.MustCompile(`^(\d+):(\d{1,2})(?::(\d{1,2}))?$`)
	// Unit suffixes are matched without a trailing word boundary: in "45m29s"
	// the "m" is followed immediately by a digit, which is not a boundary.
	// Longer alternatives come first so "sa" never wins over "saat".
	unitRe = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*(saat|saniye|dakika|sn|dk|sa|h|m|s)`)
	dotRe  = regexp.MustCompile(`^(\d+)\.(\d{1,2})(?:\.(\d{1,2}))?$`)
	// A bare decimal like "1.5" or "0,75" is decimal hours, not m.s — the
	// dotted stopwatch form always pads to two digits ("1.05.00").
	decimalRe = regexp.MustCompile(`^\d+[.,]\d+$`)
)

// ParseStopwatch reads the duration forms a hand-kept work log actually uses.
//
// The dotted form is ambiguous — "1.00.46" is 1h00m46s but "24.09.27" is
// 24m09s: nobody writes a 24-hour session in a freelance log. The leading
// value decides: 0–23 means hours, anything larger means minutes.
func ParseStopwatch(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}

	if m := clockRe.FindStringSubmatch(s); m != nil {
		h, _ := strconv.Atoi(m[1])
		mi, _ := strconv.Atoi(m[2])
		sec := 0
		if m[3] != "" {
			sec, _ = strconv.Atoi(m[3])
		}
		return time.Duration(h)*time.Hour +
			time.Duration(mi)*time.Minute +
			time.Duration(sec)*time.Second, nil
	}

	if m := dotRe.FindStringSubmatch(s); m != nil {
		a, _ := strconv.Atoi(m[1])
		b, _ := strconv.Atoi(m[2])
		c := 0
		if m[3] != "" {
			c, _ = strconv.Atoi(m[3])
		}
		if m[3] == "" {
			// Two parts only. "45.29" is minutes and seconds, but "1.5" is a
			// bare decimal: the stopwatch form always pads to two digits.
			if decimalRe.MatchString(s) && len(m[2]) == 1 {
				v, _ := strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64)
				return time.Duration(v * float64(time.Hour)), nil
			}
			return time.Duration(a)*time.Minute + time.Duration(b)*time.Second, nil
		}
		if a <= 23 {
			return time.Duration(a)*time.Hour +
				time.Duration(b)*time.Minute +
				time.Duration(c)*time.Second, nil
		}
		return time.Duration(a)*time.Minute + time.Duration(b)*time.Second +
			time.Duration(c)*time.Millisecond*10, nil
	}

	// Unit-suffixed forms: "45m29s", "1 sa 9 dk", "0,75h".
	if matches := unitRe.FindAllStringSubmatch(s, -1); len(matches) > 0 {
		var total time.Duration
		for _, m := range matches {
			val, err := strconv.ParseFloat(strings.Replace(m[1], ",", ".", 1), 64)
			if err != nil {
				continue
			}
			switch m[2] {
			case "saat", "sa", "h":
				total += time.Duration(val * float64(time.Hour))
			case "dakika", "dk", "m":
				total += time.Duration(val * float64(time.Minute))
			default:
				total += time.Duration(val * float64(time.Second))
			}
		}
		if total > 0 {
			return total, nil
		}
	}

	// Bare number: decimal hours.
	if v, err := strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64); err == nil {
		return time.Duration(v * float64(time.Hour)), nil
	}

	return 0, fmt.Errorf("unrecognised duration %q — try 1.00.46, 45m29s or 1:09:20", s)
}

// parseClock reads an HH:MM time-of-day offset from midnight.
func parseClock(s string) (time.Duration, error) {
	t, err := time.Parse("15:04", strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("expected HH:MM, got %q", s)
	}
	return time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute, nil
}

func init() {
	importCmd.Flags().StringP("project", "p", "", "assign imported sessions to a project")
	importCmd.Flags().Bool("dry-run", false, "show what would be imported without writing")
	importCmd.Flags().String("start", "09:00", "time of day the first session of each date begins")
	importCmd.Flags().String("tags", "", "tags applied to every imported session")
	rootCmd.AddCommand(importCmd)
}
