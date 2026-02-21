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
	"github.com/DikaVer/opencron/internal/messenger/telegram"
	"github.com/DikaVer/opencron/internal/platform"
	"github.com/DikaVer/opencron/internal/provider"
	"github.com/DikaVer/opencron/internal/ui"
)

// SetupResult holds the output of the setup wizard.
type SetupResult struct {
	Provider   string
	Messenger  *platform.MessengerSettings
	Chat       *platform.ChatSettings
	DaemonMode string
}

// printSetupHeader renders a setup-specific header with step number and title.
// Unlike PrintHeader, it omits the status bar since nothing is configured yet.
func printSetupHeader(step int, title string) {
	ClearScreen()
	fmt.Println()
	fmt.Println(ui.Title.Render("  🎉 OpenCron — Setup"))
	fmt.Println(ui.Dim.Render("  Configure your environment to get started"))
	fmt.Println()
	fmt.Println(ui.Title.Render(fmt.Sprintf("  Step %d · %s", step, title)))
	fmt.Println()
}

// RunSetupWizard runs the first-time setup wizard.
func RunSetupWizard() (*SetupResult, error) {
	step := 1

	// Step 1: AI Provider
	printSetupHeader(step, "AI Provider")

	var providerID string
	providerForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select AI Provider").
				Description("Which AI provider to use for running tasks.").
				Options(
					huh.NewOption("🤖 Anthropic (Claude Code)", "anthropic"),
				).
				Height(5).
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
		printSetupHeader(step, "AI Provider")
		fmt.Println(ui.Warn.Render("  ⚠️  Claude Code CLI not found on PATH."))
		fmt.Println(ui.Warn.Render("  Install it: npm install -g @anthropic-ai/claude-code"))
		fmt.Println()
		PrintPressEnter()
	}

	providerVersion := p.Version()
	if providerVersion != "" {
		fmt.Printf("  %s %s\n", ui.Dim.Render("✅ Claude Code:"), ui.Success.Render(providerVersion))
	}
	fmt.Println()

	// Step 2: Messenger
	step++
	printSetupHeader(step, "Messenger Integration")

	var messengerType string
	messengerForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Messenger Platform").
				Description("Connect a messenger to chat with Claude and manage jobs remotely.").
				Options(
					huh.NewOption("📱 Telegram", "telegram"),
					huh.NewOption("⏭️  Skip (TUI only)", ""),
				).
				Height(6).
				Value(&messengerType),
		),
	).WithTheme(theme)

	if err := messengerForm.Run(); err != nil {
		return nil, err
	}

	result := &SetupResult{
		Provider: providerID,
	}

	// Telegram setup (sub-step of step 2)
	if messengerType == "telegram" {
		printSetupHeader(step, "Telegram — Bot Setup")
		msgSettings, err := runTelegramSetup()
		if err != nil {
			return nil, err
		}
		if msgSettings != nil {
			result.Messenger = msgSettings

			// Chat Model step (only if telegram was configured)
			step++
			printSetupHeader(step, "Chat Model")

			chatSettings, err := runChatModelForm()
			if err != nil {
				return nil, err
			}
			if chatSettings != nil {
				result.Chat = chatSettings
			}
		}
	}

	// Daemon Configuration
	step++
	printSetupHeader(step, "Daemon Configuration")

	var daemonMode string
	daemonForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Daemon Mode").
				Description("How should the OpenCron daemon run?").
				Options(
					huh.NewOption("💻 Background process", "background"),
					huh.NewOption("🖥️  System service (auto-start on boot)", "service"),
				).
				Height(6).
				Value(&daemonMode),
		),
	).WithTheme(theme)

	if err := daemonForm.Run(); err != nil {
		return nil, err
	}
	result.DaemonMode = daemonMode

	// Done — completion summary
	ClearScreen()
	fmt.Println()
	fmt.Println(ui.Success.Render("  ✅ Setup Complete!"))
	fmt.Println()

	// Provider
	fmt.Printf("  %s %s\n", ui.Dim.Render("🔌 Provider: "), ui.Accent.Render(result.Provider))

	// Messenger
	if result.Messenger != nil && result.Messenger.Type != "" {
		userCount := len(result.Messenger.AllowedUsers)
		userLabel := "user"
		if userCount != 1 {
			userLabel = "users"
		}
		fmt.Printf("  %s %s\n", ui.Dim.Render("💬 Messenger:"), ui.Accent.Render(fmt.Sprintf("%s (%d %s)", result.Messenger.Type, userCount, userLabel)))
	} else {
		fmt.Printf("  %s %s\n", ui.Dim.Render("💬 Messenger:"), ui.Dim.Render("skipped"))
	}

	// Chat model
	if result.Chat != nil {
		fmt.Printf("  %s %s\n", ui.Dim.Render("🧠 Chat:     "), ui.Accent.Render(fmt.Sprintf("%s / %s", result.Chat.Model, result.Chat.Effort)))
	} else {
		fmt.Printf("  %s %s\n", ui.Dim.Render("🧠 Chat:     "), ui.Dim.Render("n/a"))
	}

	// Daemon
	fmt.Printf("  %s %s\n", ui.Dim.Render("🤖 Daemon:   "), ui.Accent.Render(result.DaemonMode))

	// Claude Code version
	if providerVersion != "" {
		fmt.Printf("  %s %s\n", ui.Dim.Render("📦 Claude:   "), ui.Accent.Render(providerVersion))
	}

	fmt.Println()
	fmt.Println(ui.Dim.Render("  Run 'opencron start' to launch the daemon."))
	fmt.Println()

	return result, nil
}

