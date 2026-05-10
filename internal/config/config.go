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
	Database DatabaseConfig           `mapstructure:"database"`
	AI       AIConfig                 `mapstructure:"ai"`
	Daemon   DaemonConfig             `mapstructure:"daemon"`
	Work     WorkConfig               `mapstructure:"work"`
	Pomo     PomoConfig               `mapstructure:"pomo"`
	GitHub   GitHubConfig             `mapstructure:"github"`
	Projects map[string]ProjectConfig `mapstructure:"projects"`
	GCal     GCalConfig               `mapstructure:"gcal"`
}

type PomoConfig struct {
	Sound  bool `mapstructure:"sound"`
	Notify bool `mapstructure:"notify"`
}

type GCalConfig struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	CalendarID   string `mapstructure:"calendar_id"`
	AutoSync     bool   `mapstructure:"auto_sync"`
}

type WorkConfig struct {
	DailyHours      int `mapstructure:"daily_hours"`
	IdleMinutes     int `mapstructure:"idle_minutes"`
	MaxHours        int `mapstructure:"max_hours"`
	ReminderMinutes int `mapstructure:"reminder_minutes"`
}

type ProjectConfig struct {
	Rate float64 `mapstructure:"rate"`
}

type DatabaseConfig struct {
	Type       string `mapstructure:"type"`
	DSN        string `mapstructure:"dsn"`
	SQLitePath string `mapstructure:"sqlite_path"`
}

type AIConfig struct {
	Provider  string `mapstructure:"provider"`
	OpenAIKey string `mapstructure:"openai_key"`
	ClaudeKey string `mapstructure:"claude_key"`
	GeminiKey string `mapstructure:"gemini_key"`
	Model     string `mapstructure:"model"`
}

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
	viper.SetDefault("work.max_hours", 12)
	viper.SetDefault("pomo.sound", true)
	viper.SetDefault("pomo.notify", true)

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

func Reload() (*Config, error) {
	instance = nil
	return Load()
}

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

	instance = nil
	return nil
}

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

func SaveMaxHours(hours int) error {
	if _, err := Load(); err != nil {
		return err
	}
	viper.Set("work.max_hours", hours)
	if err := viper.WriteConfigAs(ConfigPath()); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	instance = nil
	return nil
}

func SaveReminderMinutes(minutes int) error {
	if _, err := Load(); err != nil {
		return err
	}
	viper.Set("work.reminder_minutes", minutes)
	if err := viper.WriteConfigAs(ConfigPath()); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	instance = nil
	return nil
}

func SavePomoSound(enabled bool) error {
	if _, err := Load(); err != nil {
		return err
	}
	viper.Set("pomo.sound", enabled)
	if err := viper.WriteConfigAs(ConfigPath()); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	instance = nil
	return nil
}

func SavePomoNotify(enabled bool) error {
	if _, err := Load(); err != nil {
		return err
	}
	viper.Set("pomo.notify", enabled)
	if err := viper.WriteConfigAs(ConfigPath()); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	instance = nil
	return nil
}

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

func SaveGCal(clientID, clientSecret, calendarID string, autoSync bool) error {
	if _, err := Load(); err != nil {
		return err
	}
	viper.Set("gcal.client_id", clientID)
	viper.Set("gcal.client_secret", clientSecret)
	if calendarID != "" {
		viper.Set("gcal.calendar_id", calendarID)
	}
	viper.Set("gcal.auto_sync", autoSync)
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
  # idle_minutes: 0     # auto-stop after N minutes with no btrack activity (0 = off)
  # max_hours: 12       # hard cap on a single session's duration (0 = off)
  # reminder_minutes: 0 # OS notification every N min while a session is running (0 = off)

pomo:
  sound: true           # play a sound on phase transitions
  notify: true          # send a system notification on phase transitions

github:
  pat: ""               # personal access token (read:user, repo)
  username: ""          # your GitHub username (set automatically by: btrack github connect)

daemon:
  # socket_path: ""
  # pid_file: ""
`
	_ = os.WriteFile(filepath.Clean(path), []byte(content), 0600)
}
