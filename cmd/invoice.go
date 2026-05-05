package cmd

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/db"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var invoiceCmd = &cobra.Command{
	Use:   "invoice",
	Short: "Generate a billable invoice from sessions",
	Long: `Generate a formatted invoice from your tracked sessions.

Examples:
  btrack invoice -p myapp -r 150
  btrack invoice -p myapp -r 150 --month 2026-04
  btrack invoice -r 100 --round
  btrack invoice -p myapp -r 150 --out invoice.md

Flags:
  -p, --project   filter by project
  -r, --rate      hourly rate in $ (overrides config)
  -m, --month     month to invoice: YYYY-MM (default: current month)
  -o, --out       output file (default: stdout)
      --round     round durations to nearest 15 minutes

Set a default rate per project:
  btrack config project myapp rate 150`,
	RunE: runInvoice,
}

func runInvoice(cmd *cobra.Command, args []string) error {
	project, _ := cmd.Flags().GetString("project")
	rate, _ := cmd.Flags().GetFloat64("rate")
	monthStr, _ := cmd.Flags().GetString("month")
	outPath, _ := cmd.Flags().GetString("out")
	doRound, _ := cmd.Flags().GetBool("round")

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Resolve rate: flag → config → error
	if rate == 0 && project != "" && cfg.Projects != nil {
		if pc, ok := cfg.Projects[project]; ok {
			rate = pc.Rate
		}
	}
	if rate == 0 {
		return fmt.Errorf("hourly rate required — use -r 150 or set with: btrack config project %s rate 150", project)
	}

	// Parse month filter
	var monthStart, monthEnd time.Time
	if monthStr == "" {
		now := time.Now()
		monthStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
		monthEnd = monthStart.AddDate(0, 1, 0)
		monthStr = now.Format("January 2006")
	} else {
		t, err := time.ParseInLocation("2006-01", monthStr, time.Local)
		if err != nil {
			return fmt.Errorf("invalid month %q — use YYYY-MM format", monthStr)
		}
		monthStart = t
		monthEnd = t.AddDate(0, 1, 0)
		monthStr = t.Format("January 2006")
	}

	store, err := db.Open(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	// Fetch and filter sessions
	var sessions []*db.Session
	if project != "" {
		sessions, err = store.GetSessionsByProject(project, 5000)
	} else {
		sessions, err = store.GetRecentSessions(5000)
	}
	if err != nil {
		return fmt.Errorf("load sessions: %w", err)
	}

	// Filter to month and completed only
	var filtered []*db.Session
	for _, s := range sessions {
		if s.EndTime == nil {
			continue
		}
		st := s.StartTime.Local()
		if st.Before(monthStart) || !st.Before(monthEnd) {
			continue
		}
		filtered = append(filtered, s)
	}

	if len(filtered) == 0 {
		fmt.Println(ui.StyleSubtle.Render("\n  no sessions found for this period\n"))
		return nil
	}

	// Build output
	var sb strings.Builder
	sep := strings.Repeat("─", 62)

	sb.WriteString("\n")
	if project != "" {
		sb.WriteString(fmt.Sprintf("  Project: %-30s Rate: $%.2f/h\n", project, rate))
	} else {
		sb.WriteString(fmt.Sprintf("  Rate: $%.2f/h\n", rate))
	}
	sb.WriteString(fmt.Sprintf("  Period:  %s\n", monthStr))
	sb.WriteString("  " + sep + "\n")
	sb.WriteString(fmt.Sprintf("  %-12s  %-36s  %s\n", "Date", "Task", "Hours"))
	sb.WriteString("  " + sep + "\n")

	var totalDur time.Duration
	for _, s := range filtered {
		d := s.Duration()
		if doRound {
			d = roundTo15(d)
		}
		totalDur += d

		date := s.StartTime.Local().Format("2006-01-02")
		task := s.TaskName
		if len(task) > 34 {
			task = task[:31] + "..."
		}
		hours := d.Hours()
		sb.WriteString(fmt.Sprintf("  %-12s  %-36s  %.2f\n", date, task, hours))
	}

	totalHours := totalDur.Hours()
	amount := totalHours * rate

	sb.WriteString("  " + sep + "\n")
	sb.WriteString(fmt.Sprintf("\n  Total hours:  %.2f\n", totalHours))
	sb.WriteString(fmt.Sprintf("  Total amount: $%.2f\n\n", amount))

	output := sb.String()

	if outPath != "" {
		if err := os.WriteFile(outPath, []byte(output), 0644); err != nil {
			return fmt.Errorf("write file: %w", err)
		}
		fmt.Printf("\n  %s  invoice written to %s\n\n",
			ui.StyleSuccess.Render("✓"),
			ui.StyleHighlight.Render(outPath),
		)
		return nil
	}

	fmt.Print(output)
	return nil
}

// roundTo15 rounds a duration to the nearest 15-minute boundary.
func roundTo15(d time.Duration) time.Duration {
	const quarter = 15 * time.Minute
	return time.Duration(math.Round(float64(d)/float64(quarter))) * quarter
}

func init() {
	invoiceCmd.Flags().StringP("project", "p", "", "filter by project")
	invoiceCmd.Flags().Float64P("rate", "r", 0, "hourly rate in $ (overrides config)")
	invoiceCmd.Flags().StringP("month", "m", "", "month to invoice: YYYY-MM (default: current month)")
	invoiceCmd.Flags().StringP("out", "o", "", "output file path (default: stdout)")
	invoiceCmd.Flags().Bool("round", false, "round durations to nearest 15 minutes")
	rootCmd.AddCommand(invoiceCmd)
}
