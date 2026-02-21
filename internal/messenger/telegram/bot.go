package telegram

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/dika-maulidal/cli-scheduler/internal/chat"
	"github.com/dika-maulidal/cli-scheduler/internal/logger"
	"github.com/dika-maulidal/cli-scheduler/internal/platform"
	"github.com/dika-maulidal/cli-scheduler/internal/storage"
)

// Bot wraps the Telegram bot with scheduler integration.
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

	b.stdlog.Println("[telegram] Bot started")
	logger.Info("Telegram bot started")

	b.bot.Start(ctx)
}

// Stop gracefully shuts down the bot.
func (b *Bot) Stop() {
	if b.cancel != nil {
		b.cancel()
	}
	b.stdlog.Println("[telegram] Bot stopped")
	logger.Info("Telegram bot stopped")
}

// Send sends a text message to the given chat.
func (b *Bot) Send(ctx context.Context, chatID int64, text string) error {
	_, err := b.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeMarkdown,
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

	// Check by numeric ID
	idStr := strconv.FormatInt(userID, 10)
	if b.settings.AllowedUsers[idStr] {
		return true
	}

	// Check for pending pairing (allow all until first user pairs)
	if b.settings.AllowedUsers["__pending_pairing__"] {
		return true
	}

	return false
}

// CompletePairing adds a user ID to the allowed list and removes pending flag.
func (b *Bot) CompletePairing(userID int64) error {
	idStr := strconv.FormatInt(userID, 10)
	b.settings.AllowedUsers[idStr] = true
	delete(b.settings.AllowedUsers, "__pending_pairing__")

	// Persist to settings
	s := platform.LoadSettings()
	s.Messenger = b.settings
	return platform.SaveSettings(s)
}

// registerHandlers sets up all command and callback handlers.
func (b *Bot) registerHandlers() {
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/new", bot.MatchTypePrefix, b.authMiddleware(b.handleNew))
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/jobs", bot.MatchTypePrefix, b.authMiddleware(b.handleJobs))
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/model", bot.MatchTypePrefix, b.authMiddleware(b.handleModel))
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/effort", bot.MatchTypePrefix, b.authMiddleware(b.handleEffort))
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
		// If pending pairing, complete it
		if b.settings.AllowedUsers["__pending_pairing__"] {
			if err := b.CompletePairing(userID); err != nil {
				b.stdlog.Printf("[telegram] Pairing error: %v", err)
			}
			b.SendPlain(ctx, chatID, fmt.Sprintf("Paired successfully! Your user ID: %d\nSend any message to chat with Claude.", userID))
			b.stdlog.Printf("[telegram] User %d paired successfully", userID)
			return
		}

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
