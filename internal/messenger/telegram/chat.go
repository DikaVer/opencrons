package telegram

import (
	"context"
	"fmt"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/dika-maulidal/cli-scheduler/internal/chat"
	"github.com/dika-maulidal/cli-scheduler/internal/logger"
)

// SetChatComponents injects the session manager and runner into the bot.
// This must be called after bot creation and before Start.
func (b *Bot) SetChatComponents(sm *chat.SessionManager, runner *chat.Runner) {
	b.sessionMgr = sm
	b.chatRunner = runner
}

// handleChatMessage processes a regular text message as a chat with Claude.
func (b *Bot) handleChatMessage(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if b.sessionMgr == nil || b.chatRunner == nil {
		b.SendPlain(ctx, update.Message.Chat.ID, "Chat is not configured. Please restart the daemon.")
		return
	}

	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID
	text := update.Message.Text

	// Per-user concurrency lock
	if !b.tryLock(userID) {
		b.SendPlain(ctx, chatID, "Please wait, still processing your previous message...")
		return
	}
	defer b.unlock(userID)

	// Get or create session
	session, err := b.sessionMgr.GetOrCreateSession(userID, chatID)
	if err != nil {
		b.SendPlain(ctx, chatID, fmt.Sprintf("Error creating session: %v", err))
		logger.Debug("Session error for user %d: %v", userID, err)
		return
	}

	// Start typing indicator loop
	typingCtx, typingCancel := context.WithCancel(ctx)
	go b.sendTypingLoop(typingCtx, tgBot, chatID)

	// Run Claude
	result, err := b.chatRunner.Run(ctx, session, text)
	typingCancel() // Stop typing indicator

	if err != nil {
		errMsg := fmt.Sprintf("Error: %v\nTry /new for a fresh session.", err)
		b.SendPlain(ctx, chatID, errMsg)
		logger.Debug("Chat runner error for user %d: %v", userID, err)
		return
	}

	// Log to database (for visibility only)
	_ = b.db.AddChatLog(session.ID, "user", text, 0, 0)
	_ = b.db.AddChatLog(session.ID, "assistant", result.Response, result.CostUSD, result.Tokens)

	// Send response to Telegram
	// Try markdown first, fall back to plain text if it fails
	if err := b.Send(ctx, chatID, result.Response); err != nil {
		// Markdown parsing may fail for certain responses, try plain text
		b.SendPlain(ctx, chatID, result.Response)
	}

	// Terminal echo
	userName := update.Message.From.FirstName
	if update.Message.From.Username != "" {
		userName = "@" + update.Message.From.Username
	}
	truncatedText := text
	if len(truncatedText) > 100 {
		truncatedText = truncatedText[:100] + "..."
	}
	truncatedResponse := result.Response
	if len(truncatedResponse) > 200 {
		truncatedResponse = truncatedResponse[:200] + "..."
	}

	b.stdlog.Printf("[telegram] %s: %s", userName, truncatedText)
	b.stdlog.Printf("[telegram] Claude ($%.4f, %s): %s", result.CostUSD, result.Duration.Round(time.Millisecond), truncatedResponse)
}

// sendTypingLoop sends typing indicators every 5 seconds until context is cancelled.
func (b *Bot) sendTypingLoop(ctx context.Context, tgBot *bot.Bot, chatID int64) {
	// Send immediately
	tgBot.SendChatAction(ctx, &bot.SendChatActionParams{
		ChatID: chatID,
		Action: models.ChatActionTyping,
	})

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tgBot.SendChatAction(ctx, &bot.SendChatActionParams{
				ChatID: chatID,
				Action: models.ChatActionTyping,
			})
		}
	}
}
