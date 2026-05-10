package cmd

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

//go:embed skill_data/btrack-tracker/SKILL.md
var btrackSkillMD []byte

const skillName = "btrack-tracker"

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage the btrack-tracker Claude Code skill",
	Long: `Install or inspect the btrack-tracker skill bundled with this binary.

The skill teaches Claude Code (and other skill-aware clients) to:
  - start a btrack session before non-trivial coding work
  - drop checkpoint notes for non-obvious findings
  - stop the session right before "git commit" so "btrack shipped" lines
    sessions up with commits

The skill works alongside the MCP server. Once both are wired up:

  claude mcp add btrack -- btrack mcp
  btrack skill install

…restart Claude Code so it picks up both.`,
}

// skillInstallStatus describes the outcome of installSkill.
type skillInstallStatus int

const (
	skillInstalled skillInstallStatus = iota
	skillUpToDate
	skillBlockedByExisting
)

// installSkill writes the embedded SKILL.md into <dir>/btrack-tracker/SKILL.md.
// If a different SKILL.md is already present and force is false, it returns
// skillBlockedByExisting and does not overwrite. The skillFile path is always
// returned so callers can print user-facing guidance.
func installSkill(dir string, force bool) (status skillInstallStatus, skillFile string, err error) {
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return skillInstalled, "", fmt.Errorf("resolve home dir: %w", err)
		}
		dir = filepath.Join(home, ".claude", "skills")
	}

	dest := filepath.Join(dir, skillName)
	skillFile = filepath.Join(dest, "SKILL.md")

	if existing, err := os.ReadFile(skillFile); err == nil {
		if string(existing) == string(btrackSkillMD) {
			return skillUpToDate, skillFile, nil
		}
		if !force {
			return skillBlockedByExisting, skillFile, nil
		}
	}

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return skillInstalled, skillFile, fmt.Errorf("create %s: %w", dest, err)
	}
	if err := os.WriteFile(skillFile, btrackSkillMD, 0o644); err != nil {
		return skillInstalled, skillFile, fmt.Errorf("write %s: %w", skillFile, err)
	}
	return skillInstalled, skillFile, nil
}

var skillInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the btrack-tracker skill into ~/.claude/skills/",
	Long: `Install the bundled btrack-tracker skill into the Claude Code skills
directory (default: ~/.claude/skills/btrack-tracker/SKILL.md).

The skill markdown is embedded in this binary, so the install matches the
btrack version you're running. Run again after upgrading btrack to refresh.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		dir, _ := cmd.Flags().GetString("dir")

		status, skillFile, err := installSkill(dir, force)
		if err != nil {
			return err
		}
		switch status {
		case skillUpToDate:
			fmt.Printf("btrack-tracker skill already up to date at %s\n", skillFile)
		case skillBlockedByExisting:
			fmt.Printf("Skill already installed at %s but content differs.\n", skillFile)
			fmt.Println("Use --force to overwrite.")
		case skillInstalled:
			fmt.Printf("Installed btrack-tracker skill at %s\n\n", skillFile)
			fmt.Println("Next steps:")
			fmt.Println("  1. Make sure the MCP is registered with your client. For Claude Code:")
			fmt.Println("       claude mcp add btrack -- btrack mcp")
			fmt.Println("  2. Fully quit and reopen Claude Code so it loads the skill + MCP.")
			fmt.Println("  3. Start coding — Claude will begin tracking sessions automatically.")
		}
		return nil
	},
}

var skillPrintCmd = &cobra.Command{
	Use:           "print",
	Short:         "Print the embedded skill markdown to stdout",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := os.Stdout.Write(btrackSkillMD)
		return err
	},
}

var skillPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the install path for the btrack-tracker skill",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("dir")
		if dir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			dir = filepath.Join(home, ".claude", "skills")
		}
		fmt.Println(filepath.Join(dir, skillName, "SKILL.md"))
		return nil
	},
}

func init() {
	skillInstallCmd.Flags().Bool("force", false, "overwrite an existing SKILL.md if its content differs")
	skillInstallCmd.Flags().String("dir", "", "skills directory (default ~/.claude/skills)")
	skillPathCmd.Flags().String("dir", "", "skills directory (default ~/.claude/skills)")

	skillCmd.AddCommand(skillInstallCmd)
	skillCmd.AddCommand(skillPrintCmd)
	skillCmd.AddCommand(skillPathCmd)
	rootCmd.AddCommand(skillCmd)
}
