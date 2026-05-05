package cmd

import (
	"fmt"
	"strings"
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

		sep := ui.StyleDimmed.Render(strings.Repeat("─", 44))
		kv := func(label, val, sub string) {
			l := ui.StyleDimmed.Render(fmt.Sprintf("  %-12s", label))
			v := ui.StyleElapsed.Render(fmt.Sprintf("%-10s", val))
			s := ui.StyleDimmed.Render(sub)
			fmt.Printf("%s%s%s\n", l, v, s)
		}

		fmt.Println()
		fmt.Printf("  %s\n", ui.StyleTitle.Render("btrack stats"))
		fmt.Println("  " + sep)
		fmt.Println()

		kv("today", formatDur(today.dur), fmt.Sprintf("%d sessions", today.sessions))
		kv("this week", formatDur(week.dur), fmt.Sprintf("%d sessions", week.sessions))
		kv("this month", formatDur(month.dur), fmt.Sprintf("%d sessions", month.sessions))

		if topTag != "" {
			fmt.Println()
			fmt.Printf("  %s  %s  %s\n",
				ui.StyleDimmed.Render("top tag"),
				ui.StyleTag.Render(topTag),
				ui.StyleDimmed.Render(fmt.Sprintf("(%d this week)", topCount)),
			)
		}

		fmt.Println()
		fmt.Println("  " + sep)
		fmt.Printf("  %s\n\n",
			ui.StyleDimmed.Render("btrack ai ins  for full analytics"),
		)

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
