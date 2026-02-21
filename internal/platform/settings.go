// Package platform provides JSON-based settings persistence via settings.json.
//
// The Settings struct holds all application configuration: Debug and SetupComplete
// flags, Provider (AI provider ID), Messenger (Telegram bot token and allowed users),
// Chat (default model and effort level), and DaemonMode. LoadSettings and SaveSettings
// handle file I/O with sensible defaults for missing files. Convenience functions
// IsDebugEnabled, SetDebug, IsSetupComplete, GetMessengerConfig, and GetChatConfig
// provide targeted access with default value fallbacks.
package platform

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/DikaVer/opencrons/internal/logger"
)

var (
	debugFlag   atomic.Bool
	debugLoaded atomic.Bool
	plog        = logger.New("platform")
)

// Settings holds persisted application settings.
type Settings struct {
	Debug         bool               `json:"debug"`
	SetupComplete bool               `json:"setup_complete"`
	Provider      *ProviderSettings  `json:"provider,omitempty"`
	Messenger     *MessengerSettings `json:"messenger,omitempty"`
	Chat          *ChatSettings      `json:"chat,omitempty"`
	DaemonMode    string             `json:"daemon_mode,omitempty"`
}

// ProviderSettings holds AI provider configuration.
type ProviderSettings struct {
	ID string `json:"id"` // "anthropic"
}

// MessengerSettings holds messenger platform configuration.
type MessengerSettings struct {
	Type         string          `json:"type"` // "telegram" | ""
	BotToken     string          `json:"bot_token"`
	AllowedUsers map[string]bool `json:"allowed_users,omitempty"`
}

// ChatSettings holds default chat configuration.
type ChatSettings struct {
	Model  string `json:"model"`  // "sonnet" | "opus" | "haiku"
	Effort string `json:"effort"` // "low" | "medium" | "high" | "max"
}

// settingsFile returns the path to settings.json.
func settingsFile() string {
	return filepath.Join(BaseDir(), "settings.json")
}

// LoadSettings reads settings from disk. Returns defaults if file doesn't exist.
func LoadSettings() Settings {
	data, err := os.ReadFile(settingsFile())
	if err != nil {
		return Settings{}
	}

	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return Settings{}
	}
	return s
}

// SaveSettings writes settings to disk.
func SaveSettings(s Settings) error {
	if err := os.MkdirAll(BaseDir(), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(settingsFile(), data, 0644); err != nil {
		return err
	}

	// Keep in-memory debug cache in sync with persisted settings.
	debugFlag.Store(s.Debug)
	debugLoaded.Store(true)
	plog.Info("settings saved")
	return nil
}

// IsDebugEnabled returns whether debug logging is on.
func IsDebugEnabled() bool {
	if debugLoaded.Load() {
		return debugFlag.Load()
	}

	enabled := LoadSettings().Debug
	debugFlag.Store(enabled)
	debugLoaded.Store(true)
	return enabled
}

// SetDebug enables or disables debug logging.
func SetDebug(enabled bool) error {
	s := LoadSettings()
	s.Debug = enabled
	if err := SaveSettings(s); err != nil {
		return err
	}

	debugFlag.Store(enabled)
	debugLoaded.Store(true)
	logger.SetDebug(enabled)
	plog.Info("debug toggled", "enabled", enabled)
	return nil
}

// IsSetupComplete returns whether the first-run setup has been completed.
func IsSetupComplete() bool {
	return LoadSettings().SetupComplete
}

// GetMessengerConfig returns the messenger settings, or nil if not configured.
func GetMessengerConfig() *MessengerSettings {
	s := LoadSettings()
	if s.Messenger == nil || s.Messenger.Type == "" {
		return nil
	}
	return s.Messenger
}

// GetChatConfig returns the chat settings with defaults applied.
func GetChatConfig() *ChatSettings {
	s := LoadSettings()
	if s.Chat == nil {
		return &ChatSettings{Model: "sonnet", Effort: "high"}
	}
	c := *s.Chat
	if c.Model == "" {
		c.Model = "sonnet"
	}
	if c.Effort == "" {
		c.Effort = "high"
	}
	return &c
}
