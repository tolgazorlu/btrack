package cmd

import (
	"testing"
	"time"
)

func TestParseAtTime(t *testing.T) {
	loc := time.FixedZone("test", 0)
	now := time.Date(2026, 5, 10, 14, 0, 0, 0, loc)

	tests := []struct {
		name    string
		in      string
		want    time.Time
		wantErr bool
	}{
		{"duration ago", "2h ago", now.Add(-2 * time.Hour), false},
		{"duration ago compound", "1h30m ago", now.Add(-90 * time.Minute), false},
		{"bare duration", "45m", now.Add(-45 * time.Minute), false},
		{"hh:mm earlier today", "10:30", time.Date(2026, 5, 10, 10, 30, 0, 0, loc), false},
		{"hh:mm later → yesterday", "18:00", time.Date(2026, 5, 9, 18, 0, 0, 0, loc), false},
		{"yesterday hh:mm", "yesterday 09:15", time.Date(2026, 5, 9, 9, 15, 0, 0, loc), false},
		{"rfc3339", "2026-05-09T08:00:00Z", time.Date(2026, 5, 9, 8, 0, 0, 0, time.UTC), false},
		{"empty", "", time.Time{}, true},
		{"garbage", "next tuesday", time.Time{}, true},
		{"bad hh:mm", "25:00", time.Time{}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseAtTime(tc.in, now)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
