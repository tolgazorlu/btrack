package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/db"
	"github.com/tolgazorlu/btrack/internal/ledger"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var (
	recapDay, recapWeek, recapMonth, recapYear bool
	recapLast                                  int
	recapSince, recapUntil, recapProject       string
	recapMarkdown                              bool
)

var recapCmd = &cobra.Command{
	Use:   "recap",
	Short: "What you worked on, for how long, and what it produced",
	Long: `Summarise a period of work per project: your time, the agents' time,
sessions, commits and what they were, and estimated AI cost.

It combines sessions you logged by hand with agent sessions imported by
btrack sync. Your time per project per day is the larger of the two, so
work that was both logged and done through an agent is never counted
twice. Agent sessions left open for days count on the days they were
actually active.

Run btrack sync first to pull in the latest agent sessions and commits.

Examples:
  btrack recap                  # this week (default)
  btrack recap -d               # today
  btrack recap -m               # this month
  btrack recap --last 30        # the last 30 days
  btrack recap --since 2026-09-01 --until 2026-09-30 -p acme
  btrack recap -w --md          # markdown, for a standup or a journal
  btrack recap -m --json        # for scripts and agents`,
	RunE: runRecap,
}

func recapRange(now time.Time) (time.Time, time.Time, string, error) {
	spec := ledger.PeriodSpec{LastDays: recapLast, Since: recapSince, Until: recapUntil}
	switch {
	case recapDay:
		spec.Kind = "day"
	case recapMonth:
		spec.Kind = "month"
	case recapYear:
		spec.Kind = "year"
	}
	return spec.Resolve(now)
}

func runRecap(cmd *cobra.Command, args []string) error {
	from, to, label, err := recapRange(time.Now())
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	store, err := db.Open(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	rc, err := ledger.LoadRecap(store, from, to, recapProject)
	if err != nil {
		return err
	}

	switch {
	case jsonOutput:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rc)
	case recapMarkdown:
		printRecapMarkdown(rc, label)
	default:
		printRecap(rc, label)
	}
	return nil
}

func periodTitle(rc ledger.Recap, label string) string {
	last := rc.To.AddDate(0, 0, -1)
	span := rc.From.Format("Jan 2")
	if !last.Equal(rc.From) {
		span += " – " + last.Format("Jan 2")
	}
	if rc.From.IsZero() {
		span = "until " + last.Format("Jan 2, 2006")
	}
	if label != "" {
		return label + "  ·  " + span
	}
	return span
}

func recapStats(p ledger.ProjectRecap) string {
	parts := []string{shortDur(p.TimeSeconds) + " you"}
	if p.AgentSeconds > 0 {
		parts = append(parts, shortDur(p.AgentSeconds)+" agent")
	}
	parts = append(parts, plural(p.Sessions, "session"))
	if p.Commits > 0 {
		parts = append(parts, fmt.Sprintf("%s (+%d −%d)", plural(p.Commits, "commit"), p.Insertions, p.Deletions))
	}
	if p.CostUSD > 0 || !p.CostComplete {
		c := fmt.Sprintf("~$%.0f", p.CostUSD)
		if !p.CostComplete {
			c += "+"
		}
		parts = append(parts, c)
	}
	return strings.Join(parts, "  ·  ")
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func printRecap(rc ledger.Recap, label string) {
	ui.Header("recap", periodTitle(rc, label))
	if len(rc.Projects) == 0 {
		ui.Hint("nothing recorded in this period — run `btrack sync` to import agent sessions")
		ui.Blank()
		return
	}
	for _, p := range rc.Projects {
		ui.Plain(ui.StyleTitle.Render(p.Project))
		ui.Dim(recapStats(p))
		for _, h := range p.Highlights {
			ui.Plain("  · " + truncate(h, 72))
		}
		if extra := p.Commits - len(p.Highlights); p.Commits > 0 && extra > 0 {
			ui.Dim(fmt.Sprintf("  … and %d more commits", extra))
		}
		ui.Blank()
	}
	ui.Rule()
	ui.Plain("total  " + recapStats(rc.Total))
	ui.Dim("you = the larger of hand-logged and measured keyboard time per day; agent = agent working alone.")
	ui.Dim("Both measured times are heuristic. Cost is estimated at API list prices.")
	ui.Blank()
}

func printRecapMarkdown(rc ledger.Recap, label string) {
	fmt.Printf("## Recap: %s\n\n", periodTitle(rc, label))
	for _, p := range rc.Projects {
		fmt.Printf("### %s\n\n%s\n\n", p.Project, recapStats(p))
		for _, h := range p.Highlights {
			fmt.Printf("- %s\n", h)
		}
		if len(p.Highlights) > 0 {
			fmt.Println()
		}
	}
	fmt.Printf("**Total:** %s\n", recapStats(rc.Total))
}

func init() {
	f := recapCmd.Flags()
	f.BoolVarP(&recapDay, "day", "d", false, "Today")
	f.BoolVarP(&recapWeek, "week", "w", false, "This week (default)")
	f.BoolVarP(&recapMonth, "month", "m", false, "This month")
	f.BoolVarP(&recapYear, "year", "y", false, "This year")
	f.IntVar(&recapLast, "last", 0, "The last N days, including today")
	f.StringVar(&recapSince, "since", "", "Start date, YYYY-MM-DD")
	f.StringVar(&recapUntil, "until", "", "End date, YYYY-MM-DD, inclusive")
	f.StringVarP(&recapProject, "project", "p", "", "Only this project")
	f.BoolVar(&recapMarkdown, "md", false, "Print as Markdown")
	rootCmd.AddCommand(recapCmd)
}
