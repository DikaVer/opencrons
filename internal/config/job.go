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

	"github.com/DikaVer/opencrons/internal/ui"
)

// Retry backoff strategy constants. BackoffExponential is the empty string
// (the default) so it is omitted from YAML for cleanliness.
const (
	BackoffExponential = ""       // default; stored as empty string in YAML
	BackoffLinear      = "linear"
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
	Container        string   `yaml:"container,omitempty"`        // "" (host), "docker", or "podman"
	ContainerImage   string   `yaml:"container_image,omitempty"`  // image to use when container is set
	DisallowedTools  []string `yaml:"disallowed_tools,omitempty"`
	SummaryEnabled   bool     `yaml:"summary_enabled,omitempty"`
	NoSessionPersist bool     `yaml:"no_session_persistence,omitempty"`
	MaxRetries       int      `yaml:"max_retries,omitempty"`   // 0 = no retries (default)
	RetryBackoff     string   `yaml:"retry_backoff,omitempty"` // "" (exponential, default) or "linear"
	Enabled          bool     `yaml:"enabled"`
	// OnSuccess lists job names to run automatically when this job completes successfully.
	OnSuccess []string `yaml:"on_success,omitempty"`
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
	// Validate name (alphanumeric, hyphens, underscores)
	if err := ui.ValidateJobName(j.Name); err != nil {
		return fmt.Errorf("job name %q: %w", j.Name, err)
	}

	// Validate cron schedule
	if _, err := ui.CronParser.Parse(j.Schedule); err != nil {
		return fmt.Errorf("job %q: invalid cron schedule %q: %w", j.Name, j.Schedule, err)
	}

	// Validate working directory exists (only when explicitly set; empty = project folder)
	if j.WorkingDir != "" {
		if info, err := os.Stat(j.WorkingDir); err != nil {
			return fmt.Errorf("job %q: working_dir %q does not exist: %w", j.Name, j.WorkingDir, err)
		} else if !info.IsDir() {
			return fmt.Errorf("job %q: working_dir %q is not a directory", j.Name, j.WorkingDir)
		}
	}

	// Validate model if specified
	if j.Model != "" {
		if !ui.ValidModels[j.Model] {
			return fmt.Errorf("job %q: unknown model %q", j.Name, j.Model)
		}
	}

	// Validate effort if specified
	if j.Effort != "" {
		if !ui.ValidEfforts[j.Effort] {
			return fmt.Errorf("job %q: unknown effort %q (valid: low, medium, high, max)", j.Name, j.Effort)
		}
	}

	// Validate container runtime
	if j.Container != "" {
		if !ui.ValidContainers[j.Container] {
			return fmt.Errorf("job %q: unknown container %q (valid: docker, podman)", j.Name, j.Container)
		}
	}

	// Validate container image requires container runtime
	if j.ContainerImage != "" && j.Container == "" {
		return fmt.Errorf("job %q: container_image requires container to be set (docker or podman)", j.Name)
	}

	// Validate timeout
	if j.Timeout < 0 {
		return fmt.Errorf("job %q: timeout cannot be negative", j.Name)
	}

	// Validate retry settings
	if j.MaxRetries < 0 || j.MaxRetries > 10 {
		return fmt.Errorf("job %q: max_retries must be between 0 and 10", j.Name)
	}
	// BackoffExponential ("") is the canonical default; omitted from YAML for cleanliness.
	if j.RetryBackoff != BackoffExponential && j.RetryBackoff != BackoffLinear {
		return fmt.Errorf("job %q: retry_backoff must be \"exponential\" or \"linear\"", j.Name)
	}

	// Validate on_success job names
	for _, name := range j.OnSuccess {
		if name == j.Name {
			return fmt.Errorf("job %q: on_success cannot reference itself", j.Name)
		}
		if err := ui.ValidateJobName(name); err != nil {
			return fmt.Errorf("job %q: on_success entry %q: %w", j.Name, name, err)
		}
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
