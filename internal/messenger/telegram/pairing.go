// pairing.go provides a temporary Telegram bot for code-based user pairing
// during the setup wizard. PairingBot generates cryptographically random
// 6-digit codes when users send it a message and validates codes for single
// use. StartPairingBot verifies the bot token via GetMe and starts
// long-polling in the background.
package telegram

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// PairingResult holds the outcome of a live pairing session.
type PairingResult struct {
	UserID   int64
	Username string
	Name     string
}

// PairingBot runs a temporary Telegram bot for code-based user pairing during setup.
type PairingBot struct {
	botName string
	cancel  context.CancelFunc

	mu    sync.Mutex
	codes map[string]*PairingResult // code → user info
}

// StartPairingBot creates and starts a temporary bot that generates pairing codes
// when users message it. Validates the token before starting.
func StartPairingBot(token string) (*PairingBot, error) {
	pb := &PairingBot{
		codes: make(map[string]*PairingResult),
	}

	tgBot, err := bot.New(token, bot.WithDefaultHandler(func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		userID := update.Message.From.ID
		chatID := update.Message.Chat.ID

		name := update.Message.From.FirstName
		if update.Message.From.LastName != "" {
			name += " " + update.Message.From.LastName
		}
		username := update.Message.From.Username

		code := generateCode()

		pb.mu.Lock()
		pb.codes[code] = &PairingResult{
			UserID:   userID,
			Username: username,
			Name:     name,
		}
		pb.mu.Unlock()

		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   fmt.Sprintf("Your pairing code: %s\n\nEnter this code in the CLI to complete pairing.", code),
		})
	}))
	if err != nil {
		return nil, fmt.Errorf("creating telegram bot: %w", err)
	}

	// Verify the token by calling GetMe
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	me, err := tgBot.GetMe(ctx)
	if err != nil {
		return nil, fmt.Errorf("invalid bot token: %w", err)
	}
	pb.botName = me.Username

	// Start long-polling in background
	bgCtx, bgCancel := context.WithCancel(context.Background())
	pb.cancel = bgCancel
	go tgBot.Start(bgCtx)

	// Small delay to let long-polling establish
	time.Sleep(500 * time.Millisecond)

	return pb, nil
}

// BotName returns the bot's @username (available after successful start).
func (pb *PairingBot) BotName() string {
	return pb.botName
}

// ValidateCode looks up a pairing code and returns the associated user info.
// The code is consumed on successful validation (single use).
func (pb *PairingBot) ValidateCode(code string) (*PairingResult, error) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	result, ok := pb.codes[code]
	if !ok {
		return nil, fmt.Errorf("invalid pairing code")
	}

	delete(pb.codes, code)
	return result, nil
}

// Stop shuts down the temporary bot.
func (pb *PairingBot) Stop() {
	if pb.cancel != nil {
		pb.cancel()
	}
}

// generateCode returns a cryptographically random 6-digit code.
func generateCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		// Fallback: extremely unlikely to fail
		return "000000"
	}
	return fmt.Sprintf("%06d", n.Int64())
}
