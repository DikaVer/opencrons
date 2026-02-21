// setup_wizard.go implements the first-time setup wizard.
//
// SetupResult holds the wizard output. RunSetupWizard guides the user through a
// 4-step flow: provider detection, messenger configuration with Telegram pairing
// (via PairingBot for code-based verification), chat model defaults (model and
// effort), and daemon mode selection. runTelegramSetup handles bot token input and
// the pairing handshake. runChatModelForm collects model and effort preferences.
package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/dika-maulidal/opencron/internal/messenger/telegram"
	"github.com/dika-maulidal/opencron/internal/platform"
	"github.com/dika-maulidal/opencron/internal/provider"
	"github.com/dika-maulidal/opencron/internal/ui"
)

// SetupResult holds the output of the setup wizard.
type SetupResult struct {
	Provider   string
	Messenger  *platform.MessengerSettings
	Chat       *platform.ChatSettings
	DaemonMode string
}

// RunSetupWizard runs the first-time setup wizard.
func RunSetupWizard() (*SetupResult, error) {
	// Step 1: Welcome
	fmt.Println()
	fmt.Println(ui.Title.Render("  Welcome to OpenCron"))
	fmt.Println()
	fmt.Println(ui.Dim.Render("  OpenCron runs Claude Code tasks on cron schedules."))
	fmt.Println(ui.Dim.Render("  This wizard will help you set up your environment."))
	fmt.Println()

	// Step 2: Provider detection
	fmt.Println(ui.Title.Render("  Step 1: AI Provider"))
	fmt.Println()

	var providerID string
	providerForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select AI Provider").
				Description("Which AI provider to use for running tasks.").
				Options(
					huh.NewOption("Anthropic (Claude Code)", "anthropic"),
				).
				Value(&providerID),
		),
	).WithTheme(theme)

	if err := providerForm.Run(); err != nil {
		return nil, err
	}

	// Check provider availability
	p := provider.Get(providerID)
	if p == nil {
		return nil, fmt.Errorf("unknown provider: %s", providerID)
	}

	for !p.Detect() {
		fmt.Println()
		fmt.Println(ui.Dim.Render("  Claude Code CLI not found on PATH."))
		fmt.Println(ui.Dim.Render("  Install it: npm install -g @anthropic-ai/claude-code"))
		fmt.Println()
		PrintPressEnter()
	}

	if version := p.Version(); version != "" {
		fmt.Printf("  %s %s\n", ui.Dim.Render("Claude Code:"), ui.Success.Render(version))
	}
	fmt.Println()

	// Step 3: Messenger
	fmt.Println(ui.Title.Render("  Step 2: Messenger Integration"))
	fmt.Println()

	var messengerType string
	messengerForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Messenger Platform").
				Description("Connect a messenger to chat with Claude and manage jobs remotely.").
				Options(
					huh.NewOption("Telegram", "telegram"),
					huh.NewOption("Skip (TUI only)", ""),
				).
				Value(&messengerType),
		),
	).WithTheme(theme)

	if err := messengerForm.Run(); err != nil {
		return nil, err
	}

	result := &SetupResult{
		Provider: providerID,
	}

	// Step 4: Telegram setup
	if messengerType == "telegram" {
		msgSettings, err := runTelegramSetup()
		if err != nil {
			return nil, err
		}
		if msgSettings != nil {
			result.Messenger = msgSettings

			// Step 5: Chat Model
			fmt.Println()
			fmt.Println(ui.Title.Render("  Step 3: Chat Model"))
			fmt.Println()

			chatSettings, err := runChatModelForm()
			if err != nil {
				return nil, err
			}
			if chatSettings != nil {
				result.Chat = chatSettings
			}
		}
	}

	// Step 6: Daemon mode
	fmt.Println()
	fmt.Println(ui.Title.Render("  Step 4: Daemon Configuration"))
	fmt.Println()

	var daemonMode string
	daemonForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Daemon Mode").
				Description("How should the OpenCron daemon run?").
				Options(
					huh.NewOption("Background process", "background"),
					huh.NewOption("System service (auto-start on boot)", "service"),
				).
				Value(&daemonMode),
		),
	).WithTheme(theme)

	if err := daemonForm.Run(); err != nil {
		return nil, err
	}
	result.DaemonMode = daemonMode

	// Done
	fmt.Println()
	fmt.Println(ui.Success.Render("  Setup complete!"))
	fmt.Println()

	return result, nil
}

