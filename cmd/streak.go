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

var streakCmd = &cobra.Command{
	Use:   "streak",
	Short: "Show your consecutive working day streak",
	Long: `Display your current and longest working day streaks.

A "working day" = any day with at least one completed (stopped) session.
Active sessions that haven't been stopped yet do not count.

Usage:
  btrack streak

What you'll see:
  · Current streak (consecutive days with at least one completed session)
  · Longest streak ever recorded
  · Last 30 days activity calendar (legend: active / today / inactive)

Tips:
  · Stop sessions with: btrack x -m "what you did"
  · Even a 5-minute session counts — stop it to keep the streak alive
  · Sessions from btrack github sync also count toward your streak`,
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

		// Build a set of active days (completed sessions only).
		daySet := map[string]bool{}
		for _, s := range sessions {
			if s.EndTime != nil {
				key := s.StartTime.Local().Format("2006-01-02")
				daySet[key] = true
			}
		}

		now := time.Now()
		today := now.Local().Format("2006-01-02")
		yesterday := now.AddDate(0, 0, -1).Local().Format("2006-01-02")

		// Current streak: count back from today or yesterday.
		current := 0
		startFrom := today
		if !daySet[today] {
			startFrom = yesterday
		}
		t, _ := time.ParseInLocation("2006-01-02", startFrom, time.Local)
		for {
			key := t.Format("2006-01-02")
			if !daySet[key] {
				break
			}
			current++
			t = t.AddDate(0, 0, -1)
		}

		// Longest streak ever.
		longest := 0
		run := 0
		// Walk days in order across all history.
		if len(sessions) > 0 {
			oldest := sessions[len(sessions)-1].StartTime.Local()
			day := time.Date(oldest.Year(), oldest.Month(), oldest.Day(), 0, 0, 0, 0, time.Local)
			for !day.After(now) {
				if daySet[day.Format("2006-01-02")] {
					run++
					if run > longest {
						longest = run
					}
				} else {
					run = 0
				}
				day = day.AddDate(0, 0, 1)
			}
		}

		// Last 30 days mini-calendar.
		sep := ui.StyleDimmed.Render(strings.Repeat("─", 44))

		fmt.Println()
		fmt.Printf("  %s\n", ui.StyleTitle.Render("btrack streak"))
		fmt.Println("  " + sep)
		fmt.Println()

		flame := "🔥"
		if current == 0 {
			flame = "  "
		}
		fmt.Printf("  %s  %s  %s\n",
			flame,
			ui.StyleHighlight.Render(fmt.Sprintf("%d day streak", current)),
			ui.StyleDimmed.Render(fmt.Sprintf("(longest: %d days)", longest)),
		)
		fmt.Println()

		// 30-day calendar: 5 weeks × 7 days.
		fmt.Printf("  %s\n", ui.StyleDimmed.Render("last 30 days"))
		fmt.Println()

		// Print day-of-week header.
		fmt.Printf("  %s\n", ui.StyleDimmed.Render("Mo Tu We Th Fr Sa Su"))

		// Find the Monday on or before 29 days ago.
		startDay := now.AddDate(0, 0, -29)
		y, m, d := startDay.Local().Date()
		startDay = time.Date(y, m, d, 0, 0, 0, 0, time.Local)
		wd := int(startDay.Weekday())
		if wd == 0 {
			wd = 7
		}
		startDay = startDay.AddDate(0, 0, -(wd - 1))

		week := ""
		cur := startDay
		for cur.Before(now.AddDate(0, 0, 1)) {
			key := cur.Format("2006-01-02")
			isFuture := cur.After(now)
			isToday := key == today

			var cell string
			switch {
			case isFuture:
				cell = ui.StyleDimmed.Render("  ")
			case isToday && daySet[key]:
				cell = ui.StyleSuccess.Render("◉ ")
			case isToday:
				cell = ui.StyleWarning.Render("○ ")
			case daySet[key]:
				cell = ui.StyleSuccess.Render("█ ")
			default:
				cell = ui.StyleDimmed.Render("░ ")
			}
			week += cell

			if int(cur.Weekday()) == 0 { // Sunday → end of week
				fmt.Printf("  %s\n", strings.TrimRight(week, " "))
				week = ""
			}
			cur = cur.AddDate(0, 0, 1)
		}
		if week != "" {
			fmt.Printf("  %s\n", strings.TrimRight(week, " "))
		}

		fmt.Println()
		fmt.Printf("  %s  active day  %s  today  %s  inactive\n",
			ui.StyleSuccess.Render("█"),
			ui.StyleSuccess.Render("◉"),
			ui.StyleDimmed.Render("░"),
		)
		fmt.Println("  " + sep)
		fmt.Println()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(streakCmd)
}
