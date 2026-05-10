package cmd

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// btrackSkillFS contains the entire skills/btrack/ tree (SKILL.md + README +
// metadata.json + scripts/ + references/), embedded at build time.
//
// The "all:" prefix is required so files starting with "_" or "." (none today,
// but defensive) are included.
//
//go:embed all:skill_data/btrack
var btrackSkillFS embed.FS

const (
	skillName      = "btrack"
	skillEmbedRoot = "skill_data/btrack"
)

func btrackSkillMD() []byte {
	data, err := btrackSkillFS.ReadFile(skillEmbedRoot + "/SKILL.md")
	if err != nil {
		panic(fmt.Sprintf("embedded SKILL.md missing: %v", err))
	}
	return data
}

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage the btrack Claude Code skill",
	Long: `Install or inspect the btrack skill bundled with this binary.

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

type skillInstallStatus int

const (
	skillInstalled skillInstallStatus = iota
	skillUpToDate
	skillBlockedByExisting
)

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
		if string(existing) == string(btrackSkillMD()) {
			return skillUpToDate, skillFile, nil
		}
		if !force {
			return skillBlockedByExisting, skillFile, nil
		}
	}

	if err := writeEmbeddedSkillTree(dest); err != nil {
		return skillInstalled, skillFile, err
	}
	return skillInstalled, skillFile, nil
}

func writeEmbeddedSkillTree(dest string) error {
	return fs.WalkDir(btrackSkillFS, skillEmbedRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(path, skillEmbedRoot)
		rel = strings.TrimPrefix(rel, "/")
		target := filepath.Join(dest, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		data, err := btrackSkillFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}

		mode := os.FileMode(0o644)
		if strings.HasPrefix(rel, "scripts/") && strings.HasSuffix(rel, ".sh") {
			mode = 0o755
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, mode); err != nil {
			return fmt.Errorf("write %s: %w", target, err)
		}
		return nil
	})
}

var skillInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the btrack skill into ~/.claude/skills/btrack/",
	Long: `Install the bundled btrack skill into the Claude Code skills
directory (default: ~/.claude/skills/btrack/).

The skill markdown plus its README, metadata.json, scripts/, and references/
are embedded in this binary, so the install matches the btrack version you're
running. Run again after upgrading btrack to refresh.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		dir, _ := cmd.Flags().GetString("dir")

		status, skillFile, err := installSkill(dir, force)
		if err != nil {
			return err
		}
		switch status {
		case skillUpToDate:
			fmt.Printf("btrack skill already up to date at %s\n", skillFile)
		case skillBlockedByExisting:
			fmt.Printf("Skill already installed at %s but content differs.\n", skillFile)
			fmt.Println("Use --force to overwrite.")
		case skillInstalled:
			destDir := filepath.Dir(skillFile)
			fmt.Printf("Installed btrack skill at %s\n\n", destDir)
			fmt.Println("Next steps:")
			fmt.Println("  1. Make sure the MCP is registered with your client. For Claude Code:")
			fmt.Println("       claude mcp add btrack -- btrack mcp")
			fmt.Println("  2. Fully quit and reopen Claude Code so it loads the skill + MCP.")
			fmt.Println("  3. Start coding — Claude will begin tracking sessions automatically.")
			fmt.Println()
			fmt.Printf("  (or run %s/scripts/setup.sh to do step 1 for you)\n", destDir)
		}
		return nil
	},
}

var skillPrintCmd = &cobra.Command{
	Use:           "print",
	Short:         "Print the embedded SKILL.md to stdout",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := os.Stdout.Write(btrackSkillMD())
		return err
	},
}

var skillPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the install path for the btrack skill",
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
