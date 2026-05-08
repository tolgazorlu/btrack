package db

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type Stats struct {
	Period         string
	TotalSessions  int
	TotalDuration  time.Duration
	AvgDuration    time.Duration
	LongestSession time.Duration
	TagCounts      map[string]int
	DailyBreakdown []DayStat
	TopTasks       []TaskStat
	HourlyPattern  [24]int
}

type DayStat struct {
	Date      time.Time
	Sessions  int
	Duration  time.Duration
	IsToday   bool
}

type TaskStat struct {
	Name     string
	Sessions int
	Duration time.Duration
}

func ComputeStats(sessions []*Session, days int) *Stats {
	s := &Stats{
		Period:    fmt.Sprintf("last %d days", days),
		TagCounts: make(map[string]int),
	}
	if len(sessions) == 0 {
		return s
	}

	taskMap := map[string]*TaskStat{}
	dayMap := map[string]*DayStat{}
	now := time.Now()
	cutoff := now.AddDate(0, 0, -days)

	for _, sess := range sessions {
		if sess.StartTime.Before(cutoff) {
			continue
		}
		d := sess.Duration()
		s.TotalSessions++
		s.TotalDuration += d
		if d > s.LongestSession {
			s.LongestSession = d
		}

		for _, tag := range sess.Tags {
			s.TagCounts[tag]++
		}

		s.HourlyPattern[sess.StartTime.Local().Hour()]++

		if _, ok := taskMap[sess.TaskName]; !ok {
			taskMap[sess.TaskName] = &TaskStat{Name: sess.TaskName}
		}
		taskMap[sess.TaskName].Sessions++
		taskMap[sess.TaskName].Duration += d

		dayKey := sess.StartTime.Local().Format("2006-01-02")
		if _, ok := dayMap[dayKey]; !ok {
			dayMap[dayKey] = &DayStat{
				Date:    sess.StartTime.Local().Truncate(24 * time.Hour),
				IsToday: dayKey == now.Local().Format("2006-01-02"),
			}
		}
		dayMap[dayKey].Sessions++
		dayMap[dayKey].Duration += d
	}

	if s.TotalSessions > 0 {
		s.AvgDuration = s.TotalDuration / time.Duration(s.TotalSessions)
	}

	for _, t := range taskMap {
		s.TopTasks = append(s.TopTasks, *t)
	}
	sort.Slice(s.TopTasks, func(i, j int) bool {
		return s.TopTasks[i].Duration > s.TopTasks[j].Duration
	})
	if len(s.TopTasks) > 5 {
		s.TopTasks = s.TopTasks[:5]
	}

	for i := days - 1; i >= 0; i-- {
		day := now.Local().AddDate(0, 0, -i).Truncate(24 * time.Hour)
		key := day.Format("2006-01-02")
		if ds, ok := dayMap[key]; ok {
			s.DailyBreakdown = append(s.DailyBreakdown, *ds)
		} else {
			s.DailyBreakdown = append(s.DailyBreakdown, DayStat{
				Date:    day,
				IsToday: i == 0,
			})
		}
	}

	return s
}

func (s *Stats) ToJSON() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "{\n")
	fmt.Fprintf(&sb, "  \"period\": %q,\n", s.Period)
	fmt.Fprintf(&sb, "  \"total_sessions\": %d,\n", s.TotalSessions)
	fmt.Fprintf(&sb, "  \"total_hours\": %.2f,\n", s.TotalDuration.Hours())
	fmt.Fprintf(&sb, "  \"avg_session_minutes\": %.1f,\n", s.AvgDuration.Minutes())
	fmt.Fprintf(&sb, "  \"longest_session_minutes\": %.1f,\n", s.LongestSession.Minutes())

	fmt.Fprintf(&sb, "  \"top_tasks\": [\n")
	for i, t := range s.TopTasks {
		comma := ","
		if i == len(s.TopTasks)-1 {
			comma = ""
		}
		fmt.Fprintf(&sb, "    {\"name\": %q, \"sessions\": %d, \"hours\": %.2f}%s\n",
			t.Name, t.Sessions, t.Duration.Hours(), comma)
	}
	fmt.Fprintf(&sb, "  ],\n")

	fmt.Fprintf(&sb, "  \"tag_distribution\": {")
	i := 0
	for tag, count := range s.TagCounts {
		if i > 0 {
			fmt.Fprintf(&sb, ", ")
		}
		fmt.Fprintf(&sb, "%q: %d", tag, count)
		i++
	}
	fmt.Fprintf(&sb, "},\n")

	peakHour, peakCount := 0, 0
	for h, c := range s.HourlyPattern {
		if c > peakCount {
			peakCount = c
			peakHour = h
		}
	}
	fmt.Fprintf(&sb, "  \"peak_hour\": %d,\n", peakHour)

	fmt.Fprintf(&sb, "  \"daily_sessions\": [")
	for i, d := range s.DailyBreakdown {
		if i > 0 {
			fmt.Fprintf(&sb, ", ")
		}
		fmt.Fprintf(&sb, "%d", d.Sessions)
	}
	fmt.Fprintf(&sb, "]\n}")
	return sb.String()
}
