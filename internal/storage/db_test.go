package storage

import (
	"path/filepath"
	"testing"
	"time"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestOpen_CreatesSchema(t *testing.T) {
	db := openTestDB(t)

	// Query sqlite_master for our tables
	tables := map[string]bool{"execution_logs": false, "chat_sessions": false, "chat_messages": false}
	rows, err := db.conn.Query("SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		if _, ok := tables[name]; ok {
			tables[name] = true
		}
	}

	for table, found := range tables {
		if !found {
			t.Errorf("table %q not found in schema", table)
		}
	}
}

func TestInsertLog_UpdateLog_GetLogs(t *testing.T) {
	db := openTestDB(t)

	entry := &ExecutionLog{
		JobID:       "j1",
		JobName:     "test-job",
		StartedAt:   time.Now(),
		Status:      "running",
		TriggerType: "scheduled",
	}

	id, err := db.InsertLog(entry)
	if err != nil {
		t.Fatalf("InsertLog: %v", err)
	}
	if id <= 0 {
		t.Error("expected positive log ID")
	}

	// Update the log
	now := time.Now()
	err = db.UpdateLog(id, now, 0, "/stdout.json", "/stderr.log",
		0.05, 1000, 500, 200, 100, "success", "")
	if err != nil {
		t.Fatalf("UpdateLog: %v", err)
	}

	logs, err := db.GetLogsByJobName("test-job", 10)
	if err != nil {
		t.Fatalf("GetLogsByJobName: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("got %d logs, want 1", len(logs))
	}
	if logs[0].Status != "success" {
		t.Errorf("status = %q, want %q", logs[0].Status, "success")
	}
}

func TestGetRecentLogs(t *testing.T) {
	db := openTestDB(t)

	// Insert 3 logs
	for i, name := range []string{"a", "b", "c"} {
		entry := &ExecutionLog{
			JobID:       "j" + name,
			JobName:     name,
			StartedAt:   time.Now().Add(time.Duration(i) * time.Second),
			Status:      "running",
			TriggerType: "scheduled",
		}
		id, err := db.InsertLog(entry)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.UpdateLog(id, time.Now(), 0, "", "", 0, 0, 0, 0, 0, "success", ""); err != nil {
			t.Fatal(err)
		}
	}

	logs, err := db.GetRecentLogs(2)
	if err != nil {
		t.Fatalf("GetRecentLogs: %v", err)
	}
	if len(logs) != 2 {
		t.Errorf("got %d logs, want 2", len(logs))
	}
	// Most recent first
	if logs[0].JobName != "c" {
		t.Errorf("first log = %q, want 'c'", logs[0].JobName)
	}
}

func TestGetUsageByJobName(t *testing.T) {
	db := openTestDB(t)

	for i := 0; i < 3; i++ {
		entry := &ExecutionLog{JobID: "j1", JobName: "usage-test", StartedAt: time.Now(), Status: "running", TriggerType: "scheduled"}
		id, err := db.InsertLog(entry)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.UpdateLog(id, time.Now(), 0, "", "", 0.10, 100, 50, 0, 0, "success", ""); err != nil {
			t.Fatal(err)
		}
	}

	usage, err := db.GetUsageByJobName("usage-test")
	if err != nil {
		t.Fatalf("GetUsageByJobName: %v", err)
	}
	if usage.TotalRuns != 3 {
		t.Errorf("TotalRuns = %d, want 3", usage.TotalRuns)
	}
	if usage.TotalCostUSD < 0.29 || usage.TotalCostUSD > 0.31 {
		t.Errorf("TotalCostUSD = %f, want ~0.30", usage.TotalCostUSD)
	}
}

func TestGetTotalUsage(t *testing.T) {
	db := openTestDB(t)

	for _, name := range []string{"a", "b"} {
		entry := &ExecutionLog{JobID: name, JobName: name, StartedAt: time.Now(), Status: "running", TriggerType: "scheduled"}
		id, err := db.InsertLog(entry)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.UpdateLog(id, time.Now(), 0, "", "", 0.05, 50, 25, 0, 0, "success", ""); err != nil {
			t.Fatal(err)
		}
	}

	usage, err := db.GetTotalUsage()
	if err != nil {
		t.Fatalf("GetTotalUsage: %v", err)
	}
	if usage.TotalRuns != 2 {
		t.Errorf("TotalRuns = %d, want 2", usage.TotalRuns)
	}
}

func TestCreateSession_GetActiveSession(t *testing.T) {
	db := openTestDB(t)

	now := time.Now()
	session := &ChatSession{
		ID:         "sess-1",
		UserID:     123,
		ChatID:     456,
		Model:      "sonnet",
		Effort:     "high",
		WorkingDir: "/tmp",
		Active:     true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := db.CreateSession(session); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	active, err := db.GetActiveSession(123)
	if err != nil {
		t.Fatalf("GetActiveSession: %v", err)
	}
	if active == nil {
		t.Fatal("expected active session, got nil")
	}
	if active.ID != "sess-1" {
		t.Errorf("ID = %q, want %q", active.ID, "sess-1")
	}
	if active.Model != "sonnet" {
		t.Errorf("Model = %q, want %q", active.Model, "sonnet")
	}
}

func TestDeactivateUserSessions(t *testing.T) {
	db := openTestDB(t)

	now := time.Now()
	session := &ChatSession{ID: "sess-2", UserID: 999, ChatID: 1, Model: "sonnet", Effort: "high", WorkingDir: "/tmp", Active: true, CreatedAt: now, UpdatedAt: now}
	if err := db.CreateSession(session); err != nil {
		t.Fatal(err)
	}

	if err := db.DeactivateUserSessions(999); err != nil {
		t.Fatalf("DeactivateUserSessions: %v", err)
	}

	active, err := db.GetActiveSession(999)
	if err != nil {
		t.Fatalf("GetActiveSession: %v", err)
	}
	if active != nil {
		t.Error("expected nil session after deactivation")
	}
}

func TestTouchSession(t *testing.T) {
	db := openTestDB(t)

	old := time.Now().Add(-time.Hour)
	session := &ChatSession{ID: "sess-3", UserID: 111, ChatID: 1, Model: "sonnet", Effort: "high", WorkingDir: "/tmp", Active: true, CreatedAt: old, UpdatedAt: old}
	if err := db.CreateSession(session); err != nil {
		t.Fatal(err)
	}

	time.Sleep(10 * time.Millisecond)
	if err := db.TouchSession("sess-3"); err != nil {
		t.Fatalf("TouchSession: %v", err)
	}

	active, _ := db.GetActiveSession(111)
	if active == nil {
		t.Fatal("expected session after touch")
	}
	if !active.UpdatedAt.After(old) {
		t.Error("UpdatedAt was not updated by TouchSession")
	}
}

func TestUpdateSessionModel(t *testing.T) {
	db := openTestDB(t)

	now := time.Now()
	session := &ChatSession{ID: "sess-4", UserID: 222, ChatID: 1, Model: "sonnet", Effort: "high", WorkingDir: "/tmp", Active: true, CreatedAt: now, UpdatedAt: now}
	if err := db.CreateSession(session); err != nil {
		t.Fatal(err)
	}

	if err := db.UpdateSessionModel("sess-4", "opus"); err != nil {
		t.Fatalf("UpdateSessionModel: %v", err)
	}

	active, _ := db.GetActiveSession(222)
	if active.Model != "opus" {
		t.Errorf("Model = %q, want %q", active.Model, "opus")
	}
}

func TestUpdateSessionEffort(t *testing.T) {
	db := openTestDB(t)

	now := time.Now()
	session := &ChatSession{ID: "sess-5", UserID: 333, ChatID: 1, Model: "sonnet", Effort: "high", WorkingDir: "/tmp", Active: true, CreatedAt: now, UpdatedAt: now}
	if err := db.CreateSession(session); err != nil {
		t.Fatal(err)
	}

	if err := db.UpdateSessionEffort("sess-5", "max"); err != nil {
		t.Fatalf("UpdateSessionEffort: %v", err)
	}

	active, _ := db.GetActiveSession(333)
	if active.Effort != "max" {
		t.Errorf("Effort = %q, want %q", active.Effort, "max")
	}
}

func TestAddChatLog_GetChatLogs(t *testing.T) {
	db := openTestDB(t)

	now := time.Now()
	session := &ChatSession{ID: "chat-sess", UserID: 444, ChatID: 1, Model: "sonnet", Effort: "high", WorkingDir: "/tmp", Active: true, CreatedAt: now, UpdatedAt: now}
	if err := db.CreateSession(session); err != nil {
		t.Fatal(err)
	}

	if err := db.AddChatLog("chat-sess", "user", "hello", 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := db.AddChatLog("chat-sess", "assistant", "hi there", 0.01, 100); err != nil {
		t.Fatal(err)
	}

	msgs, err := db.GetChatLogs("chat-sess", 10)
	if err != nil {
		t.Fatalf("GetChatLogs: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("got %d messages, want 2", len(msgs))
	}
}

func TestDeactivateStaleSessions(t *testing.T) {
	db := openTestDB(t)

	old := time.Now().Add(-2 * time.Hour)
	session := &ChatSession{ID: "stale-sess", UserID: 555, ChatID: 1, Model: "sonnet", Effort: "high", WorkingDir: "/tmp", Active: true, CreatedAt: old, UpdatedAt: old}
	if err := db.CreateSession(session); err != nil {
		t.Fatal(err)
	}

	n, err := db.DeactivateStaleSessions(1 * time.Hour)
	if err != nil {
		t.Fatalf("DeactivateStaleSessions: %v", err)
	}
	if n != 1 {
		t.Errorf("deactivated %d, want 1", n)
	}

	active, _ := db.GetActiveSession(555)
	if active != nil {
		t.Error("expected nil session after stale deactivation")
	}
}

func TestGetActiveSession_NoSession(t *testing.T) {
	db := openTestDB(t)

	session, err := db.GetActiveSession(99999)
	if err != nil {
		t.Fatalf("GetActiveSession: %v", err)
	}
	if session != nil {
		t.Error("expected nil session for nonexistent user")
	}
}

func TestGetLogsByJobName_Empty(t *testing.T) {
	db := openTestDB(t)

	logs, err := db.GetLogsByJobName("nonexistent", 10)
	if err != nil {
		t.Fatalf("GetLogsByJobName: %v", err)
	}
	if logs != nil {
		t.Errorf("expected nil slice for empty result, got %d items", len(logs))
	}
}
