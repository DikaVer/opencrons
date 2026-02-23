// executor.go orchestrates the full lifecycle of a scheduled job execution.
// Run validates the working directory, creates a database log entry, sets up
// stdout/stderr capture files, applies a context timeout, builds and runs the
// claude command, determines the result status (success, failed, or timeout),
// parses JSON usage data from stdout, and updates the database log with the
// outcome. It also defines the claudeOutput struct for JSON parsing, the
// Result struct returned to callers, and the parseUsage helper.
package executor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/DikaVer/opencrons/internal/config"
	"github.com/DikaVer/opencrons/internal/logger"
	"github.com/DikaVer/opencrons/internal/platform"
	"github.com/DikaVer/opencrons/internal/storage"
)

var log = logger.New("executor")

// effectiveWorkingDir returns the working directory to use for a job.
// When job.WorkingDir is empty the job uses its managed project folder
// (platform.ProjectDir(job.Name)), which is created lazily on first run.
func effectiveWorkingDir(job *config.JobConfig) (string, error) {
	if job.WorkingDir != "" {
		return job.WorkingDir, nil
	}
	dir := platform.ProjectDir(job.Name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating project directory for job %q: %w", job.Name, err)
	}
	return dir, nil
}

// claudeOutput represents the JSON output from `claude -p --output-format json`.
type claudeOutput struct {
	Result       string  `json:"result"`
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
	Output              string // claude's result text from JSON output
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
	workingDir, err := effectiveWorkingDir(job)
	if err != nil {
		return nil, err
	}

	log.Info("job started", "name", job.Name, "trigger", triggerType, "dir", workingDir)

	// Validate working directory still exists at runtime.
	// Managed project folders (empty WorkingDir) are created by effectiveWorkingDir above,
	// so only explicit directories need a presence check here.
	if job.WorkingDir != "" {
		if info, err := os.Stat(workingDir); err != nil {
			return nil, fmt.Errorf("job %q: working directory %q does not exist or is not accessible", job.Name, workingDir)
		} else if !info.IsDir() {
			return nil, fmt.Errorf("job %q: working directory %q is not a directory", job.Name, workingDir)
		}
	}

	now := time.Now()
	timestamp := now.Format("20060102-150405")

	// Create log entry
	entry := &storage.ExecutionLog{
		JobID:       job.ID,
		JobName:     job.Name,
		StartedAt:   now,
		Status:      "running",
		TriggerType: triggerType,
	}

	logID, err := db.InsertLog(entry)
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
		_ = stdoutFile.Close()
		return nil, fmt.Errorf("creating stderr file: %w", err)
	}

	// Apply timeout if configured
	if job.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(job.Timeout)*time.Second)
		defer cancel()
	}

	// Build command with context (supports cancellation and timeout)
	built, err := BuildCommand(ctx, job, workingDir)
	if err != nil {
		result := &Result{
			ExitCode: -1,
			Status:   "failed",
			ErrorMsg: err.Error(),
		}
		if updateErr := db.UpdateLog(logID, time.Now(), -1, stdoutPath, stderrPath, 0, 0, 0, 0, 0, "failed", err.Error()); updateErr != nil {
			log.Error("failed to update log", "logID", logID, "job", job.Name, "err", updateErr)
		}
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
	_ = stdoutFile.Close()
	_ = stderrFile.Close()

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
		log.Info("job timed out", "name", job.Name, "timeout", job.Timeout)
	} else if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
		result.Status = "failed"
		result.ErrorMsg = runErr.Error()
		log.Info("job command failed", "name", job.Name, "exit", result.ExitCode, "err", runErr)
	} else {
		result.ExitCode = 0
		result.Status = "success"
	}

	// Parse usage from JSON stdout
	parseUsage(stdoutPath, result)

	// Update log
	finishedAt := time.Now()
	if updateErr := db.UpdateLog(logID, finishedAt, result.ExitCode, stdoutPath, stderrPath,
		result.CostUSD, result.InputTokens, result.OutputTokens,
		result.CacheReadTokens, result.CacheCreationTokens,
		result.Status, result.ErrorMsg); updateErr != nil {
		log.Error("failed to finalize log", "logID", logID, "job", job.Name, "err", updateErr)
	}

	log.Info("job finished", "name", job.Name, "status", result.Status,
		"exit", result.ExitCode, "duration", result.Duration, "cost", result.CostUSD)

	return result, nil
}

// parseUsage reads the stdout JSON file and extracts usage data into result.
// Claude Code may output either a single JSON object or NDJSON (one event per
// line). We try the single-object form first, then fall back to scanning each
// line so that both formats are handled transparently.
func parseUsage(stdoutPath string, result *Result) {
	f, err := os.Open(stdoutPath)
	if err != nil {
		log.Debug("parseUsage: cannot open file", "path", stdoutPath, "err", err)
		return
	}
	defer func() { _ = f.Close() }()

	var output claudeOutput
	parsed := false

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1 MiB line buffer
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if len(lines) == 0 {
		log.Debug("parseUsage: stdout file is empty", "path", stdoutPath)
		return
	}

	// Single JSON object (legacy): try the whole file joined back together.
	raw := strings.Join(lines, "\n")
	if err := json.Unmarshal([]byte(raw), &output); err == nil {
		parsed = true
	}

	// NDJSON: scan from the last line backwards for a line that has a
	// non-empty "result" field (the final result event from Claude).
	if !parsed || output.Result == "" {
		for i := len(lines) - 1; i >= 0; i-- {
			line := strings.TrimSpace(lines[i])
			if line == "" {
				continue
			}
			var candidate claudeOutput
			if jsonErr := json.Unmarshal([]byte(line), &candidate); jsonErr == nil {
				// Prefer a line that has a non-empty result; fall back to
				// the first parseable line (for cost/token data).
				if candidate.Result != "" {
					output = candidate
					parsed = true
					break
				}
				if !parsed {
					output = candidate
					parsed = true
				}
			}
		}
	}

	if !parsed {
		log.Warn("parseUsage: no parseable JSON found", "path", stdoutPath)
		return
	}

	log.Debug("parseUsage: parsed", "cost", output.TotalCostUSD,
		"inputTokens", output.Usage.InputTokens, "outputTokens", output.Usage.OutputTokens)

	result.Output = output.Result
	result.CostUSD = output.TotalCostUSD
	result.InputTokens = output.Usage.InputTokens
	result.OutputTokens = output.Usage.OutputTokens
	result.CacheReadTokens = output.Usage.CacheReadInputTokens
	result.CacheCreationTokens = output.Usage.CacheCreationInputTokens
}
