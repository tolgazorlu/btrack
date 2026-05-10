package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/ai"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/daemon"
	"github.com/tolgazorlu/btrack/internal/gcal"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var stopCmd = &cobra.Command{
	Use:     "stop",
	Aliases: []string{"x"},
	Short:   "Stop the active session",
	Long: `Stop the active session. The closing message is optional.

Usage:
  btrack stop                stop without a message
  btrack stop -m "message"   stop with a message
  btrack x -m "message"      short alias
  btrack x --at "2h ago"     backdate the stop time

Examples:
  btrack x                                  (AI suggests a message; skip = save without one)
  btrack x --no-ai                          (skip AI, save without a message)
  btrack x -m "fixed JWT expiry #bugfix"
  btrack x --at "2h ago"                    (forgot to stop earlier today)
  btrack x --at "yesterday 18:00"           (forgot to stop yesterday)
  btrack x --at "15:30"                     (today at 15:30)

Flags:
  -m, --message   Closing message (optional)
      --no-ai     Skip AI message suggestion
      --at        Backdate the stop time (relative or absolute)

Tips:
  · Add #tags at the end to categorize your work
  · Common tags: #bugfix #feature #test #docs #refactor #ci
  · btrack shipped to compare what you said vs what landed in git`,
	RunE: func(cmd *cobra.Command, args []string) error {
		message, _ := cmd.Flags().GetString("message")
		noAI, _ := cmd.Flags().GetBool("no-ai")
		atRaw, _ := cmd.Flags().GetString("at")

		var atRFC3339 string
		if atRaw != "" {
			t, err := parseAtTime(atRaw, time.Now())
			if err != nil {
				return fmt.Errorf("invalid --at value %q: %w", atRaw, err)
			}
			atRFC3339 = t.Format(time.RFC3339)
		}

		if message == "" && !noAI {
			message = suggestMessage()
		}

		// Collect tags that should be appended to the closing message:
		// 1. AI-suggested category tags from the message itself
		// 2. default_tags from the .btrack project file (if any)
		// All deduplicated against tags the user already typed.
		existingTags := map[string]bool{}
		for _, w := range strings.Fields(message) {
			if strings.HasPrefix(w, "#") {
				existingTags[strings.ToLower(w)] = true
			}
		}

		var autoTags []string
		if message != "" {
			autoTags = append(autoTags, ai.CategorizeTask(message)...)
		}
		if cwd, err := os.Getwd(); err == nil {
			if pf, _ := config.FindProjectFile(cwd); pf != nil {
				autoTags = append(autoTags, pf.DefaultTags...)
			}
		}
		for _, tag := range autoTags {
			tag = strings.ToLower(tag)
			if existingTags[tag] {
				continue
			}
			existingTags[tag] = true
			if message == "" {
				message = tag
			} else {
				message += " " + tag
			}
		}

		payload := daemon.StopPayload{Message: message, EndTime: atRFC3339}
		client := daemon.NewClient()
		resp, err := client.Send(daemon.ActionStop, payload)
		if err != nil {
			return err
		}
		if !resp.Success {
			return fmt.Errorf("%s", resp.Error)
		}

		var sess daemon.SessionDTO
		if err := json.Unmarshal(resp.Data, &sess); err != nil {
			return fmt.Errorf("parse daemon response: %w", err)
		}

		start, _ := time.Parse(time.RFC3339, sess.StartTime)
		end, _ := time.Parse(time.RFC3339, sess.EndTime)
		if end.IsZero() {
			end = time.Now()
		}
		elapsed := end.Sub(start)

		ui.Blank()
		ui.Sign(ui.StyleSuccess.Render(ui.Sym.Stop), ui.StyleHighlight.Render(sess.TaskName))
		ui.KV("duration", ui.FormatDuration(elapsed))
		if message != "" {
			ui.KV("message", ui.StyleHighlight.Render(message))
		}

		if cfg, err := config.Load(); err == nil && cfg.GCal.AutoSync && cfg.GCal.ClientID != "" {
			fmt.Printf("%s%s  syncing to Google Calendar…\r", ui.Indent, ui.StyleDimmed.Render(ui.Sym.Up))
			svc, err := gcal.NewService(cfg.GCal.ClientID, cfg.GCal.ClientSecret, config.DataDir())
			if err == nil {
				_, err := gcal.PushSession(svc, cfg.GCal.CalendarID, sess.TaskName, sess.Project, start, end)
				if err == nil {
					ui.Sign(ui.StyleSuccess.Render(ui.Sym.Up), ui.StyleDimmed.Render("synced to Google Calendar"))
				}
			}
		}
		ui.Blank()
		return nil
	},
}

