package db

import (
	"testing"
	"time"
)

func makeSession(task string, start time.Time, durationMin int, tags []string) *Session {
	end := start.Add(time.Duration(durationMin) * time.Minute)
	return &Session{
		TaskName:  task,
		StartTime: start,
		EndTime:   &end,
		Tags:      tags,
	}
}

func TestComputeStats_Empty(t *testing.T) {
	s := ComputeStats(nil, 7)
	if s.TotalSessions != 0 {
		t.Errorf("expected 0 sessions, got %d", s.TotalSessions)
	}
	if s.TotalDuration != 0 {
		t.Errorf("expected 0 duration, got %v", s.TotalDuration)
	}
}

func TestComputeStats_BasicCounts(t *testing.T) {
	now := time.Now()
	sessions := []*Session{
		makeSession("task A", now.Add(-2*time.Hour), 60, []string{"#bugfix"}),
		makeSession("task B", now.Add(-1*time.Hour), 30, []string{"#feature"}),
	}

	s := ComputeStats(sessions, 7)

	if s.TotalSessions != 2 {
		t.Errorf("TotalSessions = %d, want 2", s.TotalSessions)
	}
	if s.TotalDuration != 90*time.Minute {
		t.Errorf("TotalDuration = %v, want 90m", s.TotalDuration)
	}
	if s.AvgDuration != 45*time.Minute {
		t.Errorf("AvgDuration = %v, want 45m", s.AvgDuration)
	}
	if s.LongestSession != 60*time.Minute {
		t.Errorf("LongestSession = %v, want 60m", s.LongestSession)
	}
}

func TestComputeStats_TagCounts(t *testing.T) {
	now := time.Now()
	sessions := []*Session{
		makeSession("fix A", now.Add(-3*time.Hour), 30, []string{"#bugfix"}),
		makeSession("fix B", now.Add(-2*time.Hour), 30, []string{"#bugfix"}),
		makeSession("feat C", now.Add(-1*time.Hour), 30, []string{"#feature"}),
	}

	s := ComputeStats(sessions, 7)

	if s.TagCounts["#bugfix"] != 2 {
		t.Errorf("TagCounts[#bugfix] = %d, want 2", s.TagCounts["#bugfix"])
	}
	if s.TagCounts["#feature"] != 1 {
		t.Errorf("TagCounts[#feature] = %d, want 1", s.TagCounts["#feature"])
	}
}

func TestComputeStats_CutoffExcludesOld(t *testing.T) {
	now := time.Now()
	sessions := []*Session{
		makeSession("recent", now.Add(-1*time.Hour), 30, nil),
		makeSession("old", now.AddDate(0, 0, -10), 60, nil), // outside 7-day window
	}

	s := ComputeStats(sessions, 7)

	if s.TotalSessions != 1 {
		t.Errorf("TotalSessions = %d, want 1 (old session excluded)", s.TotalSessions)
	}
}

func TestComputeStats_TopTasksSortedByDuration(t *testing.T) {
	now := time.Now()
	sessions := []*Session{
		makeSession("short task", now.Add(-3*time.Hour), 10, nil),
		makeSession("long task", now.Add(-2*time.Hour), 120, nil),
		makeSession("medium task", now.Add(-1*time.Hour), 45, nil),
	}

	s := ComputeStats(sessions, 7)

	if len(s.TopTasks) == 0 {
		t.Fatal("TopTasks is empty")
	}
	if s.TopTasks[0].Name != "long task" {
		t.Errorf("TopTasks[0] = %q, want 'long task'", s.TopTasks[0].Name)
	}
}

func TestComputeStats_DailyBreakdownLength(t *testing.T) {
	now := time.Now()
	sessions := []*Session{
		makeSession("task", now.Add(-1*time.Hour), 30, nil),
	}

	s := ComputeStats(sessions, 7)

	if len(s.DailyBreakdown) != 7 {
		t.Errorf("DailyBreakdown len = %d, want 7", len(s.DailyBreakdown))
	}
	// Today should be marked
	if !s.DailyBreakdown[6].IsToday {
		t.Errorf("last day in breakdown should be today")
	}
}

func TestComputeStats_HourlyPattern(t *testing.T) {
	now := time.Now()
	start := now.Add(-30 * time.Minute)
	hour := start.Local().Hour() // use the session's actual start hour, not current hour
	sessions := []*Session{
		makeSession("task", start, 20, nil),
	}

	s := ComputeStats(sessions, 7)

	if s.HourlyPattern[hour] != 1 {
		t.Errorf("HourlyPattern[%d] = %d, want 1", hour, s.HourlyPattern[hour])
	}
}
