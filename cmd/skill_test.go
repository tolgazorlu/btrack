package cmd

import (
	"os"
	"testing"
)

// TestEmbeddedSkillMatchesCanonical fails if the SKILL.md embedded in the
// binary drifts from the canonical copy at .claude/skills/btrack-tracker/SKILL.md.
// Run `make sync-skill` to re-sync.
func TestEmbeddedSkillMatchesCanonical(t *testing.T) {
	canonical, err := os.ReadFile("../.claude/skills/btrack-tracker/SKILL.md")
	if err != nil {
		t.Fatalf("read canonical SKILL.md: %v", err)
	}
	if string(canonical) != string(btrackSkillMD) {
		t.Fatalf("embedded SKILL.md is out of sync with .claude/skills/btrack-tracker/SKILL.md.\n" +
			"Run `make sync-skill` and rebuild.")
	}
}

func TestInstallSkillWritesEmbeddedContent(t *testing.T) {
	tmp := t.TempDir()
	status, path, err := installSkill(tmp, false)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if status != skillInstalled {
		t.Fatalf("status = %d, want skillInstalled", status)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read installed SKILL.md: %v", err)
	}
	if string(got) != string(btrackSkillMD) {
		t.Fatalf("installed SKILL.md does not match embedded content")
	}
}

func TestInstallSkillIdempotent(t *testing.T) {
	tmp := t.TempDir()
	if _, _, err := installSkill(tmp, false); err != nil {
		t.Fatalf("first install: %v", err)
	}
	status, _, err := installSkill(tmp, false)
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	if status != skillUpToDate {
		t.Fatalf("second install status = %d, want skillUpToDate", status)
	}
}

func TestInstallSkillBlocksOnDifferentExistingFile(t *testing.T) {
	tmp := t.TempDir()
	dest := tmp + "/btrack-tracker"
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest+"/SKILL.md", []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}
	status, _, err := installSkill(tmp, false)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if status != skillBlockedByExisting {
		t.Fatalf("status = %d, want skillBlockedByExisting", status)
	}
	got, err := os.ReadFile(dest + "/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old content" {
		t.Fatalf("file was overwritten without --force")
	}
}

func TestInstallSkillForceOverwrites(t *testing.T) {
	tmp := t.TempDir()
	dest := tmp + "/btrack-tracker"
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest+"/SKILL.md", []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}
	status, _, err := installSkill(tmp, true)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if status != skillInstalled {
		t.Fatalf("status = %d, want skillInstalled", status)
	}
	got, err := os.ReadFile(dest + "/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(btrackSkillMD) {
		t.Fatalf("--force did not overwrite with embedded content")
	}
}
