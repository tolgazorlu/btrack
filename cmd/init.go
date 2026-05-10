package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create or edit the .btrack project file in the current directory",
	Long: `Interactive wizard that creates (or edits) a .btrack file in the
current directory. Re-run any time to update — existing values become the
defaults so you only change what's different.

The .btrack file sets per-repo context that is picked up automatically
whenever you start a session from anywhere inside the directory tree.

Fields:
  project       — default project name for every session started here
  task_prefix   — text prepended to every task name (e.g. "[myapp]")
  description   — short note on what this project is (used by AI features)
  daily_hours   — override the global daily hour target for this project
  billing_rate  — hourly rate ($), used by ` + "`btrack invoice`" + `
  default_tags  — comma-separated tags auto-applied to every session

Example .btrack file:
  project       = myapp
  task_prefix   = [myapp]
  description   = Customer-facing web app, Next.js + Postgres
  daily_hours   = 6
  billing_rate  = 150.00
  default_tags  = #frontend, #web

Flags:
  -y, --yes     accept all suggested defaults without prompting (CI-friendly)`,
	RunE: runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	target := filepath.Join(cwd, ".btrack")
	yes, _ := cmd.Flags().GetBool("yes")

	// Load any existing .btrack so re-running this command acts as "edit"
	// and only asks the user what's different.
	existing, _ := config.FindProjectFile(cwd)
	if existing == nil {
		existing = &config.ProjectFile{}
	}
	editing := false
	if _, err := os.Stat(target); err == nil {
		editing = true
	}

	reader := bufio.NewReader(os.Stdin)

	subtitle := filepath.Base(cwd)
	if editing {
		subtitle += "  (editing existing .btrack)"
	}
	ui.Header("init", subtitle)
	ui.Blank()

	// ── identity ────────────────────────────────────────────────────────────
	ui.Section("identity")
	ui.Hint("how this project shows up in btrack history, stats, and invoices")

	suggestedProject := existing.Project
	if suggestedProject == "" {
		suggestedProject = filepath.Base(cwd)
		if repo := gitRepo(); repo != "" {
			suggestedProject = repo
		}
	}
	project := promptOr(reader, yes, "project name", suggestedProject)

	taskPrefix := promptOr(reader, yes, "task prefix (e.g. \"[myapp]\", blank to skip)", existing.TaskPrefix)
	description := promptOr(reader, yes, "description (one line, blank to skip)", existing.Description)
	ui.Blank()

	// ── work targets ────────────────────────────────────────────────────────
	ui.Section("work")
	ui.Hint("overrides for this project; leave blank to use your global config")

	dailyHoursStr := promptOr(reader, yes, "daily hours target (blank = global)", optionalInt(existing.DailyHours))
	dailyHours, err := parseOptionalPositiveInt(dailyHoursStr)
	if err != nil {
		return fmt.Errorf("daily_hours: %w", err)
	}

	billingStr := promptOr(reader, yes, "billing rate $/hour (blank = none)", optionalFloat(existing.BillingRate))
	billingRate, err := parseOptionalNonNegFloat(billingStr)
	if err != nil {
		return fmt.Errorf("billing_rate: %w", err)
	}
	ui.Blank()

	// ── auto-tagging ────────────────────────────────────────────────────────
	ui.Section("tagging")
	ui.Hint("tags applied to every session stopped in this project")

	defaultTagsStr := promptOr(reader, yes,
		"default tags (comma-separated, e.g. \"frontend, react\")",
		strings.Join(existing.DefaultTags, ", "),
	)
	defaultTags := config.ParseTagList(defaultTagsStr)
	ui.Blank()

	// ── preview + confirm ───────────────────────────────────────────────────
	pf := &config.ProjectFile{
		Project:     project,
		TaskPrefix:  taskPrefix,
		Description: description,
		DailyHours:  dailyHours,
		BillingRate: billingRate,
		DefaultTags: defaultTags,
	}

	ui.Section("preview")
	rendered := pf.Render()
	for _, line := range strings.Split(strings.TrimRight(rendered, "\n"), "\n") {
		fmt.Println(ui.Indent + ui.StyleDimmed.Render(line))
	}
	ui.Blank()

	if !yes {
		fmt.Printf("%s%s ", ui.Indent, ui.StyleDimmed.Render("write this to .btrack? [Y/n]"))
		var confirm string
		fmt.Scanln(&confirm)
		if c := strings.ToLower(strings.TrimSpace(confirm)); c == "n" || c == "no" {
			ui.Blank()
			ui.Warn("aborted — nothing written")
			ui.Blank()
			return nil
		}
	}

	if err := os.WriteFile(target, []byte(rendered), 0644); err != nil {
		return fmt.Errorf("write .btrack: %w", err)
	}

	ui.Blank()
	verb := "created"
	if editing {
		verb = "updated"
	}
	ui.OK(verb + " " + ui.StyleHighlight.Render(".btrack"))
	if pf.Project != "" {
		ui.KV("project", ui.StyleTag.Render("@"+pf.Project))
	}
	if pf.TaskPrefix != "" {
		ui.KV("prefix", ui.StyleHighlight.Render(pf.TaskPrefix))
	}
	if len(pf.DefaultTags) > 0 {
		ui.KV("auto-tags", ui.StyleHighlight.Render(strings.Join(pf.DefaultTags, " ")))
	}
	if pf.BillingRate > 0 {
		ui.KV("rate", ui.StyleHighlight.Render(fmt.Sprintf("$%.2f/h", pf.BillingRate)))
	}
	ui.Tip("commit .btrack to share project context with your team, or .gitignore it if it's personal")
	ui.Blank()
	return nil
}

// promptOr is prompt with --yes support: when yes is true, the default value
// is accepted silently with no stdin read (handy for scripted runs).
func promptOr(r *bufio.Reader, yes bool, question, defaultVal string) string {
	if yes {
		if defaultVal != "" {
			fmt.Printf("%s%s %s\n", ui.Indent,
				ui.StyleDimmed.Render(question),
				ui.StyleDimmed.Render("← "+defaultVal),
			)
		}
		return defaultVal
	}
	return prompt(r, question, defaultVal)
}

// prompt prints a question with a default value hint and reads a line from stdin.
func prompt(r *bufio.Reader, question, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("%s%s %s ", ui.Indent, ui.StyleDimmed.Render(question), ui.StyleDimmed.Render("["+defaultVal+"]:"))
	} else {
		fmt.Printf("%s%s: ", ui.Indent, ui.StyleDimmed.Render(question))
	}
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	return line
}

func optionalInt(n int) string {
	if n <= 0 {
		return ""
	}
	return strconv.Itoa(n)
}

func optionalFloat(f float64) string {
	if f <= 0 {
		return ""
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func parseOptionalPositiveInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("expected a positive integer, got %q", s)
	}
	return n, nil
}

func parseOptionalNonNegFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 {
		return 0, fmt.Errorf("expected a non-negative number, got %q", s)
	}
	return f, nil
}

func init() {
	initCmd.Flags().BoolP("yes", "y", false, "accept all suggested defaults without prompting")
	rootCmd.AddCommand(initCmd)
}
