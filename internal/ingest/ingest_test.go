package ingest

import (
	"testing"
	"time"
)

func at(min float64) time.Time {
	return time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC).Add(time.Duration(min * float64(time.Minute)))
}

func TestComputeSplit(t *testing.T) {
	ev := []Event{
		{at(0), Human},  // first prompt: +2m allowance
		{at(1), Agent},  // agent 1m
		{at(6), Agent},  // agent 5m
		{at(9), Human},  // human read+typed 3m
		{at(50), Agent}, // 41m stall: not counted
		{at(52), Agent}, // agent 2m
		{at(90), Human}, // walked away 38m: +2m allowance
		{at(91), Agent}, // agent 1m
	}
	sp := ComputeSplit(ev, SplitOptions{})
	want := Split{Wall: 93 * time.Minute, Active: 7 * time.Minute, Agent: 9 * time.Minute}
	if sp != want {
		t.Fatalf("split = %+v, want %+v", sp, want)
	}
	if sp.Active+sp.Agent > sp.Wall {
		t.Fatal("active+agent exceeds wall")
	}
}

func TestComputeSplitUnsortedAndEmpty(t *testing.T) {
	if (ComputeSplit(nil, SplitOptions{}) != Split{}) {
		t.Fatal("empty split not zero")
	}
	sp := ComputeSplit([]Event{{at(3), Agent}, {at(0), Human}}, SplitOptions{})
	if sp.Agent != 3*time.Minute || sp.Active != 2*time.Minute || sp.Wall != 5*time.Minute {
		t.Fatalf("unsorted split = %+v", sp)
	}
}

func TestSegments(t *testing.T) {
	ev := []Event{
		{at(0), Human}, {at(1), Agent}, {at(6), Agent}, {at(9), Human},
		{at(50), Agent}, {at(52), Agent}, // 41m stall: new segment
		{at(90), Human}, {at(91), Agent}, // 38m away: new segment
	}
	segs := Segments(ev, SplitOptions{})
	if len(segs) != 3 {
		t.Fatalf("got %d segments: %+v", len(segs), segs)
	}
	want := []Segment{
		{Start: at(-2), End: at(9), Active: 5 * time.Minute, Agent: 6 * time.Minute},
		{Start: at(50), End: at(52), Agent: 2 * time.Minute},
		{Start: at(88), End: at(91), Active: 2 * time.Minute, Agent: time.Minute},
	}
	for i := range want {
		if segs[i] != want[i] {
			t.Errorf("segment %d = %+v, want %+v", i, segs[i], want[i])
		}
	}
}
