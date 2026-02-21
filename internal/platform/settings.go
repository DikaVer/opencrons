package platform

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	Type         string          `json:"type"`                    // "telegram" | ""
	BotToken     string          `json:"bot_token"`
	Pairing      string          `json:"pairing_mode"`            // "gatherToken" | "allowList"
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
	return os.WriteFile(settingsFile(), data, 0644)
}

// IsDebugEnabled returns whether debug logging is on.
func IsDebugEnabled() bool {
	return LoadSettings().Debug
}

// SetDebug enables or disables debug logging.
func SetDebug(enabled bool) error {
	s := LoadSettings()
	s.Debug = enabled
	return SaveSettings(s)
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
