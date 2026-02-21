// Package config provides YAML file I/O for job configurations. It handles
// loading, saving, and deleting job schedule files and their associated prompt
// files. Functions include LoadJob, LoadAllJobs, SaveJob, SavePromptFile,
// DeleteJob, JobNameExists, and FindJobByName. File paths are normalized to
// handle Windows drive letter casing differences.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/DikaVer/opencrons/internal/logger"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

var log = logger.New("config")

// normalizeWorkingDir fixes bare Windows drive letters ("D:" → "D:\").
// On Windows, "D:" means "current directory on drive D", not the drive root.
// YAML round-tripping can strip the trailing backslash, so we restore it here.
func normalizeWorkingDir(dir string) string {
	if runtime.GOOS == "windows" && len(dir) == 2 && dir[1] == ':' &&
		((dir[0] >= 'A' && dir[0] <= 'Z') || (dir[0] >= 'a' && dir[0] <= 'z')) {
		return dir + `\`
	}
	return dir
}

// LoadJob reads a single YAML job config file.
func LoadJob(path string) (*JobConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading job config %s: %w", path, err)
	}

	var job JobConfig
	if err := yaml.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("parsing job config %s: %w", path, err)
	}

	job.WorkingDir = normalizeWorkingDir(job.WorkingDir)

	log.Info("job config loaded", "name", job.Name, "path", path)
	return &job, nil
}

// LoadAllJobs reads all YAML files in the schedules directory.
func LoadAllJobs(schedulesDir string) ([]*JobConfig, error) {
	log.Debug("loading all jobs", "dir", schedulesDir)
	entries, err := os.ReadDir(schedulesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading schedules directory: %w", err)
	}

	var jobs []*JobConfig
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			continue
		}

		job, err := LoadJob(filepath.Join(schedulesDir, name))
		if err != nil {
			log.Warn("skipping malformed config", "file", name, "err", err)
			continue
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// SaveJob writes a job config to a YAML file.
func SaveJob(schedulesDir string, job *JobConfig) error {
	if job.ID == "" {
		job.ID = uuid.New().String()[:8]
	}

	job.WorkingDir = normalizeWorkingDir(job.WorkingDir)

	data, err := yaml.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshaling job config: %w", err)
	}

	if err := os.MkdirAll(schedulesDir, 0755); err != nil {
		return fmt.Errorf("creating schedules directory: %w", err)
	}

	path := filepath.Join(schedulesDir, job.Name+".yml")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing job config: %w", err)
	}

	log.Info("job config saved", "name", job.Name, "path", path)
	return nil
}

// SavePromptFile writes prompt content to a file in the prompts directory.
func SavePromptFile(promptsDir, filename, content string) error {
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		return fmt.Errorf("creating prompts directory: %w", err)
	}

	path := filepath.Join(promptsDir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing prompt file: %w", err)
	}

	log.Info("prompt file saved", "file", filename)
	return nil
}

// validateJobName rejects names that could escape the target directory via path traversal.
func validateJobName(name string) error {
	if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return fmt.Errorf("invalid job name %q", name)
	}
	return nil
}

// DeleteJob removes a job's YAML config and optionally its prompt file.
func DeleteJob(schedulesDir, promptsDir, jobName string, deletePrompt bool) error {
	if err := validateJobName(jobName); err != nil {
		return err
	}

	// Load the job first to get the actual prompt file name
	var promptFile string
	if deletePrompt {
		job, err := FindJobByName(schedulesDir, jobName)
		if err == nil {
			promptFile = job.PromptFile
		} else {
			// Fallback to convention-based name
			promptFile = jobName + ".md"
		}
	}

	configPath := filepath.Join(schedulesDir, jobName+".yml")
	if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
		// Try .yaml extension
		configPath = filepath.Join(schedulesDir, jobName+".yaml")
		if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing job config: %w", err)
		}
	}

	if deletePrompt && promptFile != "" {
		promptPath := filepath.Join(promptsDir, promptFile)
		if err := os.Remove(promptPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing prompt file: %w", err)
		}
	}

	log.Info("job deleted", "name", jobName)
	return nil
}

// JobNameExists checks if a job with the given name already exists.
func JobNameExists(schedulesDir, name string) bool {
	if validateJobName(name) != nil {
		return false
	}
	path := filepath.Join(schedulesDir, name+".yml")
	if _, err := os.Stat(path); err == nil {
		return true
	}
	path = filepath.Join(schedulesDir, name+".yaml")
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}

// FindJobByName loads a specific job config by name.
func FindJobByName(schedulesDir, name string) (*JobConfig, error) {
	if err := validateJobName(name); err != nil {
		return nil, err
	}

	path := filepath.Join(schedulesDir, name+".yml")
	if _, err := os.Stat(path); err == nil {
		return LoadJob(path)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("checking job config %s: %w", path, err)
	}

	path = filepath.Join(schedulesDir, name+".yaml")
	if _, err := os.Stat(path); err == nil {
		return LoadJob(path)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("checking job config %s: %w", path, err)
	}

	return nil, fmt.Errorf("job %q not found", name)
}
