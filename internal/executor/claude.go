package executor

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dika-maulidal/cli-scheduler/internal/config"
	"github.com/dika-maulidal/cli-scheduler/internal/logger"
	"github.com/dika-maulidal/cli-scheduler/internal/platform"
)

//go:embed summary-prompt.txt
var summaryPromptTemplate string

//go:embed task-preamble.txt
var taskPreamble string

// BuildCommand constructs the `claude -p` command for a job.
func BuildCommand(ctx context.Context, job *config.JobConfig) (*exec.Cmd, error) {
	args := []string{"-p"}

	// Model
	if job.Model != "" {
		args = append(args, "--model", job.Model)
	}

	// Permission mode: always bypass for unattended scheduled execution
	args = append(args, "--permission-mode", "bypassPermissions")

	// Output format: always JSON for structured, parseable output
	args = append(args, "--output-format", "json")

	// No session persistence
	if job.NoSessionPersist {
		args = append(args, "--no-session-persistence")
	}

	// Effort level
	if job.Effort != "" {
		args = append(args, "--effort", job.Effort)
	}

	// Read prompt from file
	promptPath := filepath.Join(platform.PromptsDir(), job.PromptFile)
	promptContent, err := os.ReadFile(promptPath)
	if err != nil {
		return nil, fmt.Errorf("reading prompt file %s: %w", promptPath, err)
	}

	// Build final prompt: preamble + user prompt + optional summary injection
	prompt := taskPreamble + string(promptContent)

	// Append summary injection if enabled
	if job.SummaryEnabled {
		now := time.Now()
		summaryPath := filepath.Join(platform.SummaryDir(), fmt.Sprintf("%s-%s.md", job.Name, now.Format("2006-01-02")))
		injection := strings.NewReplacer(
			"{{SUMMARY_PATH}}", summaryPath,
			"{{JOB_NAME}}", job.Name,
			"{{DATE}}", now.Format("2006-01-02 15:04"),
		).Replace(summaryPromptTemplate)
		prompt += injection
	}

	// Pass prompt via stdin to avoid exposing it in process list
	// and to avoid OS argument length limits
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Dir = job.WorkingDir

	logger.Debug("Built command: claude %s (dir=%s)", strings.Join(args, " "), job.WorkingDir)

	return cmd, nil
}
