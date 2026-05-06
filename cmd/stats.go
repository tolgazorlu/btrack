package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/db"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Quick productivity snapshot",
	Long: `Show a compact productivity snapshot across today, this week, and this month.

Only completed (stopped) sessions are counted — active sessions are excluded.

Usage:
  btrack stats

What you'll see:
  · Today: time worked and session count
  · This week: time and sessions
  · This month: time and sessions
  · Most used tag this week

Tips:
  · For full analytics with charts and AI analysis: btrack ai insights
  · For a day-by-day breakdown: btrack h -w
  · For a quick tag filter: btrack tag #bugfix
  · Connect GitHub to enrich AI insights: btrack github connect`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		store, err := db.Open(cfg)
		if err != nil {
			return err
		}
		defer store.Close()

		sessions, err := store.GetRecentSessions(2000)
		if err != nil {
			return err
		}

		now := time.Now()

		// Boundaries.
		todayStart := truncateDay(now)
		weekStart := todayStart.AddDate(0, 0, -int(now.Weekday()-1))
		if now.Weekday() == time.Sunday {
			weekStart = todayStart.AddDate(0, 0, -6)
		}
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)

		type period struct {
			dur      time.Duration
			sessions int
		}
		today := period{}
		week := period{}
		month := period{}
		weekTags := map[string]int{}

		for _, s := range sessions {
			if s.EndTime == nil {
				continue
			}
			d := s.Duration()
			st := s.StartTime.Local()

			if !st.Before(todayStart) {
				today.dur += d
				today.sessions++
			}
			if !st.Before(weekStart) {
				week.dur += d
				week.sessions++
				for _, t := range s.Tags {
					weekTags[t]++
				}
			}
			if !st.Before(monthStart) {
				month.dur += d
				month.sessions++
			}
		}

		topTag := ""
		topCount := 0
		for t, c := range weekTags {
			if c > topCount {
				topCount = c
				topTag = t
			}
		}

		ui.Header("stats", "")
		row := func(label string, p period) {
			val := ui.StyleElapsed.Render(fmt.Sprintf("%-10s", formatDur(p.dur))) +
				ui.StyleDimmed.Render(fmt.Sprintf("%d sessions", p.sessions))
			ui.KV(label, val)
		}
		row("today", today)
		row("this week", week)
		row("this month", month)

		if topTag != "" {
			ui.KV("top tag", ui.StyleTag.Render(topTag)+
				"  "+ui.StyleDimmed.Render(fmt.Sprintf("(%d this week)", topCount)))
		}

		ui.Footer("btrack ai ins  for full analytics")
		return nil
	},
}

func truncateDay(t time.Time) time.Time {
	y, m, d := t.Local().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

func init() {
	rootCmd.AddCommand(statsCmd)
}
