package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a .btrack project file in the current directory",
	Long: `Interactive wizard that creates a .btrack file in the current directory.

The .btrack file sets per-repo defaults that are picked up automatically when
you start a session from anywhere inside that directory tree.

Fields:
  project     — default project name for every session started here
  task_prefix — text prepended to every task name (e.g. "[myapp]")
  daily_hours — override your global daily hour target for this project

Example .btrack file:
  project     = myapp
  task_prefix = [myapp]
  daily_hours = 6`,
	RunE: runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	target := filepath.Join(cwd, ".btrack")

	// Warn if file already exists.
	if _, err := os.Stat(target); err == nil {
		ui.Blank()
		ui.Warn(".btrack already exists in this directory")
		fmt.Printf("%s%s ", ui.Indent, ui.StyleDimmed.Render("overwrite? [y/N]"))
		var yn string
		fmt.Scanln(&yn)
		if strings.ToLower(strings.TrimSpace(yn)) != "y" {
			ui.Blank()
			return nil
		}
	}

	reader := bufio.NewReader(os.Stdin)

	ui.Header("init", filepath.Base(cwd))

	// Suggest project name from git repo or directory name.
	suggestedProject := filepath.Base(cwd)
	if repo := gitRepo(); repo != "" {
		suggestedProject = repo
	}

	project := prompt(reader, "project name", suggestedProject)
	taskPrefix := prompt(reader, "task prefix (leave blank to skip)", "")
	dailyHoursStr := prompt(reader, "daily hours target (leave blank for global default)", "")

	// Build file content.
	var sb strings.Builder
	sb.WriteString("# btrack project config — https://github.com/tolgazorlu/btrack\n")
	if project != "" {
		sb.WriteString(fmt.Sprintf("project     = %s\n", project))
	}
	if taskPrefix != "" {
		sb.WriteString(fmt.Sprintf("task_prefix = %s\n", taskPrefix))
	}
	if n, err := strconv.Atoi(dailyHoursStr); err == nil && n > 0 {
		sb.WriteString(fmt.Sprintf("daily_hours = %d\n", n))
	}

	if err := os.WriteFile(target, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("write .btrack: %w", err)
	}

	ui.Blank()
	ui.OK("created " + ui.StyleHighlight.Render(".btrack"))
	if project != "" {
		ui.KV("project", ui.StyleTag.Render("@"+project))
	}
	if taskPrefix != "" {
		ui.KV("prefix", ui.StyleHighlight.Render(taskPrefix))
	}
	ui.Tip("add .btrack to .gitignore or commit it to share with your team")
	ui.Blank()
	return nil
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

func init() {
	rootCmd.AddCommand(initCmd)
}
