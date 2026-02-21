package tui

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/dika-maulidal/cli-scheduler/internal/platform"
)

// SettingsAction represents what the user chose from the settings menu.
type SettingsAction int

const (
	SettingsProvider SettingsAction = iota
	SettingsMessenger
	SettingsChatDefaults
	SettingsDaemonMode
	SettingsDebug
	SettingsRerunSetup
	SettingsBack
)

// RunSettingsMenu shows the settings management menu.
func RunSettingsMenu() (SettingsAction, error) {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#cba6f7"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1"))
	failStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8"))

	s := platform.LoadSettings()

	fmt.Println()
	fmt.Println(titleStyle.Render("  Settings"))
	fmt.Println()

	// Show current settings summary
	providerStr := "not configured"
	if s.Provider != nil {
		providerStr = s.Provider.ID
	}
	fmt.Printf("  %s %s\n", dimStyle.Render("Provider:"), providerStr)

	messengerStr := "not configured"
	if s.Messenger != nil && s.Messenger.Type != "" {
		messengerStr = s.Messenger.Type
		userCount := 0
		for k, v := range s.Messenger.AllowedUsers {
			if v && k != "__pending_pairing__" {
				userCount++
			}
		}
		messengerStr += fmt.Sprintf(" (%d users)", userCount)
	}
	fmt.Printf("  %s %s\n", dimStyle.Render("Messenger:"), messengerStr)

	chatStr := "sonnet / high"
	if s.Chat != nil {
		chatStr = s.Chat.Model + " / " + s.Chat.Effort
	}
	fmt.Printf("  %s %s\n", dimStyle.Render("Chat:"), chatStr)

	daemonStr := "background"
	if s.DaemonMode != "" {
		daemonStr = s.DaemonMode
	}
	fmt.Printf("  %s %s\n", dimStyle.Render("Daemon:"), daemonStr)

	if s.Debug {
		fmt.Printf("  %s %s\n", dimStyle.Render("Debug:"), successStyle.Render("on"))
	} else {
		fmt.Printf("  %s %s\n", dimStyle.Render("Debug:"), failStyle.Render("off"))
	}
	fmt.Println()

	var choice int
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title("What would you like to change?").
				Options(
					huh.NewOption("Provider               View/change AI provider", int(SettingsProvider)),
					huh.NewOption("Messenger              View/change Telegram settings", int(SettingsMessenger)),
					huh.NewOption("Chat defaults           Change model and effort", int(SettingsChatDefaults)),
					huh.NewOption("Daemon mode            Background vs. system service", int(SettingsDaemonMode)),
					huh.NewOption("Debug logging          Toggle on/off", int(SettingsDebug)),
					huh.NewOption("Re-run setup           Start setup wizard again", int(SettingsRerunSetup)),
					huh.NewOption("<< Back", int(SettingsBack)),
				).
				Value(&choice),
		),
	).WithTheme(theme)

	if err := form.Run(); err != nil {
		return SettingsBack, err
	}

	return SettingsAction(choice), nil
}

// RunProviderSettings shows the provider settings view.
func RunProviderSettings() error {
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))

	s := platform.LoadSettings()
	if s.Provider != nil {
		fmt.Printf("\n  %s %s\n", dimStyle.Render("Current provider:"), s.Provider.ID)
	} else {
		fmt.Printf("\n  %s not configured\n", dimStyle.Render("Current provider:"))
	}
	fmt.Println(dimStyle.Render("  Only Anthropic (Claude Code) is currently supported."))
	fmt.Println()
	return nil
}

// RunMessengerSettings shows messenger settings and allows editing.
func RunMessengerSettings() (*platform.MessengerSettings, error) {
	s := platform.LoadSettings()
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))

	if s.Messenger != nil && s.Messenger.Type != "" {
		fmt.Println()
		fmt.Printf("  %s %s\n", dimStyle.Render("Type:"), s.Messenger.Type)
		fmt.Printf("  %s %s...%s\n", dimStyle.Render("Token:"), s.Messenger.BotToken[:8], s.Messenger.BotToken[len(s.Messenger.BotToken)-4:])
		fmt.Printf("  %s %s\n", dimStyle.Render("Pairing:"), s.Messenger.Pairing)

		userCount := 0
		for k, v := range s.Messenger.AllowedUsers {
			if v && k != "__pending_pairing__" {
				userCount++
			}
		}
		fmt.Printf("  %s %d\n", dimStyle.Render("Allowed users:"), userCount)
		fmt.Println()

		reconfigure, err := ConfirmAction("Reconfigure messenger?", "This will replace the current configuration.")
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

// RunChatDefaultsSettings lets the user change chat model and effort.
func RunChatDefaultsSettings() (*platform.ChatSettings, error) {
	return runChatDefaultsForm()
}

// RunDaemonModeSettings lets the user change daemon mode.
func RunDaemonModeSettings() (string, error) {
	s := platform.LoadSettings()
	current := s.DaemonMode
	if current == "" {
		current = "background"
	}

	var mode string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Daemon Mode").
				Description(fmt.Sprintf("Current: %s", current)).
				Options(
					huh.NewOption("Background process      Start manually with 'scheduler start'", "background"),
					huh.NewOption("System service          Auto-start on boot (requires admin)", "service"),
				).
				Value(&mode),
		),
	).WithTheme(theme)

	if err := form.Run(); err != nil {
		return "", err
	}

	return mode, nil
}
