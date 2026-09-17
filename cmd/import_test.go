package cmd

import (
	"testing"
	"time"
)

func TestParseStopwatch(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		// Dotted stopwatch form: leading value 0-23 reads as hours.
		{"1.00.46", time.Hour + 46*time.Second},
		{"3.00.24", 3*time.Hour + 24*time.Second},
		{"1.13.56", time.Hour + 13*time.Minute + 56*time.Second},
		// Leading value above 23 cannot be hours in a freelance log: minutes.
		{"24.09.27", 24*time.Minute + 9*time.Second + 270*time.Millisecond},
		{"52.11.00", 52*time.Minute + 11*time.Second},
		// Two-part dotted form is minutes and seconds.
		{"45.29", 45*time.Minute + 29*time.Second},
		// Clock form.
		{"1:09:20", time.Hour + 9*time.Minute + 20*time.Second},
		{"0:37", 37 * time.Minute},
		// Unit-suffixed, English and Turkish.
		{"45m29s", 45*time.Minute + 29*time.Second},
		{"1 sa 9 dk", time.Hour + 9*time.Minute},
		{"13 dk 18 sn", 13*time.Minute + 18*time.Second},
		{"1 sa 00 dk 46 sn", time.Hour + 46*time.Second},
		// Decimal hours.
		{"0,75h", 45 * time.Minute},
		{"1.5", 90 * time.Minute},
	}
	for _, c := range cases {
		got, err := ParseStopwatch(c.in)
		if err != nil {
			t.Errorf("ParseStopwatch(%q) returned error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseStopwatch(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseStopwatchRejectsGarbage(t *testing.T) {
	for _, in := range []string{"", "   ", "yarım saat", "abc"} {
		if _, err := ParseStopwatch(in); err == nil {
			t.Errorf("ParseStopwatch(%q) should have failed", in)
		}
	}
}

func TestSplitTrailingFlag(t *testing.T) {
	cases := []struct {
		in       string
		wantDesc string
		wantFlag string
	}{
		{"Mobil uygulama hata kontrolü [commit yok]", "Mobil uygulama hata kontrolü", "commit yok"},
		{"Domain problemi [Selge]", "Domain problemi", "Selge"},
		{"Normal açıklama", "Normal açıklama", ""},
		// Brackets in the middle are part of the sentence, not a flag.
		{"Excel [v2] kolonları düzeltildi", "Excel [v2] kolonları düzeltildi", ""},
	}
	for _, c := range cases {
		desc, flag := splitTrailingFlag(c.in)
		if desc != c.wantDesc || flag != c.wantFlag {
			t.Errorf("splitTrailingFlag(%q) = (%q, %q), want (%q, %q)",
				c.in, desc, flag, c.wantDesc, c.wantFlag)
		}
	}
}

func TestParseReportDate(t *testing.T) {
	ref := time.Date(2026, 9, 17, 0, 0, 0, 0, time.Local)
	cases := []struct {
		in string
		y  int
		m  time.Month
		d  int
	}{
		{"21.08.2026", 2026, time.August, 21},
		{"2026-08-21", 2026, time.August, 21},
		{"21.08", 2026, time.August, 21}, // bare day-month uses the reference year
		{"16.09", 2026, time.September, 16},
	}
	for _, c := range cases {
		got, err := parseReportDate(c.in, ref)
		if err != nil {
			t.Errorf("parseReportDate(%q) returned error: %v", c.in, err)
			continue
		}
		y, m, d := got.Date()
		if y != c.y || m != c.m || d != c.d {
			t.Errorf("parseReportDate(%q) = %04d-%02d-%02d, want %04d-%02d-%02d",
				c.in, y, m, d, c.y, c.m, c.d)
		}
	}
	if _, err := parseReportDate("not-a-date", ref); err == nil {
		t.Error("parseReportDate should reject garbage")
	}
}

func TestTruncateRespectsRunes(t *testing.T) {
	// Turkish text must not be cut mid-character.
	in := "İstek anlama + ürün rezervasyon düzenleme + test + deploy"
	got := truncate(in, 20)
	if len([]rune(got)) > 20 {
		t.Errorf("truncate produced %d runes, want <= 20: %q", len([]rune(got)), got)
	}
	for _, r := range got {
		if r == '\uFFFD' {
			t.Errorf("truncate corrupted UTF-8: %q", got)
		}
	}
	if short := truncate("kısa", 20); short != "kısa" {
		t.Errorf("truncate shortened a string that fits: %q", short)
	}
}
