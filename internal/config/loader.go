package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

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

	return &job, nil
}

// LoadAllJobs reads all YAML files in the schedules directory.
func LoadAllJobs(schedulesDir string) ([]*JobConfig, error) {
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
			log.Printf("Warning: skipping %s: %v", name, err)
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

	return nil
}

// DeleteJob removes a job's YAML config and optionally its prompt file.
func DeleteJob(schedulesDir, promptsDir, jobName string, deletePrompt bool) error {
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

	return nil
}

// JobNameExists checks if a job with the given name already exists.
func JobNameExists(schedulesDir, name string) bool {
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

// FindJobByName loads all jobs and returns the one matching the given name.
func FindJobByName(schedulesDir, name string) (*JobConfig, error) {
	jobs, err := LoadAllJobs(schedulesDir)
	if err != nil {
		return nil, err
	}

	for _, job := range jobs {
		if job.Name == name {
			return job, nil
		}
	}

	return nil, fmt.Errorf("job %q not found", name)
}
