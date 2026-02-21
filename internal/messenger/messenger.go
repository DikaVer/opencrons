// Package messenger defines the Messenger interface for pluggable messaging
// platform integrations. Each implementation (e.g. Telegram) provides
// Start/Stop lifecycle, message sending, and user authorization.
package messenger

import "context"

// Messenger defines the interface for a messaging platform integration.
type Messenger interface {
	// Type returns the messenger type identifier (e.g. "telegram").
	Type() string
	// Start begins the messenger's event loop (blocking).
	Start(ctx context.Context) error
	// Stop gracefully shuts down the messenger.
	Stop()
	// Send sends a text message to the given chat.
	Send(chatID int64, text string) error
	// IsAuthorized checks if a user is allowed to interact with the bot.
	IsAuthorized(userID int64) bool
}