// runTelegramSetup handles the Telegram bot configuration flow with code-based pairing.
func runTelegramSetup() (*platform.MessengerSettings, error) {
	fmt.Println()
	fmt.Println(ui.Dim.Render("  Create a bot with @BotFather on Telegram to get your token."))
	fmt.Println(ui.Dim.Render("  1. Open Telegram and search for @BotFather"))
	fmt.Println(ui.Dim.Render("  2. Send /newbot and follow the instructions"))
	fmt.Println(ui.Dim.Render("  3. Copy the HTTP API token"))
	fmt.Println()

	var botToken string
	tokenForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Bot Token").
				Description("Paste the HTTP API token from @BotFather.").
				Placeholder("123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11").
				Value(&botToken).
				Validate(func(s string) error {
					s = strings.TrimSpace(s)
					if s == "" {
						return fmt.Errorf("bot token is required")
					}
					if !strings.Contains(s, ":") {
						return fmt.Errorf("invalid token format (should contain ':')")
					}
					return nil
				}),
		),
	).WithTheme(theme)

	if err := tokenForm.Run(); err != nil {
		if IsAborted(err) {
			return nil, nil
		}
		return nil, err
	}
	botToken = strings.TrimSpace(botToken)

	// Start pairing bot to validate token and begin code-based pairing
	fmt.Println()
	fmt.Println(ui.Dim.Render("  Verifying bot token..."))

	pb, err := telegram.StartPairingBot(botToken)
	if err != nil {
		return nil, fmt.Errorf("bot token error: %w", err)
	}
	defer pb.Stop()

	if pb.BotName() != "" {
		fmt.Println(ui.Success.Render(fmt.Sprintf("  Bot @%s is running!", pb.BotName())))
	} else {
		fmt.Println(ui.Success.Render("  Bot is running!"))
	}

	settings := &platform.MessengerSettings{
		Type:         "telegram",
		BotToken:     botToken,
		AllowedUsers: make(map[string]bool),
	}

	// Code-based pairing loop
	for {
		fmt.Println()
		fmt.Println(ui.Dim.Render("  Send any message to your bot in Telegram."))
		fmt.Println(ui.Dim.Render("  You'll receive a 6-digit pairing code — enter it below."))
		fmt.Println()

		var code string
		codeForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Pairing Code").
					Description("Enter the 6-digit code from Telegram.").
					Placeholder("000000").
					Value(&code).
					Validate(func(s string) error {
						s = strings.TrimSpace(s)
						if s == "" {
							return fmt.Errorf("pairing code is required")
						}
						if len(s) != 6 {
							return fmt.Errorf("code must be 6 digits")
						}
						return nil
					}),
			),
		).WithTheme(theme)

		if err := codeForm.Run(); err != nil {
			if IsAborted(err) {
				return nil, nil
			}
			return nil, err
		}
		code = strings.TrimSpace(code)

		result, err := pb.ValidateCode(code)
		if err != nil {
			fmt.Println(ui.Fail.Render("  Invalid code. Make sure you sent a message to the bot and try again."))
			continue
		}

		settings.AllowedUsers[strconv.FormatInt(result.UserID, 10)] = true

		displayName := fmt.Sprintf("%d", result.UserID)
		if result.Username != "" {
			displayName = fmt.Sprintf("@%s (%d)", result.Username, result.UserID)
		} else if result.Name != "" {
			displayName = fmt.Sprintf("%s (%d)", result.Name, result.UserID)
		}

		fmt.Println(ui.Accent.Render(fmt.Sprintf("  Paired with %s", displayName)))

		addMore, err := ConfirmAction("Add another user?", "")
		if err != nil {
			return nil, err
		}
		if !addMore {
			break
		}
	}

	fmt.Println()
	return settings, nil
}

// runChatModelForm gets chat model settings.
// Returns nil, nil if user chose back.
func runChatModelForm() (*platform.ChatSettings, error) {
	var model string
	modelForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Default Chat Model").
				Description("Which model to use for Telegram chat sessions.").
				Options(
					huh.NewOption("Sonnet (recommended)", "sonnet"),
					huh.NewOption("Opus", "opus"),
					huh.NewOption("Haiku", "haiku"),
					huh.NewOption("<< Back", "__back__"),
				).
				Value(&model),
		),
	).WithTheme(theme)

	if err := modelForm.Run(); err != nil {
		if IsAborted(err) {
			return nil, nil
		}
		return nil, err
	}
	if model == "__back__" {
		return nil, nil
	}

	var effort string
	effortForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Default Effort Level").
				Description("Controls how much thinking effort Claude uses.").
				Options(
					huh.NewOption("High (recommended)", "high"),
					huh.NewOption("Medium", "medium"),
					huh.NewOption("Low", "low"),
					huh.NewOption("Max (Opus only)", "max"),
					huh.NewOption("<< Back", "__back__"),
				).
				Value(&effort),
		),
	).WithTheme(theme)

	if err := effortForm.Run(); err != nil {
		if IsAborted(err) {
			return nil, nil
		}
		return nil, err
	}
	if effort == "__back__" {
		return nil, nil
	}

	return &platform.ChatSettings{
		Model:  model,
		Effort: effort,
	}, nil
}
