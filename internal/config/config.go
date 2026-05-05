package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/viper"
)

type GitHubConfig struct {
	PAT      string `mapstructure:"pat"`
	Username string `mapstructure:"username"`
}

type Config struct {
	Database DatabaseConfig            `mapstructure:"database"`
	AI       AIConfig                  `mapstructure:"ai"`
	Daemon   DaemonConfig              `mapstructure:"daemon"`
	Work     WorkConfig                `mapstructure:"work"`
	GitHub   GitHubConfig              `mapstructure:"github"`
	Projects map[string]ProjectConfig  `mapstructure:"projects"`
}

type WorkConfig struct {
	DailyHours  int `mapstructure:"daily_hours"`
	IdleMinutes int `mapstructure:"idle_minutes"` // 0 = disabled
}

type ProjectConfig struct {
	Rate float64 `mapstructure:"rate"` // hourly billing rate
}

type DatabaseConfig struct {
	Type       string `mapstructure:"type"`
	DSN        string `mapstructure:"dsn"`
	SQLitePath string `mapstructure:"sqlite_path"`
}

type AIConfig struct {
	Provider  string `mapstructure:"provider"`   // active provider: "openai" | "claude" | "gemini"
	OpenAIKey string `mapstructure:"openai_key"` // per-provider keys
	ClaudeKey string `mapstructure:"claude_key"`
	GeminiKey string `mapstructure:"gemini_key"`
	Model     string `mapstructure:"model"` // optional model override
}

// ActiveKey returns the API key for the currently selected provider.
func (a AIConfig) ActiveKey() string {
	switch a.Provider {
	case "claude":
		return a.ClaudeKey
	case "gemini":
		return a.GeminiKey
	default:
		return a.OpenAIKey
	}
}

type DaemonConfig struct {
	SocketPath string `mapstructure:"socket_path"`
	PidFile    string `mapstructure:"pid_file"`
}

var (
	instance *Config
	cfgDir   string
)

func ConfigDir() string {
	if cfgDir != "" {
		return cfgDir
	}
	home, _ := os.UserHomeDir()
	cfgDir = filepath.Join(home, ".config", "btrack")
	return cfgDir
}

func DataDir() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(home, "AppData", "Local", "btrack")
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "btrack")
	default:
		return filepath.Join(home, ".local", "share", "btrack")
	}
}

func SocketPath() string {
	if instance != nil && instance.Daemon.SocketPath != "" {
		return instance.Daemon.SocketPath
	}
	return filepath.Join(DataDir(), "btrack.sock")
}

func PidFile() string {
	if instance != nil && instance.Daemon.PidFile != "" {
		return instance.Daemon.PidFile
	}
	return filepath.Join(DataDir(), "daemon.pid")
}

func SQLitePath() string {
	if instance != nil && instance.Database.SQLitePath != "" {
		return instance.Database.SQLitePath
	}
	return filepath.Join(DataDir(), "btrack.db")
}

func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}

func Load() (*Config, error) {
	if instance != nil {
		return instance, nil
	}

	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}
	if err := os.MkdirAll(DataDir(), 0750); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(dir)

	viper.SetDefault("database.type", "sqlite")
	viper.SetDefault("ai.provider", "")
	viper.SetDefault("work.daily_hours", 8)

	viper.SetEnvPrefix("BTRACK")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
		writeDefaultConfig(ConfigPath())
	}

	cfg := &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	instance = cfg
	return cfg, nil
}

// Reload forces a fresh read of the config file.
func Reload() (*Config, error) {
	instance = nil
	return Load()
}

// SaveProviderKey persists an API key for a provider and sets it as active.
func SaveProviderKey(provider, key string) error {
	if _, err := Load(); err != nil {
		return err
	}

	keyField := map[string]string{
		"openai": "ai.openai_key",
		"claude": "ai.claude_key",
		"gemini": "ai.gemini_key",
	}
	field, ok := keyField[provider]
	if !ok {
		return fmt.Errorf("unknown provider: %s", provider)
	}

	viper.Set(field, key)
	viper.Set("ai.provider", provider)

	if err := viper.WriteConfigAs(ConfigPath()); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	// Invalidate cache so next Load() picks up new values.
	instance = nil
	return nil
}

// SaveGitHub persists the GitHub PAT and username.
func SaveGitHub(pat, username string) error {
	if _, err := Load(); err != nil {
		return err
	}
	viper.Set("github.pat", pat)
	viper.Set("github.username", username)
	if err := viper.WriteConfigAs(ConfigPath()); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	instance = nil
	return nil
}

// SaveDailyHours persists the daily work-hours target.
func SaveDailyHours(hours int) error {
	if _, err := Load(); err != nil {
		return err
	}
	viper.Set("work.daily_hours", hours)
	if err := viper.WriteConfigAs(ConfigPath()); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	instance = nil
	return nil
}

// SaveIdleMinutes persists the idle auto-stop threshold (0 = disabled).
func SaveIdleMinutes(minutes int) error {
	if _, err := Load(); err != nil {
		return err
	}
	viper.Set("work.idle_minutes", minutes)
	if err := viper.WriteConfigAs(ConfigPath()); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	instance = nil
	return nil
}

// SaveProjectRate persists an hourly billing rate for a project.
func SaveProjectRate(project string, rate float64) error {
	if _, err := Load(); err != nil {
		return err
	}
	viper.Set("projects."+project+".rate", rate)
	if err := viper.WriteConfigAs(ConfigPath()); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	instance = nil
	return nil
}

func writeDefaultConfig(path string) {
	content := `# btrack configuration
# https://github.com/tolgazorlu/btrack

database:
  type: sqlite
  # dsn: "postgres://user:pass@localhost/btrack?sslmode=disable"
  # sqlite_path: ""

ai:
  provider: ""          # active provider: openai | claude | gemini
  openai_key: ""        # OpenAI API key
  claude_key: ""        # Anthropic API key
  gemini_key: ""        # Google Gemini API key
  # model: ""           # optional override (e.g. gpt-4o, claude-sonnet-4-6)

work:
  daily_hours: 8        # target working hours per day

github:
  pat: ""               # personal access token (read:user, repo)
  username: ""          # your GitHub username (set automatically by: btrack github connect)

daemon:
  # socket_path: ""
  # pid_file: ""
`
	_ = os.WriteFile(filepath.Clean(path), []byte(content), 0600)
}
