// Package config defines the JobConfig struct and its YAML serialization for
// scheduled job configuration. It provides validation for all job fields
// including name format, cron schedule syntax, prompt file path security
// (rejecting traversal and absolute paths), working directory existence,
// model selection, effort level, and timeout bounds.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dika-maulidal/opencron/internal/ui"
)

// JobConfig represents a single scheduled job configuration.
type JobConfig struct {
	ID               string `yaml:"id"`
	Name             string `yaml:"name"`
	Schedule         string `yaml:"schedule"`
	WorkingDir       string `yaml:"working_dir"`
	PromptFile       string `yaml:"prompt_file"`
	Model            string `yaml:"model,omitempty"`
	Timeout          int    `yaml:"timeout,omitempty"`
	Effort           string   `yaml:"effort,omitempty"`
	DisallowedTools  []string `yaml:"disallowed_tools,omitempty"`
	SummaryEnabled   bool     `yaml:"summary_enabled,omitempty"`
	NoSessionPersist bool   `yaml:"no_session_persistence,omitempty"`
	Enabled          bool   `yaml:"enabled"`
}

// Validate checks that all required fields are present and valid.
func (j *JobConfig) Validate() error {
	if j.Name == "" {
		return fmt.Errorf("job name is required")
	}
	if j.Schedule == "" {
		return fmt.Errorf("job %q: schedule is required", j.Name)
	}
	if j.PromptFile == "" {
		return fmt.Errorf("job %q: prompt_file is required", j.Name)
	}
	if strings.Contains(j.PromptFile, "..") || filepath.IsAbs(j.PromptFile) {
		return fmt.Errorf("job %q: prompt_file must be a relative path without '..'", j.Name)
	}
	if j.WorkingDir == "" {
		return fmt.Errorf("job %q: working_dir is required", j.Name)
	}

	// Validate name (alphanumeric, hyphens, underscores)
	if err := ui.ValidateJobName(j.Name); err != nil {
		return fmt.Errorf("job name %q: %w", j.Name, err)
	}

	// Validate cron schedule
	if _, err := ui.CronParser.Parse(j.Schedule); err != nil {
		return fmt.Errorf("job %q: invalid cron schedule %q: %w", j.Name, j.Schedule, err)
	}

	// Validate working directory exists
	if info, err := os.Stat(j.WorkingDir); err != nil {
		return fmt.Errorf("job %q: working_dir %q does not exist: %w", j.Name, j.WorkingDir, err)
	} else if !info.IsDir() {
		return fmt.Errorf("job %q: working_dir %q is not a directory", j.Name, j.WorkingDir)
	}

	// Validate model if specified
	if j.Model != "" {
		validModels := map[string]bool{
			"sonnet": true, "opus": true, "haiku": true,
			"claude-sonnet-4-6": true, "claude-opus-4-6": true, "claude-haiku-4-5-20251001": true,
		}
		if !validModels[j.Model] {
			return fmt.Errorf("job %q: unknown model %q", j.Name, j.Model)
		}
	}

	// Validate effort if specified
	if j.Effort != "" {
		validEfforts := map[string]bool{
			"low": true, "medium": true, "high": true, "max": true,
		}
		if !validEfforts[j.Effort] {
			return fmt.Errorf("job %q: unknown effort %q (valid: low, medium, high, max)", j.Name, j.Effort)
		}
	}

	// Validate timeout
	if j.Timeout < 0 {
		return fmt.Errorf("job %q: timeout cannot be negative", j.Name)
	}

	return nil
}

// ValidatePromptFileExists checks that the prompt file exists at the given base dir.
func (j *JobConfig) ValidatePromptFileExists(promptsDir string) error {
	path := filepath.Join(promptsDir, j.PromptFile)
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("job %q: prompt file %q not found at %s", j.Name, j.PromptFile, path)
	}
	return nil
}