// runTelegramSetup handles the Telegram bot configuration flow with code-based pairing.
func runTelegramSetup() (*platform.MessengerSettings, error) {
	fmt.Println()
	fmt.Println(ui.Dim.Render("  Create a bot with @BotFather on Telegram to get your token."))
	fmt.Println(ui.Dim.Render("  1️⃣  Open Telegram and search for @BotFather"))
	fmt.Println(ui.Dim.Render("  2️⃣  Send /newbot and follow the instructions"))
	fmt.Println(ui.Dim.Render("  3️⃣  Copy the HTTP API token"))
	fmt.Println()

	var botToken string
	tokenForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("🔑 Bot Token").
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
	fmt.Println(ui.Dim.Render("  🔄 Verifying bot token..."))

	pb, err := telegram.StartPairingBot(botToken)
	if err != nil {
		return nil, fmt.Errorf("bot token error: %w", err)
	}
	defer pb.Stop()

	if pb.BotName() != "" {
		fmt.Println(ui.Success.Render(fmt.Sprintf("  ✅ Bot @%s is running!", pb.BotName())))
	} else {
		fmt.Println(ui.Success.Render("  ✅ Bot is running!"))
	}

	settings := &platform.MessengerSettings{
		Type:         "telegram",
		BotToken:     botToken,
		AllowedUsers: make(map[string]bool),
	}

	// Code-based pairing loop
	for {
		fmt.Println()
		fmt.Println(ui.Dim.Render("  📱 Send any message to your bot in Telegram."))
		fmt.Println(ui.Dim.Render("  You'll receive a 6-digit pairing code — enter it below."))
		fmt.Println()

		var code string
		codeForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("🔗 Pairing Code").
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
			fmt.Println(ui.Fail.Render("  ❌ Invalid code. Make sure you sent a message to the bot and try again."))
			continue
		}

		settings.AllowedUsers[strconv.FormatInt(result.UserID, 10)] = true

		displayName := fmt.Sprintf("%d", result.UserID)
		if result.Username != "" {
			displayName = fmt.Sprintf("@%s (%d)", result.Username, result.UserID)
		} else if result.Name != "" {
			displayName = fmt.Sprintf("%s (%d)", result.Name, result.UserID)
		}

		fmt.Println(ui.Accent.Render(fmt.Sprintf("  🤝 Paired with %s", displayName)))

		addMore, err := ConfirmAction("👥 Add another user?", "")
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
				Title("🧠 Default Chat Model").
				Description("Which model to use for Telegram chat sessions.").
				Options(
					huh.NewOption("⚡ Sonnet — fast & capable (recommended)", "sonnet"),
					huh.NewOption("🧠 Opus — most capable", "opus"),
					huh.NewOption("🐇 Haiku — fastest & cheapest", "haiku"),
					huh.NewOption("◀️  Back", "__back__"),
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
				Title("💪 Default Effort Level").
				Description("Controls how much thinking effort Claude uses.").
				Options(
					huh.NewOption("🔥 High — full capability (recommended)", "high"),
					huh.NewOption("⚖️  Medium — balanced", "medium"),
					huh.NewOption("💨 Low — token-efficient", "low"),
					huh.NewOption("💎 Max — absolute maximum (Opus only)", "max"),
					huh.NewOption("◀️  Back", "__back__"),
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