func init() {
	stopCmd.Flags().StringP("message", "m", "", "closing message for the session (optional)")
	stopCmd.Flags().Bool("no-ai", false, "skip AI message suggestion")
	stopCmd.Flags().String("at", "", "backdate the stop time (e.g. \"2h ago\", \"15:30\", \"yesterday 18:00\", or RFC3339)")
	rootCmd.AddCommand(stopCmd)
}

// parseAtTime accepts a few forgiving formats for backdating a stop:
//   - duration ago:    "2h ago", "30m ago", "1h30m ago"
//   - bare duration:   "2h", "30m", "1h30m"  (treated as "<dur> ago")
//   - HH:MM today:     "15:30"
//   - yesterday HH:MM: "yesterday 18:00"
//   - absolute:        RFC3339, e.g. "2026-05-10T15:30:00Z"
func parseAtTime(raw string, now time.Time) (time.Time, error) {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return time.Time{}, fmt.Errorf("empty value")
	}

	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}

	if rest, ok := strings.CutSuffix(s, " ago"); ok {
		d, err := time.ParseDuration(strings.TrimSpace(rest))
		if err != nil {
			return time.Time{}, fmt.Errorf("parse duration: %w", err)
		}
		return now.Add(-d), nil
	}

	if d, err := time.ParseDuration(s); err == nil {
		return now.Add(-d), nil
	}

	parseHHMM := func(v string) (h, m int, err error) {
		parts := strings.Split(v, ":")
		if len(parts) != 2 {
			return 0, 0, fmt.Errorf("expected HH:MM")
		}
		h, err = strconv.Atoi(parts[0])
		if err != nil || h < 0 || h > 23 {
			return 0, 0, fmt.Errorf("invalid hour")
		}
		m, err = strconv.Atoi(parts[1])
		if err != nil || m < 0 || m > 59 {
			return 0, 0, fmt.Errorf("invalid minute")
		}
		return h, m, nil
	}

	if rest, ok := strings.CutPrefix(s, "yesterday "); ok {
		h, m, err := parseHHMM(strings.TrimSpace(rest))
		if err != nil {
			return time.Time{}, err
		}
		y := now.AddDate(0, 0, -1)
		return time.Date(y.Year(), y.Month(), y.Day(), h, m, 0, 0, now.Location()), nil
	}

	if h, m, err := parseHHMM(s); err == nil {
		t := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
		// If user typed a future time (e.g. it's 09:00 and they said 18:00),
		// assume they meant yesterday.
		if t.After(now) {
			t = t.AddDate(0, 0, -1)
		}
		return t, nil
	}

	return time.Time{}, fmt.Errorf("unrecognized time format")
}

func suggestMessage() string {
	cfg, err := config.Load()
	if err != nil || cfg.AI.ActiveKey() == "" {
		return ""
	}

	provider, err := ai.NewProvider(cfg)
	if err != nil {
		return ""
	}

	client := daemon.NewClient()
	resp, err := client.Send(daemon.ActionStatus, nil)
	if err != nil || !resp.Success {
		return ""
	}
	var status daemon.StatusData
	if err := json.Unmarshal(resp.Data, &status); err != nil || !status.Active {
		return ""
	}

	var notes []string
	for _, l := range status.RecentLog {
		notes = append(notes, l.Note)
	}

	fmt.Print(ui.Indent + ui.StyleDimmed.Render("✦ asking AI for a commit message…\r"))
	aiCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	suggestion, err := ai.SuggestCommitMessage(aiCtx, provider,
		status.Session.TaskName, notes)
	fmt.Print("                                                     \r")

	if err != nil {
		return ""
	}
	ui.Blank()
	ui.KV("ai suggests", ui.StyleHighlight.Render(suggestion))
	fmt.Printf("%s%s ", ui.Indent, ui.StyleDimmed.Render("use this? [y/N] "))

	var input string
	fmt.Scanln(&input)
	if strings.ToLower(strings.TrimSpace(input)) == "y" {
		return suggestion
	}
	return ""
}
