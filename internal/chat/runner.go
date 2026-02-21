// Package chat provides chat session execution and lifecycle management
// for interactive conversations with Claude Code via Telegram.
//
// Runner executes "claude -p" commands within a session context, piping
// user messages via stdin and parsing the JSON output into ChatResult
// values containing the response text, cost, and token usage.
//
// SessionManager (in session.go) maps Telegram users to persistent
// session UUIDs stored in SQLite, enabling multi-turn conversations
// through Claude Code's --session-id (new) and --resume (existing) flags.
package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/dika-maulidal/opencron/internal/logger"
	"github.com/dika-maulidal/opencron/internal/storage"
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
// When isNew is true the session is freshly created and --session-id is used to
// label it; otherwise --resume is used to load prior conversation history.
// The caller controls the lifetime via ctx — there is no internal timeout.
func (r *Runner) Run(ctx context.Context, session *storage.ChatSession, message string, isNew bool) (*ChatResult, error) {
	args := []string{"-p"}

	if isNew {
		args = append(args, "--session-id", session.ID)
	} else {
		args = append(args, "--resume", session.ID)
	}

	args = append(args,
		"--model", session.Model,
		"--output-format", "json",
		"--permission-mode", "bypassPermissions",
	)

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

	if ctx.Err() == context.Canceled {
		return nil, fmt.Errorf("query stopped")
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
