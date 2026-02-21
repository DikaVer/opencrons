// Package telegram implements the Telegram messenger integration using
// go-telegram/bot. The Bot struct manages the database, settings, chat
// components, and per-user processing locks (sync.Map). It provides
// Start/Stop lifecycle, Send/SendPlain message delivery, IsAuthorized
// user checks, command and callback handler registration, auth middleware
// for both message and callback handlers, and tryLock/unlock for per-user
// concurrency control.
package telegram

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/dika-maulidal/opencron/internal/chat"
	"github.com/dika-maulidal/opencron/internal/logger"
	"github.com/dika-maulidal/opencron/internal/platform"
	"github.com/dika-maulidal/opencron/internal/storage"
)

// Bot wraps the Telegram bot with OpenCron integration.
type Bot struct {
	bot      *bot.Bot
	db       *storage.DB
	settings *platform.MessengerSettings
	stdlog   *log.Logger
	cancel   context.CancelFunc

	// Chat components (injected via SetChatComponents)
	sessionMgr *chat.SessionManager
	chatRunner *chat.Runner

	// Per-user processing lock (prevents concurrent message processing)
	processing sync.Map // map[int64]bool

	// Per-user cancel functions for in-flight queries (allows /stop)
	cancels sync.Map // map[int64]context.CancelFunc
}

// New creates a new Telegram bot instance.
func New(db *storage.DB, settings *platform.MessengerSettings, stdlog *log.Logger) (*Bot, error) {
	if settings == nil || settings.BotToken == "" {
		return nil, fmt.Errorf("telegram bot token not configured")
	}

	b := &Bot{
		db:       db,
		settings: settings,
		stdlog:   stdlog,
	}

	opts := []bot.Option{
		bot.WithDefaultHandler(b.handleDefault),
		bot.WithErrorsHandler(func(err error) {
			stdlog.Printf("[telegram] error: %v", err)
			logger.Debug("Telegram bot error: %v", err)
		}),
	}

	telegramBot, err := bot.New(settings.BotToken, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating telegram bot: %w", err)
	}

	b.bot = telegramBot
	b.registerHandlers()

	return b, nil
}

// Start begins the bot's long-polling event loop. Blocks until ctx is cancelled.
func (b *Bot) Start(ctx context.Context) {
	ctx, b.cancel = context.WithCancel(ctx)

	// Register commands with Telegram so they appear in the commands menu
	b.setCommands(ctx)

	b.stdlog.Println("[telegram] Bot started")
	logger.Info("Telegram bot started")

	b.bot.Start(ctx)
}

// setCommands registers the bot's command list with Telegram.
func (b *Bot) setCommands(ctx context.Context) {
	_, err := b.bot.SetMyCommands(ctx, &bot.SetMyCommandsParams{
		Commands: []models.BotCommand{
			{Command: "new", Description: "Start a new conversation"},
			{Command: "jobs", Description: "List scheduled jobs"},
			{Command: "model", Description: "Change the AI model"},
			{Command: "effort", Description: "Change the effort level"},
			{Command: "stop", Description: "Stop the current query"},
			{Command: "status", Description: "Show bot status"},
			{Command: "help", Description: "Show help message"},
		},
	})
	if err != nil {
		b.stdlog.Printf("[telegram] Failed to set commands: %v", err)
		logger.Debug("SetMyCommands error: %v", err)
	}
}

// Stop gracefully shuts down the bot.
func (b *Bot) Stop() {
	if b.cancel != nil {
		b.cancel()
	}
	b.stdlog.Println("[telegram] Bot stopped")
	logger.Info("Telegram bot stopped")
}

// Send sends a text message to the given chat with HTML formatting.
// Markdown from Claude responses is converted to Telegram HTML to avoid
// MarkdownV2 escaping issues (e.g. backslashes in Windows paths).
func (b *Bot) Send(ctx context.Context, chatID int64, text string) error {
	_, err := b.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      markdownToHTML(text),
		ParseMode: models.ParseModeHTML,
	})
	return err
}

// SendPlain sends a plain text message without markdown parsing.
func (b *Bot) SendPlain(ctx context.Context, chatID int64, text string) error {
	_, err := b.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
	return err
}

// IsAuthorized checks if a user is allowed to interact with the bot.
func (b *Bot) IsAuthorized(userID int64) bool {
	if b.settings == nil || b.settings.AllowedUsers == nil {
		return false
	}

	idStr := strconv.FormatInt(userID, 10)
	return b.settings.AllowedUsers[idStr]
}

// registerHandlers sets up all command and callback handlers.
func (b *Bot) registerHandlers() {
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/new", bot.MatchTypePrefix, b.authMiddleware(b.handleNew))
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/jobs", bot.MatchTypePrefix, b.authMiddleware(b.handleJobs))
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/model", bot.MatchTypePrefix, b.authMiddleware(b.handleModel))
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/effort", bot.MatchTypePrefix, b.authMiddleware(b.handleEffort))
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/stop", bot.MatchTypePrefix, b.authMiddleware(b.handleStopQuery))
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/status", bot.MatchTypePrefix, b.authMiddleware(b.handleStatus))
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/help", bot.MatchTypePrefix, b.authMiddleware(b.handleHelp))
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypePrefix, b.authMiddleware(b.handleHelp))

	// Callback query handlers for inline keyboards
	b.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "job:", bot.MatchTypePrefix, b.authCallbackMiddleware(b.handleJobCallback))
	b.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "model:", bot.MatchTypePrefix, b.authCallbackMiddleware(b.handleModelCallback))
	b.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "effort:", bot.MatchTypePrefix, b.authCallbackMiddleware(b.handleEffortCallback))
}

// handleDefault processes non-command text messages as chat messages.
func (b *Bot) handleDefault(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.Text == "" {
		return
	}

	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID

	// Authorization check
	if !b.IsAuthorized(userID) {
		b.SendPlain(ctx, chatID, "You are not authorized to use this bot.")
		return
	}

	// Process as chat message
	b.handleChatMessage(ctx, tgBot, update)
}

// authMiddleware wraps a handler with authorization check.
func (b *Bot) authMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}
		if !b.IsAuthorized(update.Message.From.ID) {
			b.SendPlain(ctx, update.Message.Chat.ID, "You are not authorized to use this bot.")
			return
		}
		next(ctx, tgBot, update)
	}
}

// authCallbackMiddleware wraps a callback handler with authorization check.
func (b *Bot) authCallbackMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
		if update.CallbackQuery == nil {
			return
		}
		if !b.IsAuthorized(update.CallbackQuery.From.ID) {
			tgBot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
				CallbackQueryID: update.CallbackQuery.ID,
				Text:            "Not authorized",
				ShowAlert:       true,
			})
			return
		}
		next(ctx, tgBot, update)
	}
}

// tryLock attempts to acquire a per-user processing lock.
func (b *Bot) tryLock(userID int64) bool {
	_, loaded := b.processing.LoadOrStore(userID, true)
	return !loaded // true if we acquired the lock (wasn't already present)
}

// unlock releases a per-user processing lock.
func (b *Bot) unlock(userID int64) {
	b.processing.Delete(userID)
}
