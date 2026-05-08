package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/ai"
	"github.com/tolgazorlu/btrack/internal/db"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var standupCmd = &cobra.Command{
	Use:     "standup",
	Aliases: []string{"su"},
	Short:   "Generate yesterday's standup with AI",
	Long: `Use AI to write a standup from your tracked sessions.

Defaults to yesterday — run it in the morning before your standup meeting.

Examples:
  btrack standup              yesterday (default)
  btrack standup --today      today's sessions so far
  btrack standup --days 3     last 3 days

Setup:
  btrack ai setup             configure an AI key (required)
  btrack github connect       connect GitHub for richer output (optional)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		today, _ := cmd.Flags().GetBool("today")
		days, _ := cmd.Flags().GetInt("days")

		cfg, err := loadConfigWithAICheck()
		if err != nil {
			return err
		}
		store, err := db.Open(cfg)
		if err != nil {
			return err
		}
		defer store.Close()

		now := time.Now()
		var sessions []*db.Session
		var targetDate time.Time

		switch {
		case days > 1:
			targetDate = now
			sessions, err = store.GetRecentSessions(days * 15)
			if err != nil {
				return fmt.Errorf("load sessions: %w", err)
			}
			since := startOfDay(now.AddDate(0, 0, -(days - 1)))
			var filtered []*db.Session
			for _, s := range sessions {
				if !s.StartTime.Before(since) {
					filtered = append(filtered, s)
				}
			}
			sessions = filtered
		case today:
			targetDate = now
			sessions, err = store.GetSessionsForDate(now)
			if err != nil {
				return fmt.Errorf("load sessions: %w", err)
			}
		default:
			targetDate = now.AddDate(0, 0, -1)
			sessions, err = store.GetSessionsForDate(targetDate)
			if err != nil {
				return fmt.Errorf("load sessions: %w", err)
			}
		}

		if len(sessions) == 0 {
			ui.Blank()
			dateLabel := targetDate.Format("Monday, January 2")
			ui.Hint("no sessions found for " + dateLabel)
			ui.Hint("run `btrack start` to begin tracking")
			ui.Blank()
			return nil
		}

		logsMap := map[int64][]*db.LogEntry{}
		for _, s := range sessions {
			if logs, err := store.GetAllLogs(s.ID); err == nil {
				logsMap[s.ID] = logs
			}
		}

		provider, err := ai.NewProvider(cfg)
		if err != nil {
			return err
		}

		githubActivity := ""
		if ghClient := ghClientFromConfig(cfg); ghClient != nil {
			since := startOfDay(targetDate)
			until := since.Add(24 * time.Hour)
			if days > 1 {
				since = startOfDay(now.AddDate(0, 0, -(days - 1)))
				until = now
			}
			if act, err := ghClient.GetActivity(since.UTC(), until.UTC()); err == nil && !act.IsEmpty() {
				githubActivity = act.Summary()
			}
		}

		ui.Blank()
		ui.Dim("✦ generating standup…")
		ui.Blank()

		aiCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		summary, err := ai.GenerateStandup(aiCtx, provider, sessions, logsMap, targetDate)
		if err != nil {
			return fmt.Errorf("AI error: %w", err)
		}

		if githubActivity != "" {
			summary += "\n\nGitHub:\n  • " + githubActivity
		}

		title := "Standup  ·  " + targetDate.Format("Mon Jan 2") + "  ·  " + provider.Name()
		fmt.Println(ui.RenderBox(title, summary))
		return nil
	},
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Local().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

func init() {
	standupCmd.Flags().Bool("today", false, "use today's sessions instead of yesterday")
	standupCmd.Flags().IntP("days", "d", 1, "number of days to include")
	rootCmd.AddCommand(standupCmd)
}
