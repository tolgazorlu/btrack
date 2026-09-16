package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Use a temp dir so we don't touch the real config.
	tmp := t.TempDir()
	cfgDir = tmp
	instance = nil
	t.Cleanup(func() {
		cfgDir = ""
		instance = nil
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Work.DailyHours != 8 {
		t.Errorf("default DailyHours = %d, want 8", cfg.Work.DailyHours)
	}
}

func TestLoad_CreatesConfigFile(t *testing.T) {
	tmp := t.TempDir()
	cfgDir = tmp
	instance = nil
	t.Cleanup(func() {
		cfgDir = ""
		instance = nil
	})

	if _, err := Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	cfgFile := filepath.Join(tmp, "config.yaml")
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		t.Error("Load() should create config.yaml if it doesn't exist")
	}
}
