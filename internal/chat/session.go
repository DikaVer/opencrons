// File session.go manages chat session lifecycle for Telegram users.
//
// SessionManager maps each Telegram user to a unique Claude Code session
// UUID persisted in SQLite via [storage.DB]. It provides GetOrCreateSession
// to lazily initialize sessions, CreateSession to start a fresh session
// with the current chat configuration (model, effort), and ClearSession
// to deactivate a user's active session (triggered by the /new command).
package chat

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/dika-maulidal/opencron/internal/platform"
	"github.com/dika-maulidal/opencron/internal/storage"
)

// SessionManager manages chat sessions for users.
type SessionManager struct {
	db *storage.DB
}

// NewSessionManager creates a new session manager.
func NewSessionManager(db *storage.DB) *SessionManager {
	return &SessionManager{db: db}
}

// GetOrCreateSession returns the active session for a user, creating one if needed.
// The returned bool is true when a brand-new session was created (first message).
func (sm *SessionManager) GetOrCreateSession(userID, chatID int64) (*storage.ChatSession, bool, error) {
	session, err := sm.db.GetActiveSession(userID)
	if err != nil {
		return nil, false, fmt.Errorf("checking active session: %w", err)
	}

	if session != nil {
		// Touch the session to update last activity
		_ = sm.db.TouchSession(session.ID)
		return session, false, nil
	}

	// Create a new session
	newSession, err := sm.CreateSession(userID, chatID)
	return newSession, true, err
}

// CreateSession creates a new chat session for a user.
func (sm *SessionManager) CreateSession(userID, chatID int64) (*storage.ChatSession, error) {
	chatCfg := platform.GetChatConfig()

	// Use the managed workspace directory so Claude sees CLAUDE.md/.claude context.
	workingDir := platform.WorkspaceDir()
	if workingDir == "" {
		workingDir = "."
	}

	now := time.Now()
	session := &storage.ChatSession{
		ID:         uuid.New().String(),
		UserID:     userID,
		ChatID:     chatID,
		Model:      chatCfg.Model,
		Effort:     chatCfg.Effort,
		WorkingDir: workingDir,
		Active:     true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := sm.db.CreateSession(session); err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}

	return session, nil
}

// ClearSession deactivates the active session for a user.
func (sm *SessionManager) ClearSession(userID int64) error {
	return sm.db.DeactivateUserSessions(userID)
}
