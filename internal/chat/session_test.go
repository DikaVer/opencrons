package chat

import (
	"path/filepath"
	"testing"

	"github.com/DikaVer/opencrons/internal/platform"
	"github.com/DikaVer/opencrons/internal/storage"
)

func setupSessionTest(t *testing.T) (*SessionManager, *storage.DB) {
	t.Helper()
	dir := t.TempDir()
	platform.SetBaseDir(dir)
	t.Cleanup(func() { platform.SetBaseDir("") })

	dbPath := filepath.Join(dir, "test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	return NewSessionManager(db), db
}

func TestGetOrCreateSession_New(t *testing.T) {
	sm, _ := setupSessionTest(t)

	session, isNew, err := sm.GetOrCreateSession(100, 200)
	if err != nil {
		t.Fatalf("GetOrCreateSession: %v", err)
	}
	if !isNew {
		t.Error("expected isNew=true for first call")
	}
	if session == nil {
		t.Fatal("expected non-nil session")
	}
	if session.ID == "" {
		t.Error("expected non-empty session ID")
	}
}

func TestGetOrCreateSession_Existing(t *testing.T) {
	sm, _ := setupSessionTest(t)

	first, _, _ := sm.GetOrCreateSession(100, 200)
	second, isNew, err := sm.GetOrCreateSession(100, 200)
	if err != nil {
		t.Fatalf("GetOrCreateSession (second): %v", err)
	}
	if isNew {
		t.Error("expected isNew=false for second call")
	}
	if second.ID != first.ID {
		t.Errorf("session ID changed: %q -> %q", first.ID, second.ID)
	}
}

func TestClearSession(t *testing.T) {
	sm, db := setupSessionTest(t)

	sm.GetOrCreateSession(100, 200)

	if err := sm.ClearSession(100); err != nil {
		t.Fatalf("ClearSession: %v", err)
	}

	active, err := db.GetActiveSession(100)
	if err != nil {
		t.Fatalf("GetActiveSession: %v", err)
	}
	if active != nil {
		t.Error("expected nil session after ClearSession")
	}
}

func TestCreateSession_DefaultConfig(t *testing.T) {
	sm, _ := setupSessionTest(t)

	session, err := sm.CreateSession(100, 200)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Default chat config: model=sonnet, effort=high
	if session.Model != "sonnet" {
		t.Errorf("Model = %q, want %q", session.Model, "sonnet")
	}
	if session.Effort != "high" {
		t.Errorf("Effort = %q, want %q", session.Effort, "high")
	}
}
