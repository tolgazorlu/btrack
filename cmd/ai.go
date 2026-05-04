package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/ai"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/db"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var aiCmd = &cobra.Command{
	Use:   "ai",
	Short: "AI-powered summaries and insights",
	Long: `Use AI to summarize your work and analyze productivity patterns.

Commands:
  btrack ai setup       Configure an API key (OpenAI, Claude, or Gemini)
  btrack ai summarize   Generate a standup summary from today's sessions
  btrack ai insights    Show a stats dashboard with AI analysis

Run any command with --help for details.`,
}

// ─── ai setup ────────────────────────────────────────────────────────────────

var aiSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure an AI provider key (interactive)",
	Long: `Walk through adding an API key for an AI provider.

Supported providers:
  · OpenAI  (GPT-4o)      — platform.openai.com/api-keys
  · Claude  (Sonnet 4.6)  — console.anthropic.com/settings/keys
  · Gemini  (2.0 Flash)   — aistudio.google.com/apikey

What happens:
  1. Pick a provider
  2. Paste your API key (masked input)
  3. Key is validated with a live API call
  4. Saved to ~/.config/btrack/config.yaml

You can add multiple providers and switch between them.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSetupWizard()
	},
}

func runSetupWizard() error {
	// Inject save function (avoids import cycle between ui ↔ config).
	ui.SetSaveKeyFunc(config.SaveProviderKey)

	model := ui.NewSetupModel()
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	m, ok := finalModel.(ui.SetupModel)
	if !ok || len(m.SavedProviders()) == 0 {
		return nil
	}

	cfg, err := config.Reload()
	if err != nil {
		return err
	}
	fmt.Printf("\n  %s  Active provider: %s\n\n",
		ui.StyleSuccess.Render("✓"),
		ui.StyleHighlight.Render(cfg.AI.Provider),
	)
	fmt.Printf("  %s\n\n",
		ui.StyleDimmed.Render("run `btrack ai insights` to see your productivity stats"),
	)
	return nil
}

// ─── ai summarize ────────────────────────────────────────────────────────────

var aiSummarizeCmd = &cobra.Command{
	Use:     "summarize",
	Aliases: []string{"sum", "s"},
	Short:   "Generate a standup summary from your sessions",
	Long: `Use AI to write a standup-ready summary of your recent work.

Examples:
  btrack ai summarize
  btrack ai summarize --days 3

Flags:
  -d, --days   Number of days to include (default 1 = today)

Tips:
  · The more notes you add with "btrack note", the better the summary
  · Requires AI key — run: btrack ai setup`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		sessions, err := store.GetRecentSessions(days * 10)
		if err != nil {
			return fmt.Errorf("load sessions: %w", err)
		}
		if len(sessions) == 0 {
			fmt.Println(ui.StyleSubtle.Render("\n  no sessions found\n"))
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

		fmt.Print(ui.StyleDimmed.Render("\n  ✦ generating standup...\n\n"))
		summary, err := ai.SummarizeStandup(context.Background(), provider, sessions, logsMap)
		if err != nil {
			return fmt.Errorf("AI error: %w", err)
		}

		fmt.Println(ui.RenderBox("Daily Standup  ·  "+provider.Name(), summary))
		return nil
	},
}

// ─── ai insights ─────────────────────────────────────────────────────────────

var aiInsightsCmd = &cobra.Command{
	Use:     "insights",
	Aliases: []string{"ins", "i"},
	Short:   "Show productivity dashboard with AI analysis",
	Long: `Display a stats dashboard and AI analysis of your work patterns.

Examples:
  btrack ai insights
  btrack ai insights --days 14
  btrack ai insights --no-ai    (stats only, no AI needed)

Flags:
  -d, --days   Days to analyze (default 7)
      --no-ai  Show stats only, skip AI analysis

