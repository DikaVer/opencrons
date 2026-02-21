// settings_menu.go implements the settings management TUI.
//
// It defines the SettingsAction enum and provides RunSettingsMenu, which displays
// current configuration values and allows the user to change them. Sub-menus include
// RunProviderSettings (read-only, Anthropic only), RunMessengerSettings with a
// reconfigure flow for Telegram, RunChatModelSettings for default model and effort
// selection, and RunDaemonModeSettings for choosing background or service mode.
package tui

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/DikaVer/opencron/internal/platform"
	"github.com/DikaVer/opencron/internal/ui"
)

// SettingsAction represents what the user chose from the settings menu.
type SettingsAction int

const (
	SettingsProvider SettingsAction = iota
	SettingsMessenger
	SettingsChatModel
	SettingsDaemonMode
	SettingsDebug
	SettingsRerunSetup
	SettingsBack
)

// RunSettingsMenu shows the settings management menu.
func RunSettingsMenu() (SettingsAction, error) {
	s := platform.LoadSettings()

	PrintHeader("⚙️  Settings")

	// Show current settings summary
	providerStr := "not configured"
	if s.Provider != nil {
		providerStr = s.Provider.ID
	}
	fmt.Printf("  %s %s\n", ui.Dim.Render("🔌 Provider:"), providerStr)

	messengerStr := "not configured"
	if s.Messenger != nil && s.Messenger.Type != "" {
		messengerStr = s.Messenger.Type
		userCount := 0
		for _, v := range s.Messenger.AllowedUsers {
			if v {
				userCount++
			}
		}
		messengerStr += fmt.Sprintf(" (%d users)", userCount)
	}
	fmt.Printf("  %s %s\n", ui.Dim.Render("💬 Messenger:"), messengerStr)

	chatStr := "sonnet / high"
	if s.Chat != nil {
		chatStr = s.Chat.Model + " / " + s.Chat.Effort
	}
	fmt.Printf("  %s %s\n", ui.Dim.Render("🧠 Chat:"), chatStr)

	daemonStr := "background"
	if s.DaemonMode != "" {
		daemonStr = s.DaemonMode
	}
	fmt.Printf("  %s %s\n", ui.Dim.Render("🤖 Daemon:"), daemonStr)

	if s.Debug {
		fmt.Printf("  %s %s\n", ui.Dim.Render("🐛 Debug:"), ui.Success.Render("✅ on"))
	} else {
		fmt.Printf("  %s %s\n", ui.Dim.Render("🐛 Debug:"), ui.Fail.Render("❌ off"))
	}
	fmt.Println()

	var choice int
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title("What would you like to change?").
				Options(
					huh.NewOption("🔌 Provider", int(SettingsProvider)),
					huh.NewOption("💬 Messenger", int(SettingsMessenger)),
					huh.NewOption("🧠 Chat Model", int(SettingsChatModel)),
					huh.NewOption("🤖 Daemon mode", int(SettingsDaemonMode)),
					huh.NewOption("🐛 Debug logging", int(SettingsDebug)),
					huh.NewOption("🔁 Re-run setup", int(SettingsRerunSetup)),
					huh.NewOption("◀️  Back", int(SettingsBack)),
				).
				Value(&choice),
		),
	).WithTheme(theme)

	if err := form.Run(); err != nil {
		if IsAborted(err) {
			return SettingsBack, nil
		}
		return SettingsBack, err
	}

	return SettingsAction(choice), nil
}

// RunProviderSettings shows the provider settings view.
func RunProviderSettings() error {
	PrintHeader("🔌 Provider")
	s := platform.LoadSettings()
	if s.Provider != nil {
		fmt.Printf("  %s %s\n", ui.Dim.Render("🔌 Current provider:"), s.Provider.ID)
	} else {
		fmt.Printf("  %s not configured\n", ui.Dim.Render("🔌 Current provider:"))
	}
	fmt.Println(ui.Dim.Render("  Only Anthropic (Claude Code) is currently supported."))
	fmt.Println()
	return nil
}

// RunMessengerSettings shows messenger settings and allows editing.
func RunMessengerSettings() (*platform.MessengerSettings, error) {
	PrintHeader("💬 Messenger")
	s := platform.LoadSettings()

	if s.Messenger != nil && s.Messenger.Type != "" {
		fmt.Printf("  %s %s\n", ui.Dim.Render("💬 Type:"), s.Messenger.Type)
		fmt.Printf("  %s %s...%s\n", ui.Dim.Render("🔑 Token:"), s.Messenger.BotToken[:8], s.Messenger.BotToken[len(s.Messenger.BotToken)-4:])

		userCount := 0
		for _, v := range s.Messenger.AllowedUsers {
			if v {
				userCount++
			}
		}
		fmt.Printf("  %s %d\n", ui.Dim.Render("👥 Allowed users:"), userCount)
		fmt.Println()

		reconfigure, err := ConfirmAction("🔄 Reconfigure messenger?", "This will replace the current configuration.")
		if err != nil {
			return nil, err
		}
		if !reconfigure {
			return nil, nil
		}
	}

	// Run telegram setup
	return runTelegramSetup()
}

// RunChatModelSettings lets the user change chat model and effort.
func RunChatModelSettings() (*platform.ChatSettings, error) {
	PrintHeader("🧠 Chat Model")
	return runChatModelForm()
}

// RunDaemonModeSettings lets the user change daemon mode.
func RunDaemonModeSettings() (string, error) {
	PrintHeader("🤖 Daemon Mode")
	s := platform.LoadSettings()
	current := s.DaemonMode
	if current == "" {
		current = "background"
	}

	var mode string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("🤖 Daemon Mode").
				Description(fmt.Sprintf("Current: %s", current)).
				Options(
					huh.NewOption("💻 Background process", "background"),
					huh.NewOption("🖥️  System service", "service"),
					huh.NewOption("◀️  Back", "__back__"),
				).
				Value(&mode),
		),
	).WithTheme(theme)

	if err := form.Run(); err != nil {
		if IsAborted(err) {
			return "", nil
		}
		return "", err
	}
	if mode == "__back__" {
		return "", nil
	}

	return mode, nil
}
