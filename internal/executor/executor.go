// executor.go orchestrates the full lifecycle of a scheduled job execution.
// Run validates the working directory, creates a database log entry, sets up
// stdout/stderr capture files, applies a context timeout, builds and runs the
// claude command, determines the result status (success, failed, or timeout),
// parses JSON usage data from stdout, and updates the database log with the
// outcome. It also defines the claudeOutput struct for JSON parsing, the
// Result struct returned to callers, and the parseUsage helper.
package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/dika-maulidal/opencron/internal/config"
	"github.com/dika-maulidal/opencron/internal/logger"
	"github.com/dika-maulidal/opencron/internal/platform"
	"github.com/dika-maulidal/opencron/internal/storage"
)

// claudeOutput represents the JSON output from `claude -p --output-format json`.
type claudeOutput struct {
	TotalCostUSD float64 `json:"total_cost_usd"`
	Usage        struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	} `json:"usage"`
}

// Result captures the outcome of a job execution.
type Result struct {
	ExitCode            int
	StdoutPath          string
	StderrPath          string
	SummaryPath         string // path to summary file (if summary_enabled)
	Status              string
	ErrorMsg            string
	Duration            time.Duration
	CostUSD             float64
	InputTokens         int
	OutputTokens        int
	CacheReadTokens     int
	CacheCreationTokens int
}

// Run executes a job and logs the result to the database.
func Run(ctx context.Context, db *storage.DB, job *config.JobConfig, triggerType string) (*Result, error) {
	logger.Debug("Starting job %q (trigger=%s, dir=%s)", job.Name, triggerType, job.WorkingDir)

	// Validate working directory still exists at runtime
	if info, err := os.Stat(job.WorkingDir); err != nil {
		return nil, fmt.Errorf("job %q: working directory %q does not exist — was it deleted after job creation?", job.Name, job.WorkingDir)
	} else if !info.IsDir() {
		return nil, fmt.Errorf("job %q: working directory %q is not a directory", job.Name, job.WorkingDir)
	}

	now := time.Now()
	timestamp := now.Format("20060102-150405")

	// Create log entry
	log := &storage.ExecutionLog{
		JobID:       job.ID,
		JobName:     job.Name,
		StartedAt:   now,
		Status:      "running",
		TriggerType: triggerType,
	}

	logID, err := db.InsertLog(log)
	if err != nil {
		return nil, fmt.Errorf("creating execution log: %w", err)
	}

	// Set up output capture files
	if err := os.MkdirAll(platform.LogsDir(), 0755); err != nil {
		return nil, fmt.Errorf("creating logs directory: %w", err)
	}

	stdoutPath := filepath.Join(platform.LogsDir(), fmt.Sprintf("%s-%s-stdout.json", job.Name, timestamp))
	stderrPath := filepath.Join(platform.LogsDir(), fmt.Sprintf("%s-%s-stderr.log", job.Name, timestamp))

	stdoutFile, err := os.Create(stdoutPath)
	if err != nil {
		return nil, fmt.Errorf("creating stdout file: %w", err)
	}

	stderrFile, err := os.Create(stderrPath)
	if err != nil {
		stdoutFile.Close()
		return nil, fmt.Errorf("creating stderr file: %w", err)
	}

	// Apply timeout if configured
	if job.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(job.Timeout)*time.Second)
		defer cancel()
	}

	// Build command with context (supports cancellation and timeout)
	built, err := BuildCommand(ctx, job)
	if err != nil {
		result := &Result{
			ExitCode: -1,
			Status:   "failed",
			ErrorMsg: err.Error(),
		}
		_ = db.UpdateLog(logID, time.Now(), -1, stdoutPath, stderrPath, 0, 0, 0, 0, 0, "failed", err.Error())
		return result, nil
	}

	cmd := built.Cmd
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile

	// Execute
	startTime := time.Now()
	runErr := cmd.Run()
	duration := time.Since(startTime)

	// Close output files before reading them back
	stdoutFile.Close()
	stderrFile.Close()

	// Determine result
	result := &Result{
		StdoutPath: stdoutPath,
		StderrPath: stderrPath,
		Duration:   duration,
	}

	if ctx.Err() == context.DeadlineExceeded {
		result.ExitCode = -1
		result.Status = "timeout"
		result.ErrorMsg = fmt.Sprintf("job timed out after %ds", job.Timeout)
	} else if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
		result.Status = "failed"
		result.ErrorMsg = runErr.Error()
	} else {
		result.ExitCode = 0
		result.Status = "success"
	}

	// Set summary path if summary was enabled
	if built.SummaryPath != "" {
		result.SummaryPath = built.SummaryPath
	}

	// Parse usage from JSON stdout
	parseUsage(stdoutPath, result)

	// Update log
	finishedAt := time.Now()
	_ = db.UpdateLog(logID, finishedAt, result.ExitCode, stdoutPath, stderrPath,
		result.CostUSD, result.InputTokens, result.OutputTokens,
		result.CacheReadTokens, result.CacheCreationTokens,
		result.Status, result.ErrorMsg)

	logger.Debug("Job %q finished: status=%s exit=%d duration=%s cost=$%.4f",
		job.Name, result.Status, result.ExitCode, result.Duration, result.CostUSD)

	return result, nil
}

// parseUsage reads the stdout JSON file and extracts usage data into result.
func parseUsage(stdoutPath string, result *Result) {
	data, err := os.ReadFile(stdoutPath)
	if err != nil || len(data) == 0 {
		return
	}

	var output claudeOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return
	}

	result.CostUSD = output.TotalCostUSD
	result.InputTokens = output.Usage.InputTokens
	result.OutputTokens = output.Usage.OutputTokens
	result.CacheReadTokens = output.Usage.CacheReadInputTokens
	result.CacheCreationTokens = output.Usage.CacheCreationInputTokens
}
