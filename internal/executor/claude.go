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

	// Permission mode
	if job.PermissionMode != "" {
		args = append(args, "--permission-mode", job.PermissionMode)
	}

	// Output format: always JSON for structured, parseable output
	args = append(args, "--output-format", "json")

	// Max budget
	if job.MaxBudgetUSD > 0 {
		args = append(args, "--max-budget-usd", fmt.Sprintf("%.2f", job.MaxBudgetUSD))
	}

	// Additional directories
	for _, dir := range job.AddDirs {
		args = append(args, "--add-dir", dir)
	}

	// MCP config
	if job.MCPConfig != "" {
		args = append(args, "--mcp-config", job.MCPConfig)
	}

	// Max turns
	if job.MaxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", job.MaxTurns))
	}

	// No session persistence
	if job.NoSessionPersistence {
		args = append(args, "--no-session-persistence")
	}

	// Context & memory control: --setting-sources
	if job.DisableProjectMemory || job.DisableUserMemory || job.DisableLocalMemory {
		var sources []string
		if !job.DisableProjectMemory {
			sources = append(sources, "project")
		}
		if !job.DisableUserMemory {
			sources = append(sources, "user")
		}
		if !job.DisableLocalMemory {
			sources = append(sources, "local")
		}
		if len(sources) > 0 {
			args = append(args, "--setting-sources", strings.Join(sources, ","))
		} else {
			// All disabled: pass empty to load no settings
			args = append(args, "--setting-sources", "")
		}
	}

	// Disable skills/slash commands
	if job.DisableSkills {
		args = append(args, "--disable-slash-commands")
	}

	// Read prompt from file and pass as positional argument
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

	// Environment variables
	var envExtras []string
	if job.DisableAutoMemory {
		envExtras = append(envExtras, "CLAUDE_CODE_DISABLE_AUTO_MEMORY=1")
	}
	if job.Effort != "" {
		envExtras = append(envExtras, "CLAUDE_CODE_EFFORT_LEVEL="+job.Effort)
	}
	if len(envExtras) > 0 {
		cmd.Env = append(os.Environ(), envExtras...)
	}

	return cmd, nil
}
