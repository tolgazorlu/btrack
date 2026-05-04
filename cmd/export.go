package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/db"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export sessions to CSV or JSON",
	Long: `Export your tracking sessions to a file or stdout.

Usage:
  btrack export                       CSV to stdout
  btrack export --format json         JSON to stdout
  btrack export --out sessions.csv    write to file
  btrack export --days 30             last 30 days only

Examples:
  btrack export > sessions.csv
  btrack export --format json | jq .
  btrack export --days 30 --out april.csv

Use cases:
  · Import into a spreadsheet for invoicing
  · Feed into a reporting tool
  · Back up your session data`,
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("format")
		outPath, _ := cmd.Flags().GetString("out")
		days, _ := cmd.Flags().GetInt("days")

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		store, err := db.Open(cfg)
		if err != nil {
			return err
		}
		defer store.Close()

		sessions, err := store.GetRecentSessions(10000)
		if err != nil {
			return fmt.Errorf("load sessions: %w", err)
		}

		if days > 0 {
			cutoff := time.Now().AddDate(0, 0, -days)
			var filtered []*db.Session
			for _, s := range sessions {
				if s.StartTime.After(cutoff) {
					filtered = append(filtered, s)
				}
			}
			sessions = filtered
		}

		out := os.Stdout
		if outPath != "" {
			f, err := os.Create(outPath)
			if err != nil {
				return fmt.Errorf("create file: %w", err)
			}
			defer f.Close()
			out = f
		}

		switch strings.ToLower(format) {
		case "json":
			err = exportJSON(sessions, out)
		default:
			err = exportCSV(sessions, out)
		}
		if err != nil {
			return err
		}

		if outPath != "" {
			fmt.Printf("\n  %s  exported %d sessions → %s\n\n",
				ui.StyleSuccess.Render("✓"),
				len(sessions),
				ui.StyleHighlight.Render(outPath),
			)
		}
		return nil
	},
}

type exportRow struct {
	ID              int64    `json:"id"`
	Task            string   `json:"task"`
	Date            string   `json:"date"`
	StartTime       string   `json:"start_time"`
	EndTime         string   `json:"end_time"`
	DurationMinutes string   `json:"duration_minutes"`
	Message         string   `json:"message"`
	Tags            []string `json:"tags"`
	GitBranch       string   `json:"git_branch"`
	GitRepo         string   `json:"git_repo"`
}

func sessionToRow(s *db.Session) exportRow {
	endStr := ""
	if s.EndTime != nil {
		endStr = s.EndTime.Local().Format("2006-01-02 15:04")
	}
	return exportRow{
		ID:              s.ID,
		Task:            s.TaskName,
		Date:            s.StartTime.Local().Format("2006-01-02"),
		StartTime:       s.StartTime.Local().Format("2006-01-02 15:04"),
		EndTime:         endStr,
		DurationMinutes: fmt.Sprintf("%.1f", s.Duration().Minutes()),
		Message:         s.Message,
		Tags:            s.Tags,
		GitBranch:       s.GitBranch,
		GitRepo:         s.GitRepo,
	}
}

func exportCSV(sessions []*db.Session, out *os.File) error {
	w := csv.NewWriter(out)
	_ = w.Write([]string{"id", "date", "start_time", "end_time", "duration_minutes", "task", "message", "tags", "git_branch", "git_repo"})
	for _, s := range sessions {
		r := sessionToRow(s)
		_ = w.Write([]string{
			fmt.Sprintf("%d", r.ID),
			r.Date, r.StartTime, r.EndTime, r.DurationMinutes,
			r.Task, r.Message, strings.Join(r.Tags, " "),
			r.GitBranch, r.GitRepo,
		})
	}
	w.Flush()
	return w.Error()
}

func exportJSON(sessions []*db.Session, out *os.File) error {
	rows := make([]exportRow, len(sessions))
	for i, s := range sessions {
		rows[i] = sessionToRow(s)
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

func init() {
	exportCmd.Flags().StringP("format", "f", "csv", "output format: csv or json")
	exportCmd.Flags().StringP("out", "o", "", "output file path (default stdout)")
	exportCmd.Flags().IntP("days", "n", 0, "number of past days to export (0 = all)")
	rootCmd.AddCommand(exportCmd)
}
