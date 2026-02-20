package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

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
		conn.Close()
		return nil, fmt.Errorf("migrating database: %w", err)
	}

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
		db.conn.Exec(m)
	}

	return nil
}

// InsertLog creates a new execution log entry and returns its ID.
func (db *DB) InsertLog(log *ExecutionLog) (int64, error) {
	result, err := db.conn.Exec(
		`INSERT INTO execution_logs (job_id, job_name, started_at, status, trigger_type)
		 VALUES (?, ?, ?, ?, ?)`,
		log.JobID, log.JobName, log.StartedAt, log.Status, log.TriggerType,
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
	defer rows.Close()

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
	defer rows.Close()

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

func scanLogs(rows *sql.Rows) ([]ExecutionLog, error) {
	var logs []ExecutionLog
	for rows.Next() {
		var log ExecutionLog
		var finishedAt sql.NullTime
		var exitCode sql.NullInt64
		var costUSD sql.NullFloat64
		var inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens sql.NullInt64

		err := rows.Scan(
			&log.ID, &log.JobID, &log.JobName, &log.StartedAt, &finishedAt,
			&exitCode, &log.StdoutPath, &log.StderrPath, &costUSD,
			&inputTokens, &outputTokens, &cacheReadTokens, &cacheCreationTokens,
			&log.Status, &log.TriggerType, &log.ErrorMsg,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning log row: %w", err)
		}

		if finishedAt.Valid {
			log.FinishedAt = &finishedAt.Time
		}
		if exitCode.Valid {
			code := int(exitCode.Int64)
			log.ExitCode = &code
		}
		if costUSD.Valid {
			log.CostUSD = &costUSD.Float64
		}
		if inputTokens.Valid {
			v := int(inputTokens.Int64)
			log.InputTokens = &v
		}
		if outputTokens.Valid {
			v := int(outputTokens.Int64)
			log.OutputTokens = &v
		}
		if cacheReadTokens.Valid {
			v := int(cacheReadTokens.Int64)
			log.CacheReadTokens = &v
		}
		if cacheCreationTokens.Valid {
			v := int(cacheCreationTokens.Int64)
			log.CacheCreationTokens = &v
		}

		logs = append(logs, log)
	}
	return logs, rows.Err()
}
