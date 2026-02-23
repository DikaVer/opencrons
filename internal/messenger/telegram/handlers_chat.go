// handlers_chat.go implements Telegram handlers for chat session management.
//
// SetChatComponents injects the session manager and runner after bot creation.
// handleNew clears the current session so the next message starts fresh.
// handleStopQuery cancels an in-flight Claude query via the stored cancel func.
// handleChatMessage is the main free-text handler: acquires a per-user lock,
// gets or creates a session, sends typing indicators, runs "claude -p" with
// --session-id (new) or --resume (existing), logs the exchange to the database,
// and sends the response to Telegram.
// sendTypingLoop refreshes the typing indicator every 5 seconds.
package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/DikaVer/opencrons/internal/chat"
	"github.com/DikaVer/opencrons/internal/ui"
)

// SetChatComponents injects the session manager and runner into the bot.
// This must be called after bot creation and before Start.
func (b *Bot) SetChatComponents(sm *chat.SessionManager, runner *chat.Runner) {
	b.sessionMgr = sm
	b.chatRunner = runner
}

func (b *Bot) handleNew(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID

	// Deactivate existing sessions via session manager
	if b.sessionMgr != nil {
		if err := b.sessionMgr.ClearSession(userID); err != nil {
			slogger.Warn("error clearing session", "userID", userID, "err", err)
		}
	} else if err := b.db.DeactivateUserSessions(userID); err != nil {
		slogger.Warn("error deactivating sessions", "userID", userID, "err", err)
	}

	_ = b.SendPlain(ctx, chatID, "Session cleared. Send a message to start a fresh conversation with Claude.")
	b.stdlog.Printf("[telegram] User %d started new session", userID)
}

func (b *Bot) handleStopQuery(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID

	cancelVal, ok := b.cancels.Load(userID)
	if !ok {
		_ = b.SendPlain(ctx, chatID, "No active query to stop.")
		return
	}

	cancelFunc := cancelVal.(context.CancelFunc)
	cancelFunc()
	b.stdlog.Printf("[telegram] User %d stopped active query", userID)
}

// handleChatMessage processes a regular text message as a chat with Claude.
func (b *Bot) handleChatMessage(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	if b.sessionMgr == nil || b.chatRunner == nil {
		_ = b.SendPlain(ctx, update.Message.Chat.ID, "Chat is not configured. Please restart the daemon.")
		return
	}

	userID := update.Message.From.ID
	chatID := update.Message.Chat.ID
	text := update.Message.Text

	// Per-user concurrency lock
	if !b.tryLock(userID) {
		_ = b.SendPlain(ctx, chatID, "Please wait, still processing your previous message...")
		return
	}
	defer b.unlock(userID)

	// Get or create session
	session, isNew, err := b.sessionMgr.GetOrCreateSession(userID, chatID)
	if err != nil {
		_ = b.SendPlain(ctx, chatID, fmt.Sprintf("Error creating session: %v", err))
		slogger.Warn("session error", "userID", userID, "err", err)
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
		slogger.Debug("no Claude transcript, creating fresh session", "sessionID", session.ID, "userID", userID)
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
		slogger.Debug("session in use, retrying after delay", "sessionID", session.ID, "userID", userID)

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
			slogger.Debug("retry failed, creating fresh session", "userID", userID)
			_ = b.sessionMgr.ClearSession(userID)
			if newSession, sessErr := b.sessionMgr.CreateSession(userID, chatID); sessErr == nil {
				session = newSession
				typingCtx3, typingCancel3 := context.WithCancel(runCtx)
				go b.sendTypingLoop(typingCtx3, tgBot, chatID)
				result, err = b.chatRunner.Run(runCtx, newSession, text, true)
				typingCancel3()
			}
		}
	}

	if err != nil {
		errMsg := fmt.Sprintf("Error: %v\nTry /new for a fresh session.", err)
		_ = b.SendPlain(ctx, chatID, errMsg)
		slogger.Warn("chat runner error", "userID", userID, "err", err)
		return
	}

	slogger.Info("chat response delivered", "userID", userID, "cost", result.CostUSD, "duration", result.Duration)

	// Log to database (for visibility only)
	if err := b.db.AddChatLog(session.ID, "user", text, 0, 0); err != nil {
		slogger.Warn("chat log write failed", "sessionID", session.ID, "role", "user", "err", err)
	}
	if err := b.db.AddChatLog(session.ID, "assistant", result.Response, result.CostUSD, result.Tokens); err != nil {
		slogger.Warn("chat log write failed", "sessionID", session.ID, "role", "assistant", "err", err)
	}

	// Send response to Telegram; fall back to plain text if HTML parsing fails.
	if err := b.Send(ctx, chatID, result.Response); err != nil {
		_ = b.SendPlain(ctx, chatID, result.Response)
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
	_, _ = tgBot.SendChatAction(ctx, &bot.SendChatActionParams{
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
			_, _ = tgBot.SendChatAction(ctx, &bot.SendChatActionParams{
				ChatID: chatID,
				Action: models.ChatActionTyping,
			})
		}
	}
}
