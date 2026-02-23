// claude.go builds the claude -p command for job execution. It embeds
// task-preamble.txt and summary-prompt.txt via //go:embed, constructs the
// full prompt (preamble + user prompt + optional summary injection), and
// assembles CLI flags (--model, --permission-mode bypassPermissions,
// --output-format json, --effort, --no-session-persistence). The prompt
// is passed via stdin to avoid OS argument length limits and process list
// exposure. BuildCommand returns an exec.Cmd ready to run, and BuildResult
// holds the combined prompt text along with the constructed command.
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

	"github.com/DikaVer/opencrons/internal/config"
	"github.com/DikaVer/opencrons/internal/platform"
)

//go:embed summary-prompt.txt
var summaryPromptTemplate string

//go:embed task-preamble.txt
var taskPreamble string

// BuildResult holds the constructed command and any metadata from the build process.
type BuildResult struct {
	Cmd         *exec.Cmd
	SummaryPath string // non-empty if summary_enabled
}

// BuildCommand constructs the `claude -p` command for a job.
// workingDir is the resolved working directory (may differ from job.WorkingDir
// when the job uses the managed project folder).
func BuildCommand(ctx context.Context, job *config.JobConfig, workingDir string) (*BuildResult, error) {
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

	// Disallowed tools
	if len(job.DisallowedTools) > 0 {
		args = append(args, "--disallowedTools")
		args = append(args, job.DisallowedTools...)
	}

	// Read prompt from file
	promptPath := filepath.Join(platform.PromptsDir(), job.PromptFile)
	log.Debug("reading prompt file", "path", promptPath)
	promptContent, err := os.ReadFile(promptPath)
	if err != nil {
		return nil, fmt.Errorf("reading prompt file %s: %w", promptPath, err)
	}

	// Build final prompt: preamble + user prompt + optional summary injection
	prompt := taskPreamble + string(promptContent)

	var summaryPath string

	// Append summary injection if enabled
	if job.SummaryEnabled {
		now := time.Now()
		summaryPath = filepath.Join(platform.SummaryDir(), fmt.Sprintf("%s-%s.md", job.Name, now.Format("20060102-150405")))
		injection := strings.NewReplacer(
			"{{SUMMARY_PATH}}", summaryPath,
			"{{JOB_NAME}}", job.Name,
			"{{DATE}}", now.Format("2006-01-02 15:04"),
		).Replace(summaryPromptTemplate)
		prompt += injection
		log.Debug("summary injection applied", "job", job.Name, "summaryPath", summaryPath)
	}

	// Pass prompt via stdin to avoid exposing it in process list
	// and to avoid OS argument length limits
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Dir = workingDir

	log.Debug("built command", "args", strings.Join(args, " "), "dir", workingDir)

	return &BuildResult{Cmd: cmd, SummaryPath: summaryPath}, nil
}
