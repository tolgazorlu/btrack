package cmd

import (
	"testing"
	"time"
)

func TestSlugCannotEscapeDirectory(t *testing.T) {
	cases := map[string]string{
		"Yalçınkaya B2B":      "yalcinkaya-b2b",
		"../../.ssh/x":        "sshx",
		`C:\Windows\System32`: "cwindowssystem32",
		"a & calc.exe":        "a--calcexe",
		"///":                 "client",
		"":                    "client",
	}
	for in, want := range cases {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}
}

// Callers bound the range as [from, to+1 day); --to must therefore not
// leak sessions from the following day into the report.
func TestResolveRangeToExcludesNextDay(t *testing.T) {
	from, to, err := resolveRange("01.10.2026", "31.10.2026", false)
	if err != nil {
		t.Fatal(err)
	}
	end := to.AddDate(0, 0, 1)
	loc := from.Location()
	lastEvening := time.Date(2026, 10, 31, 23, 30, 0, 0, loc)
	nextNoon := time.Date(2026, 11, 1, 12, 10, 0, 0, loc)
	if !lastEvening.Before(end) {
		t.Errorf("31.10 23:30 should be inside the range ending %v", end)
	}
	if nextNoon.Before(end) {
		t.Errorf("01.11 12:10 leaked into the range ending %v", end)
	}
	if got := to.Format("02.01.2006"); got != "31.10.2026" {
		t.Errorf("displayed end = %s, want 31.10.2026", got)
	}
}
