package chat

import (
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/dika-maulidal/cli-scheduler/internal/platform"
	"github.com/dika-maulidal/cli-scheduler/internal/storage"
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
func (sm *SessionManager) GetOrCreateSession(userID, chatID int64) (*storage.ChatSession, error) {
	session, err := sm.db.GetActiveSession(userID)
	if err != nil {
		return nil, fmt.Errorf("checking active session: %w", err)
	}

	if session != nil {
		// Touch the session to update last activity
		_ = sm.db.TouchSession(session.ID)
		return session, nil
	}

	// Create a new session
	return sm.CreateSession(userID, chatID)
}

// CreateSession creates a new chat session for a user.
func (sm *SessionManager) CreateSession(userID, chatID int64) (*storage.ChatSession, error) {
	chatCfg := platform.GetChatConfig()

	// Use the user's home directory or a default working directory
	workingDir, _ := os.UserHomeDir()
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
