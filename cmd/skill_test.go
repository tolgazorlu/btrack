package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// TestEmbeddedSkillMatchesCanonical fails if the SKILL.md embedded in the
// binary drifts from the canonical copy at skills/btrack/SKILL.md.
// Run `make sync-skill` to re-sync.
func TestEmbeddedSkillMatchesCanonical(t *testing.T) {
	canonical, err := os.ReadFile("../skills/btrack/SKILL.md")
	if err != nil {
		t.Fatalf("read canonical SKILL.md: %v", err)
	}
	if string(canonical) != string(btrackSkillMD()) {
		t.Fatalf("embedded SKILL.md is out of sync with skills/btrack/SKILL.md.\n" +
			"Run `make sync-skill` and rebuild.")
	}
}

// TestEmbeddedTreeMatchesCanonical asserts every file under skills/btrack/
// is present in the embedded FS with identical content.
func TestEmbeddedTreeMatchesCanonical(t *testing.T) {
	canonicalRoot := "../skills/btrack"
	err := filepath.Walk(canonicalRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(canonicalRoot, path)
		if err != nil {
			return err
		}
		canonicalBytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		embeddedPath := skillEmbedRoot + "/" + filepath.ToSlash(rel)
		embeddedBytes, err := btrackSkillFS.ReadFile(embeddedPath)
		if err != nil {
			t.Errorf("embedded tree missing %s: %v (run `make sync-skill`)", embeddedPath, err)
			return nil
		}
		if string(canonicalBytes) != string(embeddedBytes) {
			t.Errorf("embedded %s differs from canonical (run `make sync-skill`)", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk canonical tree: %v", err)
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
	if string(got) != string(btrackSkillMD()) {
		t.Fatalf("installed SKILL.md does not match embedded content")
	}
}

// TestInstallSkillCopiesWholeTree verifies that scripts and references land
// in the destination, not just SKILL.md.
func TestInstallSkillCopiesWholeTree(t *testing.T) {
	tmp := t.TempDir()
	if _, _, err := installSkill(tmp, false); err != nil {
		t.Fatalf("install: %v", err)
	}
	want := []string{
		"btrack/SKILL.md",
		"btrack/README.md",
		"btrack/metadata.json",
		"btrack/scripts/setup.sh",
		"btrack/references/installation.md",
		"btrack/references/standup-workflow.md",
		"btrack/references/shipped-workflow.md",
		"btrack/references/troubleshooting.md",
	}
	for _, p := range want {
		full := filepath.Join(tmp, p)
		if _, err := os.Stat(full); err != nil {
			t.Errorf("missing %s after install: %v", p, err)
		}
	}
}

// TestInstallSkillScriptsAreExecutable verifies setup.sh has +x after install.
func TestInstallSkillScriptsAreExecutable(t *testing.T) {
	tmp := t.TempDir()
	if _, _, err := installSkill(tmp, false); err != nil {
		t.Fatalf("install: %v", err)
	}
	info, err := os.Stat(filepath.Join(tmp, "btrack/scripts/setup.sh"))
	if err != nil {
		t.Fatalf("stat setup.sh: %v", err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Fatalf("setup.sh is not executable: %v", info.Mode())
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
	dest := filepath.Join(tmp, "btrack")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "SKILL.md"), []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}
	status, _, err := installSkill(tmp, false)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if status != skillBlockedByExisting {
		t.Fatalf("status = %d, want skillBlockedByExisting", status)
	}
	got, err := os.ReadFile(filepath.Join(dest, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old content" {
		t.Fatalf("file was overwritten without --force")
	}
}

func TestInstallSkillForceOverwrites(t *testing.T) {
	tmp := t.TempDir()
	dest := filepath.Join(tmp, "btrack")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "SKILL.md"), []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}
	status, _, err := installSkill(tmp, true)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if status != skillInstalled {
		t.Fatalf("status = %d, want skillInstalled", status)
	}
	got, err := os.ReadFile(filepath.Join(dest, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(btrackSkillMD()) {
		t.Fatalf("--force did not overwrite with embedded content")
	}
}
