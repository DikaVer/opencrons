// Package storage provides a SQLite persistence layer using modernc.org/sqlite
// (pure Go, no CGO). The DB type wraps sql.DB and opens the database with WAL
// journal mode and a 5-second busy timeout. Schema migrations run automatically
// on first open, creating the execution_logs, chat_sessions, and chat_messages
// tables with appropriate indexes. CRUD operations cover execution log recording
// and querying, chat session lifecycle management, and chat message logging.
package storage

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/DikaVer/opencrons/internal/logger"
	_ "modernc.org/sqlite"
)

var log = logger.New("storage")

// DB wraps the SQLite database connection.
type DB struct {
	conn *sql.DB
}

// Open opens or creates the SQLite database at the given path.
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path+"?_pragma=journal_mode(wal)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("migrating database: %w", err)
	}

	log.Debug("database opened", "path", path)
	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS execution_logs (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		job_id       TEXT NOT NULL,
		job_name     TEXT NOT NULL,
		started_at   DATETIME NOT NULL,
		finished_at  DATETIME,
		exit_code    INTEGER,
		stdout_path  TEXT DEFAULT '',
		stderr_path  TEXT DEFAULT '',
		cost_usd     REAL,
		tokens_used  INTEGER,
		status       TEXT NOT NULL DEFAULT 'running'
		             CHECK (status IN ('running','success','failed','timeout','cancelled')),
		trigger_type TEXT NOT NULL DEFAULT 'scheduled'
		             CHECK (trigger_type IN ('scheduled','manual')),
		error_msg    TEXT DEFAULT ''
	);

	CREATE INDEX IF NOT EXISTS idx_logs_job_id ON execution_logs(job_id);
	CREATE INDEX IF NOT EXISTS idx_logs_started_at ON execution_logs(started_at);
	`
	if _, err := db.conn.Exec(schema); err != nil {
		return err
	}

	// Migration: add token usage columns (idempotent)
	migrations := []string{
		"ALTER TABLE execution_logs ADD COLUMN input_tokens INTEGER",
		"ALTER TABLE execution_logs ADD COLUMN output_tokens INTEGER",
		"ALTER TABLE execution_logs ADD COLUMN cache_read_tokens INTEGER",
		"ALTER TABLE execution_logs ADD COLUMN cache_creation_tokens INTEGER",
	}
	for _, m := range migrations {
		// Ignore "duplicate column" errors — column already exists
		_, _ = db.conn.Exec(m)
	}

	// Chat sessions and messages tables
	chatSchema := `
	CREATE TABLE IF NOT EXISTS chat_sessions (
		id          TEXT PRIMARY KEY,
		user_id     INTEGER NOT NULL,
		chat_id     INTEGER NOT NULL,
		model       TEXT NOT NULL DEFAULT 'sonnet',
		effort      TEXT NOT NULL DEFAULT 'high',
		working_dir TEXT NOT NULL,
		active      BOOLEAN NOT NULL DEFAULT 1,
		created_at  DATETIME NOT NULL,
		updated_at  DATETIME NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_sessions_user_active ON chat_sessions(user_id, active);

	CREATE TABLE IF NOT EXISTS chat_messages (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL REFERENCES chat_sessions(id),
		role       TEXT NOT NULL CHECK (role IN ('user','assistant')),
		content    TEXT NOT NULL,
		cost_usd   REAL DEFAULT 0,
		tokens     INTEGER DEFAULT 0,
		created_at DATETIME NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_messages_session ON chat_messages(session_id);
	`
	if _, err := db.conn.Exec(chatSchema); err != nil {
		return err
	}

	log.Info("schema migration complete")
	return nil
}

// InsertLog creates a new execution log entry and returns its ID.
func (db *DB) InsertLog(entry *ExecutionLog) (int64, error) {
	result, err := db.conn.Exec(
		`INSERT INTO execution_logs (job_id, job_name, started_at, status, trigger_type)
		 VALUES (?, ?, ?, ?, ?)`,
		entry.JobID, entry.JobName, entry.StartedAt, entry.Status, entry.TriggerType,
	)
	if err != nil {
		return 0, fmt.Errorf("inserting log: %w", err)
	}
	return result.LastInsertId()
}

// UpdateLog updates an existing execution log entry with results and usage data.
func (db *DB) UpdateLog(id int64, finishedAt time.Time, exitCode int, stdoutPath, stderrPath string, costUSD float64, inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens int, status, errorMsg string) error {
	_, err := db.conn.Exec(
		`UPDATE execution_logs SET
			finished_at = ?, exit_code = ?, stdout_path = ?, stderr_path = ?,
			cost_usd = ?, input_tokens = ?, output_tokens = ?,
			cache_read_tokens = ?, cache_creation_tokens = ?,
			status = ?, error_msg = ?
		 WHERE id = ?`,
		finishedAt, exitCode, stdoutPath, stderrPath,
		costUSD, inputTokens, outputTokens,
		cacheReadTokens, cacheCreationTokens,
		status, errorMsg, id,
	)
	if err != nil {
		return fmt.Errorf("updating log: %w", err)
	}
	return nil
}

// GetLogsByJobName returns execution logs for a specific job, most recent first.
func (db *DB) GetLogsByJobName(jobName string, limit int) ([]ExecutionLog, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := db.conn.Query(
		`SELECT id, job_id, job_name, started_at, finished_at, exit_code,
		        stdout_path, stderr_path, cost_usd,
		        input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
		        status, trigger_type, error_msg
		 FROM execution_logs
		 WHERE job_name = ?
		 ORDER BY started_at DESC
		 LIMIT ?`,
		jobName, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("querying logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanLogs(rows)
}

// GetRecentLogs returns recent execution logs across all jobs.
func (db *DB) GetRecentLogs(limit int) ([]ExecutionLog, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := db.conn.Query(
		`SELECT id, job_id, job_name, started_at, finished_at, exit_code,
		        stdout_path, stderr_path, cost_usd,
		        input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
		        status, trigger_type, error_msg
		 FROM execution_logs
		 ORDER BY started_at DESC
		 LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("querying logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanLogs(rows)
}

// GetUsageByJobName returns aggregated usage stats for a specific job.
func (db *DB) GetUsageByJobName(jobName string) (*UsageSummary, error) {
	row := db.conn.QueryRow(
		`SELECT COUNT(*),
		        COALESCE(SUM(cost_usd), 0),
		        COALESCE(SUM(input_tokens), 0),
		        COALESCE(SUM(output_tokens), 0),
		        COALESCE(SUM(cache_read_tokens), 0),
		        COALESCE(SUM(cache_creation_tokens), 0)
		 FROM execution_logs
		 WHERE job_name = ? AND status != 'running'`,
		jobName,
	)

	var s UsageSummary
	err := row.Scan(&s.TotalRuns, &s.TotalCostUSD, &s.TotalInputTokens,
		&s.TotalOutputTokens, &s.TotalCacheRead, &s.TotalCacheCreation)
	if err != nil {
		return nil, fmt.Errorf("querying usage: %w", err)
	}
	return &s, nil
}

// GetTotalUsage returns aggregated usage stats across all jobs.
func (db *DB) GetTotalUsage() (*UsageSummary, error) {
	row := db.conn.QueryRow(
		`SELECT COUNT(*),
		        COALESCE(SUM(cost_usd), 0),
		        COALESCE(SUM(input_tokens), 0),
		        COALESCE(SUM(output_tokens), 0),
		        COALESCE(SUM(cache_read_tokens), 0),
		        COALESCE(SUM(cache_creation_tokens), 0)
		 FROM execution_logs
		 WHERE status != 'running'`,
	)

	var s UsageSummary
	err := row.Scan(&s.TotalRuns, &s.TotalCostUSD, &s.TotalInputTokens,
		&s.TotalOutputTokens, &s.TotalCacheRead, &s.TotalCacheCreation)
	if err != nil {
		return nil, fmt.Errorf("querying total usage: %w", err)
	}
	return &s, nil
}

// CreateSession inserts a new chat session.
func (db *DB) CreateSession(session *ChatSession) error {
	_, err := db.conn.Exec(
		`INSERT INTO chat_sessions (id, user_id, chat_id, model, effort, working_dir, active, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.UserID, session.ChatID, session.Model, session.Effort,
		session.WorkingDir, session.Active, session.CreatedAt, session.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("creating session: %w", err)
	}
	return nil
}

// GetActiveSession returns the active chat session for a user, or nil if none.
func (db *DB) GetActiveSession(userID int64) (*ChatSession, error) {
	row := db.conn.QueryRow(
		`SELECT id, user_id, chat_id, model, effort, working_dir, active, created_at, updated_at
		 FROM chat_sessions
		 WHERE user_id = ? AND active = 1
		 ORDER BY updated_at DESC
		 LIMIT 1`,
		userID,
	)

	var s ChatSession
	err := row.Scan(&s.ID, &s.UserID, &s.ChatID, &s.Model, &s.Effort,
		&s.WorkingDir, &s.Active, &s.CreatedAt, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying active session: %w", err)
	}
	return &s, nil
}

// DeactivateUserSessions marks all sessions for a user as inactive.
func (db *DB) DeactivateUserSessions(userID int64) error {
	_, err := db.conn.Exec(
		`UPDATE chat_sessions SET active = 0, updated_at = ? WHERE user_id = ? AND active = 1`,
		time.Now(), userID,
	)
	if err != nil {
		return fmt.Errorf("deactivating sessions: %w", err)
	}
	return nil
}

// TouchSession updates the session's updated_at timestamp.
func (db *DB) TouchSession(sessionID string) error {
	_, err := db.conn.Exec(
		`UPDATE chat_sessions SET updated_at = ? WHERE id = ?`,
		time.Now(), sessionID,
	)
	if err != nil {
		return fmt.Errorf("touching session: %w", err)
	}
	return nil
}

// UpdateSessionModel updates the model for a session.
func (db *DB) UpdateSessionModel(sessionID, model string) error {
	_, err := db.conn.Exec(
		`UPDATE chat_sessions SET model = ?, updated_at = ? WHERE id = ?`,
		model, time.Now(), sessionID,
	)
	if err != nil {
		return fmt.Errorf("updating session model: %w", err)
	}
	return nil
}

// UpdateSessionEffort updates the effort for a session.
func (db *DB) UpdateSessionEffort(sessionID, effort string) error {
	_, err := db.conn.Exec(
		`UPDATE chat_sessions SET effort = ?, updated_at = ? WHERE id = ?`,
		effort, time.Now(), sessionID,
	)
	if err != nil {
		return fmt.Errorf("updating session effort: %w", err)
	}
	return nil
}

// AddChatLog inserts a chat message log entry.
func (db *DB) AddChatLog(sessionID, role, content string, costUSD float64, tokens int) error {
	_, err := db.conn.Exec(
		`INSERT INTO chat_messages (session_id, role, content, cost_usd, tokens, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		sessionID, role, content, costUSD, tokens, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("adding chat log: %w", err)
	}
	return nil
}

// GetChatLogs returns chat messages for a session, ordered by creation time.
func (db *DB) GetChatLogs(sessionID string, limit int) ([]ChatMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.conn.Query(
		`SELECT id, session_id, role, content, cost_usd, tokens, created_at
		 FROM chat_messages
		 WHERE session_id = ?
		 ORDER BY created_at DESC
		 LIMIT ?`,
		sessionID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("querying chat logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var messages []ChatMessage
	for rows.Next() {
		var m ChatMessage
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.CostUSD, &m.Tokens, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning chat message: %w", err)
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

// DeactivateStaleSessions deactivates sessions idle for more than the given duration.
func (db *DB) DeactivateStaleSessions(maxIdle time.Duration) (int64, error) {
	cutoff := time.Now().Add(-maxIdle)
	result, err := db.conn.Exec(
		`UPDATE chat_sessions SET active = 0 WHERE active = 1 AND updated_at < ?`,
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("deactivating stale sessions: %w", err)
	}
	n, _ := result.RowsAffected()
	log.Debug("stale sessions deactivated", "count", n, "maxIdle", maxIdle)
	return n, nil
}

func scanLogs(rows *sql.Rows) ([]ExecutionLog, error) {
	var logs []ExecutionLog
	for rows.Next() {
		var entry ExecutionLog
		var finishedAt sql.NullTime
		var exitCode sql.NullInt64
		var costUSD sql.NullFloat64
		var inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens sql.NullInt64

		err := rows.Scan(
			&entry.ID, &entry.JobID, &entry.JobName, &entry.StartedAt, &finishedAt,
			&exitCode, &entry.StdoutPath, &entry.StderrPath, &costUSD,
			&inputTokens, &outputTokens, &cacheReadTokens, &cacheCreationTokens,
			&entry.Status, &entry.TriggerType, &entry.ErrorMsg,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning log row: %w", err)
		}

		if finishedAt.Valid {
			entry.FinishedAt = &finishedAt.Time
		}
		if exitCode.Valid {
			code := int(exitCode.Int64)
			entry.ExitCode = &code
		}
		if costUSD.Valid {
			entry.CostUSD = &costUSD.Float64
		}
		if inputTokens.Valid {
			v := int(inputTokens.Int64)
			entry.InputTokens = &v
		}
		if outputTokens.Valid {
			v := int(outputTokens.Int64)
			entry.OutputTokens = &v
		}
		if cacheReadTokens.Valid {
			v := int(cacheReadTokens.Int64)
			entry.CacheReadTokens = &v
		}
		if cacheCreationTokens.Valid {
			v := int(cacheCreationTokens.Int64)
			entry.CacheCreationTokens = &v
		}

		logs = append(logs, entry)
	}
	return logs, rows.Err()
}
