// File settings.go implements the settings command and the handleSettingsMenu loop.
// It dispatches to handlers for provider, messenger, chat model, daemon mode,
// debug, and re-run setup options, saving changed settings to the platform config.
package cmd

import (
	"fmt"
	"os"

	"github.com/DikaVer/opencron/internal/platform"
	"github.com/DikaVer/opencron/internal/tui"
	"github.com/spf13/cobra"
)

var settingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Manage application settings",
	Long:  "View and modify provider, messenger, chat, daemon, and debug settings.",
	RunE:  runSettings,
}

func init() {
	rootCmd.AddCommand(settingsCmd)
}

func runSettings(cmd *cobra.Command, args []string) error {
	return handleSettingsMenu()
}

func handleSettingsMenu() error {
	for {
		action, err := tui.RunSettingsMenu()
		if err != nil {
			return err
		}

		switch action {
		case tui.SettingsProvider:
			if err := tui.RunProviderSettings(); err != nil {
				fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
			}
			tui.PrintPressEnter()

		case tui.SettingsMessenger:
			msgSettings, err := tui.RunMessengerSettings()
			if err != nil {
				fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
				tui.PrintPressEnter()
				continue
			}
			if msgSettings != nil {
				s := platform.LoadSettings()
				s.Messenger = msgSettings
				if err := platform.SaveSettings(s); err != nil {
					fmt.Fprintf(os.Stderr, "  Error saving: %v\n", err)
				} else {
					fmt.Println("  Messenger settings saved.")
				}
			}
			tui.PrintPressEnter()

		case tui.SettingsChatModel:
			chatSettings, err := tui.RunChatModelSettings()
			if err != nil {
				fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
				tui.PrintPressEnter()
				continue
			}
			if chatSettings == nil {
				continue // back
			}
			s := platform.LoadSettings()
			s.Chat = chatSettings
			if err := platform.SaveSettings(s); err != nil {
				fmt.Fprintf(os.Stderr, "  Error saving: %v\n", err)
			} else {
				fmt.Println("Chat Model saved.")
			}
			tui.PrintPressEnter()

		case tui.SettingsDaemonMode:
			mode, err := tui.RunDaemonModeSettings()
			if err != nil {
				fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
				tui.PrintPressEnter()
				continue
			}
			if mode == "" {
				continue // back
			}
			s := platform.LoadSettings()
			s.DaemonMode = mode
			if err := platform.SaveSettings(s); err != nil {
				fmt.Fprintf(os.Stderr, "  Error saving: %v\n", err)
			} else {
				fmt.Printf("  Daemon mode set to %q.\n", mode)
			}
			tui.PrintPressEnter()

		case tui.SettingsDebug:
			handleDebugMenu()

		case tui.SettingsRerunSetup:
			if err := runSetupWizard(); err != nil {
				fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
			}
			tui.PrintPressEnter()

		case tui.SettingsBack:
			return nil
		}
	}
}
