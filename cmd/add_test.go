package cmd

import (
	"testing"
	"time"
)

func TestParseAddTime(t *testing.T) {
	day := time.Date(2026, 9, 16, 0, 0, 0, 0, time.Local)

	got, err := parseAddTime("14:03", day)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 9, 16, 14, 3, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Errorf("HH:MM: got %v, want %v", got, want)
	}

	got, err = parseAddTime("2026-09-16 22:10", day)
	if err != nil {
		t.Fatal(err)
	}
	want = time.Date(2026, 9, 16, 22, 10, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Errorf("full timestamp: got %v, want %v", got, want)
	}

	if _, err := parseAddTime("gibberish", day); err == nil {
		t.Error("expected error for unrecognised time")
	}
}
