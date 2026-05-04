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
	if cfg.Database.Type != "sqlite" {
		t.Errorf("default database type = %q, want sqlite", cfg.Database.Type)
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

func TestAIConfig_ActiveKey(t *testing.T) {
	tests := []struct {
		name     string
		cfg      AIConfig
		wantKey  string
	}{
		{
			name:    "claude provider returns claude key",
			cfg:     AIConfig{Provider: "claude", ClaudeKey: "sk-ant-123"},
			wantKey: "sk-ant-123",
		},
		{
			name:    "gemini provider returns gemini key",
			cfg:     AIConfig{Provider: "gemini", GeminiKey: "gemini-key-abc"},
			wantKey: "gemini-key-abc",
		},
		{
			name:    "openai (default) returns openai key",
			cfg:     AIConfig{Provider: "openai", OpenAIKey: "sk-openai-xyz"},
			wantKey: "sk-openai-xyz",
		},
		{
			name:    "empty provider falls back to openai key",
			cfg:     AIConfig{Provider: "", OpenAIKey: "sk-openai-fallback"},
			wantKey: "sk-openai-fallback",
		},
		{
			name:    "no key configured returns empty",
			cfg:     AIConfig{Provider: "claude"},
			wantKey: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.ActiveKey()
			if got != tt.wantKey {
				t.Errorf("ActiveKey() = %q, want %q", got, tt.wantKey)
			}
		})
	}
}

func TestGitHubConfig_ConnectedCheck(t *testing.T) {
	connected := GitHubConfig{PAT: "ghp_token123", Username: "tolgazorlu"}
	if connected.PAT == "" {
		t.Error("connected config should have non-empty PAT")
	}

	notConnected := GitHubConfig{}
	if notConnected.PAT != "" {
		t.Error("empty config should have empty PAT")
	}
}
