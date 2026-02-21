// chat.go handles free-text Telegram messages as Claude conversations.
//
// SetChatComponents injects the session manager and runner after bot creation.
// handleChatMessage acquires a per-user lock, gets or creates a session,
// sends typing indicators, runs "claude -p" with --session-id (new) or
// --resume (existing), logs the exchange to the database, sends the response
// to Telegram, and echoes a summary to the terminal.
// sendTypingLoop refreshes the typing indicator every 5 seconds.
package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/DikaVer/opencron/internal/chat"
	"github.com/DikaVer/opencron/internal/logger"
	"github.com/DikaVer/opencron/internal/ui"
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
	session, isNew, err := b.sessionMgr.GetOrCreateSession(userID, chatID)
	if err != nil {
		b.SendPlain(ctx, chatID, fmt.Sprintf("Error creating session: %v", err))
		logger.Debug("Session error for user %d: %v", userID, err)
		return
	}

	// Create a cancellable context so /stop can kill this query
	runCtx, runCancel := context.WithCancel(ctx)
	b.cancels.Store(userID, runCancel)
	defer func() {
		b.cancels.Delete(userID)
		runCancel()
	}()

	// Start typing indicator loop
	typingCtx, typingCancel := context.WithCancel(runCtx)
	go b.sendTypingLoop(typingCtx, tgBot, chatID)

	// Run Claude (no timeout — runs until completion or /stop)
	result, err := b.chatRunner.Run(runCtx, session, text, isNew)
	typingCancel() // Stop typing indicator

	// If --resume fails because Claude has no transcript on disk (e.g. the
	// first call in this session errored), fall back to a fresh session.
	if err != nil && !isNew && strings.Contains(err.Error(), "No conversation found") {
		logger.Debug("No Claude transcript for session %s, creating fresh session for user %d", session.ID, userID)
		_ = b.sessionMgr.ClearSession(userID)
		if newSession, sessErr := b.sessionMgr.CreateSession(userID, chatID); sessErr == nil {
			session = newSession
			isNew = true
			typingCtx2, typingCancel2 := context.WithCancel(runCtx)
			go b.sendTypingLoop(typingCtx2, tgBot, chatID)
			result, err = b.chatRunner.Run(runCtx, newSession, text, true)
			typingCancel2()
		}
	}

	// If session lock is stale ("already in use"), wait briefly and retry
	// to preserve conversation history before falling back to a new session.
	if err != nil && strings.Contains(err.Error(), "already in use") {
		logger.Debug("Session %s in use for user %d, retrying after delay", session.ID, userID)

		select {
		case <-runCtx.Done():
		case <-time.After(2 * time.Second):
			typingCtx2, typingCancel2 := context.WithCancel(runCtx)
			go b.sendTypingLoop(typingCtx2, tgBot, chatID)
			result, err = b.chatRunner.Run(runCtx, session, text, isNew)
			typingCancel2()
		}

		// Still failing — create a fresh session as last resort
		if err != nil && strings.Contains(err.Error(), "already in use") {
			logger.Debug("Retry failed, creating fresh session for user %d", userID)
			_ = b.sessionMgr.ClearSession(userID)
			if newSession, sessErr := b.sessionMgr.CreateSession(userID, chatID); sessErr == nil {
				session = newSession
				isNew = true
				typingCtx3, typingCancel3 := context.WithCancel(runCtx)
				go b.sendTypingLoop(typingCtx3, tgBot, chatID)
				result, err = b.chatRunner.Run(runCtx, newSession, text, true)
				typingCancel3()
			}
		}
	}

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
	truncatedText := ui.Truncate(text, 100)
	truncatedResponse := ui.Truncate(result.Response, 200)

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