What you'll see:
  · Summary: total sessions, time, avg and longest session
  · Daily activity chart with hours per day
  · Top tasks by time spent
  · Tag breakdown (#bugfix, #feature, etc.)
  · Hourly activity pattern (when you work best)
  · AI analysis of your patterns and suggestions`,
	RunE: func(cmd *cobra.Command, args []string) error {
		days, _ := cmd.Flags().GetInt("days")
		noAI, _ := cmd.Flags().GetBool("no-ai")

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		store, err := db.Open(cfg)
		if err != nil {
			return err
		}
		defer store.Close()

		sessions, err := store.GetRecentSessions(days * 20)
		if err != nil {
			return fmt.Errorf("load sessions: %w", err)
		}

		stats := db.ComputeStats(sessions, days)

		fmt.Println()
		printStatsHeader(stats, days)
		printDailyChart(stats)
		printTopTasks(stats)
		printTagBreakdown(stats)
		printPeakHours(stats)

		if noAI || cfg.AI.ActiveKey() == "" {
			if cfg.AI.ActiveKey() == "" {
				fmt.Printf("\n  %s\n\n",
					ui.StyleDimmed.Render("tip: run `btrack ai setup` to unlock AI insights"),
				)
			}
			return nil
		}

		provider, err := ai.NewProvider(cfg)
		if err != nil {
			fmt.Printf("\n  %s\n\n", ui.StyleDimmed.Render("AI unavailable: "+err.Error()))
			return nil
		}

		fmt.Print(ui.StyleDimmed.Render("\n  ✦ analysing with " + provider.Name() + "...\n\n"))
		analysis, err := ai.AnalyzeStats(context.Background(), provider, stats.ToJSON())
		if err != nil {
			fmt.Printf("  %s\n\n", ui.StyleWarning.Render("AI error: "+err.Error()))
			return nil
		}

		fmt.Println(ui.RenderBox("AI Insights  ·  "+provider.Name(), analysis))
		return nil
	},
}

// ─── stats rendering ─────────────────────────────────────────────────────────

func printStatsHeader(s *db.Stats, days int) {
	title := fmt.Sprintf("Stats — last %d days", days)
	var sb strings.Builder

	sb.WriteString(ui.RenderStat("total sessions",
		ui.StyleHighlight.Render(fmt.Sprintf("%d", s.TotalSessions))) + "\n")
	sb.WriteString(ui.RenderStat("total time",
		ui.StyleElapsed.Render(formatStatDur(s.TotalDuration))) + "\n")
	sb.WriteString(ui.RenderStat("avg session",
		ui.StyleSubtle.Render(formatStatDur(s.AvgDuration))) + "\n")
	sb.WriteString(ui.RenderStat("longest session",
		ui.StyleSubtle.Render(formatStatDur(s.LongestSession))) + "\n")

	fmt.Println(ui.RenderBox(title, sb.String()))
}

func printDailyChart(s *db.Stats) {
	if len(s.DailyBreakdown) == 0 {
		return
	}
	var maxDur time.Duration
	for _, d := range s.DailyBreakdown {
		if d.Duration > maxDur {
			maxDur = d.Duration
		}
	}

	var sb strings.Builder
	for _, d := range s.DailyBreakdown {
		label := d.Date.Format("Mon 02")
		if d.IsToday {
			label = "today   "
		}
		bar := ui.RenderBar(label, d.Duration.Hours(), maxDur.Hours(), 24)
		hrs := ""
		if d.Duration > 0 {
			hrs = ui.StyleDimmed.Render(fmt.Sprintf("  %s", formatStatDur(d.Duration)))
		}
		sb.WriteString(bar + hrs + "\n")
	}
	fmt.Println(ui.RenderBox("Daily Activity", sb.String()))
}

func printTopTasks(s *db.Stats) {
	if len(s.TopTasks) == 0 {
		return
	}
	maxDur := s.TopTasks[0].Duration
	var sb strings.Builder
	for _, t := range s.TopTasks {
		name := t.Name
		if len(name) > 22 {
			name = name[:19] + "..."
		}
		bar := ui.RenderBar(name, t.Duration.Hours(), maxDur.Hours(), 20)
		sb.WriteString(fmt.Sprintf("%s  %s  %s\n",
			bar,
			ui.StyleDimmed.Render(fmt.Sprintf("%d sessions", t.Sessions)),
			ui.StyleSubtle.Render(formatStatDur(t.Duration)),
		))
	}
	fmt.Println(ui.RenderBox("Top Tasks", sb.String()))
}

func printTagBreakdown(s *db.Stats) {
	if len(s.TagCounts) == 0 {
		return
	}
	maxCount := 0
	for _, c := range s.TagCounts {
		if c > maxCount {
			maxCount = c
		}
	}
	var sb strings.Builder
	for tag, count := range s.TagCounts {
		bar := ui.RenderBar(tag, float64(count), float64(maxCount), 20)
		sb.WriteString(fmt.Sprintf("%s  %s\n",
			bar,
			ui.StyleDimmed.Render(fmt.Sprintf("%d sessions", count)),
		))
	}
	fmt.Println(ui.RenderBox("Tags", sb.String()))
}

func printPeakHours(s *db.Stats) {
	maxCount := 0
	for _, c := range s.HourlyPattern {
		if c > maxCount {
			maxCount = c
		}
	}
	if maxCount == 0 {
		return
	}
	var sb strings.Builder
	// Show 6am–10pm only
	for h := 6; h <= 22; h++ {
		count := s.HourlyPattern[h]
		label := fmt.Sprintf("%02d:00", h)
		bar := ui.RenderBar(label, float64(count), float64(maxCount), 16)
		sb.WriteString(bar + "\n")
	}
	fmt.Println(ui.RenderBox("Hourly Activity Pattern", sb.String()))
}

func formatStatDur(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %02dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// loadConfigWithAICheck loads config and guides the user through setup if no key.
func loadConfigWithAICheck() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if cfg.AI.ActiveKey() == "" {
		fmt.Println(ui.StyleWarning.Render("\n  No AI key configured."))
		fmt.Printf("  %s\n\n",
			ui.StyleDimmed.Render("run `btrack ai setup` to add one — takes 30 seconds"),
		)
		return nil, fmt.Errorf("AI not configured")
	}
	return cfg, nil
}

func init() {
	aiSummarizeCmd.Flags().IntP("days", "d", 1, "number of days to include")
	aiInsightsCmd.Flags().IntP("days", "d", 7, "number of days to analyse")
	aiInsightsCmd.Flags().Bool("no-ai", false, "show stats only, skip AI analysis")

	aiCmd.AddCommand(aiSetupCmd, aiSummarizeCmd, aiInsightsCmd)
	rootCmd.AddCommand(aiCmd)
}
