// handlers_commands.go implements Telegram bot command handlers.
// Each function handles one slash command: /jobs, /model, /effort, /status, /help.
package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/DikaVer/opencrons/internal/config"
	"github.com/DikaVer/opencrons/internal/platform"
)

func (b *Bot) handleJobs(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID

	jobs, err := config.LoadAllJobs(platform.SchedulesDir())
	if err != nil {
		_ = b.SendPlain(ctx, chatID, fmt.Sprintf("Error loading jobs: %v", err))
		return
	}

	if len(jobs) == 0 {
		_ = b.SendPlain(ctx, chatID, "No jobs configured. Use the TUI to add jobs.")
		return
	}

	// Build inline keyboard with jobs
	var rows [][]models.InlineKeyboardButton
	for _, j := range jobs {
		statusIcon := "+"
		if !j.Enabled {
			statusIcon = "-"
		}
		label := fmt.Sprintf("[%s] %s (%s)", statusIcon, j.Name, j.Schedule)
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: label, CallbackData: "job:select:" + j.Name},
		})
	}

	kb := &models.InlineKeyboardMarkup{InlineKeyboard: rows}

	_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        "Scheduled Jobs:\n(+ enabled, - disabled)",
		ReplyMarkup: kb,
	})
}

func (b *Bot) handleModel(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID

	// Check current session model
	session, _ := b.db.GetActiveSession(update.Message.From.ID)
	currentModel := "sonnet"
	if session != nil {
		currentModel = session.Model
	}

	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: modelLabel("sonnet", currentModel), CallbackData: "model:sonnet"},
				{Text: modelLabel("opus", currentModel), CallbackData: "model:opus"},
				{Text: modelLabel("haiku", currentModel), CallbackData: "model:haiku"},
			},
		},
	}

	_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        fmt.Sprintf("Current model: *%s*\nSelect a model:", currentModel),
		ParseMode:   models.ParseModeMarkdown,
		ReplyMarkup: kb,
	})
}

func (b *Bot) handleEffort(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID

	session, _ := b.db.GetActiveSession(update.Message.From.ID)
	currentEffort := "high"
	if session != nil {
		currentEffort = session.Effort
	}

	kb := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: effortLabel("low", currentEffort), CallbackData: "effort:low"},
				{Text: effortLabel("medium", currentEffort), CallbackData: "effort:medium"},
			},
			{
				{Text: effortLabel("high", currentEffort), CallbackData: "effort:high"},
				{Text: effortLabel("max", currentEffort), CallbackData: "effort:max"},
			},
		},
	}

	_, _ = tgBot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        fmt.Sprintf("Current effort: *%s*\nSelect effort level:", currentEffort),
		ParseMode:   models.ParseModeMarkdown,
		ReplyMarkup: kb,
	})
}

func (b *Bot) handleStatus(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID

	// Daemon status
	pid, running := platform.CheckDaemonRunning()
	var statusLines []string
	if running {
		statusLines = append(statusLines, fmt.Sprintf("Daemon: running (PID %d)", pid))
	} else {
		statusLines = append(statusLines, "Daemon: stopped")
	}

	// Job count
	jobs, _ := config.LoadAllJobs(platform.SchedulesDir())
	enabled := 0
	for _, j := range jobs {
		if j.Enabled {
			enabled++
		}
	}
	statusLines = append(statusLines, fmt.Sprintf("Jobs: %d total, %d enabled", len(jobs), enabled))

	// Active session
	session, _ := b.db.GetActiveSession(update.Message.From.ID)
	if session != nil {
		statusLines = append(statusLines, fmt.Sprintf("Session: active (model=%s, effort=%s)", session.Model, session.Effort))
		statusLines = append(statusLines, fmt.Sprintf("Working dir: %s", session.WorkingDir))
	} else {
		statusLines = append(statusLines, "Session: none (send a message to start)")
	}

	_ = b.SendPlain(ctx, chatID, strings.Join(statusLines, "\n"))
}

func (b *Bot) handleHelp(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID

	help := `OpenCron Bot

Commands:
/new - Start a fresh chat session
/stop - Stop the current running query
/jobs - List and manage scheduled jobs
/model - Change the Claude model
/effort - Change the effort level
/status - Show daemon and session status
/help - Show this help message

Send any text message to chat with Claude.`

	_ = b.SendPlain(ctx, chatID, help)
}
