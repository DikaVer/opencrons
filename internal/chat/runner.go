package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/dika-maulidal/cli-scheduler/internal/logger"
	"github.com/dika-maulidal/cli-scheduler/internal/storage"
)

// chatOutput represents the JSON output from claude -p --output-format json.
type chatOutput struct {
	Result       string  `json:"result"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// ChatResult holds the output of a chat interaction.
type ChatResult struct {
	Response string
	CostUSD  float64
	Tokens   int
	Duration time.Duration
}

// Runner executes claude -p commands for chat sessions.
type Runner struct{}

// NewRunner creates a new chat runner.
func NewRunner() *Runner {
	return &Runner{}
}

// Run sends a message to Claude in the context of a session and returns the response.
func (r *Runner) Run(ctx context.Context, session *storage.ChatSession, message string) (*ChatResult, error) {
	// Apply timeout (120s for chat)
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	args := []string{
		"-p",
		"--session-id", session.ID,
		"--model", session.Model,
		"--output-format", "json",
		"--permission-mode", "bypassPermissions",
	}

	// Effort: only pass if not the default "high"
	if session.Effort != "" && session.Effort != "high" {
		args = append(args, "--effort", session.Effort)
	}

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Stdin = strings.NewReader(message)
	cmd.Dir = session.WorkingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	logger.Debug("Chat runner: claude %s (dir=%s)", strings.Join(args, " "), session.WorkingDir)

	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime)

	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("response timed out after 120s")
	}

	if err != nil {
		stderrStr := stderr.String()
		if stderrStr != "" {
			return nil, fmt.Errorf("claude error: %s", strings.TrimSpace(stderrStr))
		}
		return nil, fmt.Errorf("claude error: %w", err)
	}

	// Parse JSON output
	result := &ChatResult{Duration: duration}

	var output chatOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		// If JSON parsing fails, return raw output as the response
		result.Response = strings.TrimSpace(stdout.String())
		return result, nil
	}

	result.Response = output.Result
	result.CostUSD = output.TotalCostUSD
	result.Tokens = output.Usage.InputTokens + output.Usage.OutputTokens

	if result.Response == "" {
		result.Response = "(empty response)"
	}

	return result, nil
}
