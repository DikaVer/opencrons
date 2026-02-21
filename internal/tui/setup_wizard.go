package tui

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/dika-maulidal/cli-scheduler/internal/platform"
	"github.com/dika-maulidal/cli-scheduler/internal/provider"
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
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#cba6f7"))
	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6c7086"))
	successStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#a6e3a1"))

	// Step 1: Welcome
	fmt.Println()
	fmt.Println(titleStyle.Render("  Welcome to CLI Scheduler"))
	fmt.Println()
	fmt.Println(dimStyle.Render("  CLI Scheduler runs Claude Code tasks on cron schedules."))
	fmt.Println(dimStyle.Render("  This wizard will help you set up your environment."))
	fmt.Println()

	// Step 2: Provider detection
	fmt.Println(titleStyle.Render("  Step 1: AI Provider"))
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
		fmt.Println(dimStyle.Render("  Claude Code CLI not found on PATH."))
		fmt.Println(dimStyle.Render("  Install it: npm install -g @anthropic-ai/claude-code"))
		fmt.Println()
		PrintPressEnter()
	}

	if version := p.Version(); version != "" {
		fmt.Printf("  %s %s\n", dimStyle.Render("Claude Code:"), successStyle.Render(version))
	}
	fmt.Println()

	// Step 3: Messenger
	fmt.Println(titleStyle.Render("  Step 2: Messenger Integration"))
	fmt.Println()

	var messengerType string
	messengerForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Messenger Platform").
				Description("Connect a messenger to chat with Claude and manage jobs remotely.").
				Options(
					huh.NewOption("Telegram                Chat with Claude + manage jobs via Telegram bot", "telegram"),
					huh.NewOption("Skip                    Use TUI only (can configure later in Settings)", ""),
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
		result.Messenger = msgSettings

		// Step 5: Chat defaults
		fmt.Println()
		fmt.Println(titleStyle.Render("  Step 3: Chat Defaults"))
		fmt.Println()

		chatSettings, err := runChatDefaultsForm()
		if err != nil {
			return nil, err
		}
		result.Chat = chatSettings
	}

	// Step 6: Daemon mode
	fmt.Println()
	fmt.Println(titleStyle.Render("  Step 4: Daemon Configuration"))
	fmt.Println()

	var daemonMode string
	daemonForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Daemon Mode").
				Description("How should the scheduler daemon run?").
				Options(
					huh.NewOption("Background process      Start manually with 'scheduler start'", "background"),
					huh.NewOption("System service          Auto-start on boot (requires admin)", "service"),
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
	fmt.Println(successStyle.Render("  Setup complete!"))
	fmt.Println()

	return result, nil
}

// runTelegramSetup handles the Telegram bot configuration flow.
func runTelegramSetup() (*platform.MessengerSettings, error) {
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))

	fmt.Println()
	fmt.Println(dimStyle.Render("  Create a bot with @BotFather on Telegram to get your token."))
	fmt.Println(dimStyle.Render("  1. Open Telegram and search for @BotFather"))
	fmt.Println(dimStyle.Render("  2. Send /newbot and follow the instructions"))
	fmt.Println(dimStyle.Render("  3. Copy the HTTP API token"))
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
		return nil, err
	}
	botToken = strings.TrimSpace(botToken)

	// Pairing mode
	var pairingMode string
	pairingForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("User Authorization").
				Description("How should the bot verify who can use it?").
				Options(
					huh.NewOption("Pairing token           Generate a code, send it to your bot to pair", "gatherToken"),
					huh.NewOption("Allow list              Manually enter Telegram user IDs or @usernames", "allowList"),
				).
				Value(&pairingMode),
		),
	).WithTheme(theme)

	if err := pairingForm.Run(); err != nil {
		return nil, err
	}

	settings := &platform.MessengerSettings{
		Type:         "telegram",
		BotToken:     botToken,
		Pairing:      pairingMode,
		AllowedUsers: make(map[string]bool),
	}

	if pairingMode == "gatherToken" {
		// Generate pairing code
		code := generatePairingCode()
		fmt.Println()
		fmt.Printf("  Your pairing code: %s\n", lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f5c2e7")).Render(code))
		fmt.Println()
		fmt.Println(dimStyle.Render("  Send any message to your bot on Telegram."))
		fmt.Println(dimStyle.Render("  The bot will reply with your pairing code."))
		fmt.Println(dimStyle.Render("  Enter the code below to confirm pairing."))
		fmt.Println()

		// Store the code for later verification during bot startup
		// For now, we save the code as a special allowed_users entry
		// The actual pairing happens when the bot starts in the daemon
		settings.AllowedUsers["__pairing_code__"] = false
		settings.AllowedUsers["__code:"+code+"__"] = true

		var confirmCode string
		confirmForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Confirm Pairing Code").
					Description("Enter the code your bot sent you to confirm pairing.").
					Value(&confirmCode).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return fmt.Errorf("code is required")
						}
						if strings.TrimSpace(s) != code {
							return fmt.Errorf("code does not match — check your Telegram bot")
						}
						return nil
					}),
			),
		).WithTheme(theme)

		if err := confirmForm.Run(); err != nil {
			return nil, err
		}

		// Clean up pairing entries — actual user will be added when bot starts
		delete(settings.AllowedUsers, "__pairing_code__")
		delete(settings.AllowedUsers, "__code:"+code+"__")
		settings.AllowedUsers["__pending_pairing__"] = true

	} else {
		// Allow list mode
		for {
			var userID string
			userForm := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Telegram User").
						Description("Enter a Telegram @username or numeric user ID.").
						Placeholder("@username or 123456789").
						Value(&userID).
						Validate(func(s string) error {
							if strings.TrimSpace(s) == "" {
								return fmt.Errorf("user ID is required")
							}
							return nil
						}),
				),
			).WithTheme(theme)

			if err := userForm.Run(); err != nil {
				return nil, err
			}

			settings.AllowedUsers[strings.TrimSpace(userID)] = true

			addMore, err := ConfirmAction("Add another user?", "")
			if err != nil {
				return nil, err
			}
			if !addMore {
				break
			}
		}
	}

	return settings, nil
}

// runChatDefaultsForm gets chat default settings.
func runChatDefaultsForm() (*platform.ChatSettings, error) {
	var model, effort string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Default Chat Model").
				Description("Which model to use for Telegram chat sessions.").
				Options(
					huh.NewOption("Sonnet — fast, capable, cost-effective (recommended)", "sonnet"),
					huh.NewOption("Opus — most capable, best for complex reasoning", "opus"),
					huh.NewOption("Haiku — fastest, cheapest, good for simple tasks", "haiku"),
				).
				Value(&model),
			huh.NewSelect[string]().
				Title("Default Effort Level").
				Description("Controls how much thinking effort Claude uses.").
				Options(
					huh.NewOption("High — full capability (recommended)", "high"),
					huh.NewOption("Medium — balanced speed and cost", "medium"),
					huh.NewOption("Low — most token-efficient", "low"),
					huh.NewOption("Max — absolute maximum (Opus only)", "max"),
				).
				Value(&effort),
		),
	).WithTheme(theme)

	if err := form.Run(); err != nil {
		return nil, err
	}

	return &platform.ChatSettings{
		Model:  model,
		Effort: effort,
	}, nil
}

// generatePairingCode creates a random 6-character alphanumeric code.
func generatePairingCode() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no I/O/0/1 to avoid confusion
	code := make([]byte, 6)
	for i := range code {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		code[i] = chars[n.Int64()]
	}
	return string(code)
}
