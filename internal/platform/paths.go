// Package platform provides cross-platform directory path resolution for the
// OpenCron's runtime configuration layout.
//
// BaseDir returns the root configuration directory (with a test override mechanism).
// Derived paths include SchedulesDir, PromptsDir, LogsDir, DataDir,
// DBPath, PIDFile, AgentsDir, AgentsFile, SkillsDir, ProjectsDir, and ProjectDir.
// EnsureDirs creates all required directories on first run. The actual default
// base directory is resolved by platform-specific implementations of defaultBaseDir.
package platform

import (
	"os"
	"path/filepath"
)

var baseDirOverride string

// SetBaseDir overrides the default base directory (for testing).
func SetBaseDir(dir string) {
	baseDirOverride = dir
}

// BaseDir returns the root configuration directory.
func BaseDir() string {
	if baseDirOverride != "" {
		return baseDirOverride
	}
	return defaultBaseDir()
}

// SchedulesDir returns the path to the schedules directory.
func SchedulesDir() string {
	return filepath.Join(BaseDir(), "schedules")
}

// PromptsDir returns the path to the prompts directory.
func PromptsDir() string {
	return filepath.Join(BaseDir(), "prompts")
}

// LogsDir returns the path to the logs directory (stdout/stderr capture).
func LogsDir() string {
	return filepath.Join(BaseDir(), "logs")
}

// DataDir returns the path to the data directory (SQLite DB).
func DataDir() string {
	return filepath.Join(BaseDir(), "data")
}

// DBPath returns the full path to the SQLite database file.
func DBPath() string {
	return filepath.Join(DataDir(), "opencrons.db")
}

// PIDFile returns the path to the PID file.
func PIDFile() string {
	return filepath.Join(BaseDir(), "opencrons.pid")
}

// AgentsDir returns the path to the canonical .agents/ directory.
func AgentsDir() string {
	return filepath.Join(BaseDir(), ".agents")
}

// AgentsFile returns the path to the canonical AGENTS.md file.
func AgentsFile() string {
	return filepath.Join(BaseDir(), "AGENTS.md")
}

// SkillsDir returns the path to the skills directory inside .agents/.
func SkillsDir() string {
	return filepath.Join(AgentsDir(), "skills")
}

// ProjectsDir returns the path to the projects directory (per-job workspace data).
// Reserved for future use — not yet created by EnsureDirs.
func ProjectsDir() string {
	return filepath.Join(BaseDir(), "projects")
}

// ProjectDir returns the path to a specific job's project directory.
// Reserved for future use — not yet created by EnsureDirs.
func ProjectDir(jobName string) string {
	return filepath.Join(ProjectsDir(), jobName)
}

// EnsureDirs creates all required directories if they don't exist.
// It also runs the V1 migration and creates provider-specific symlinks.
func EnsureDirs() error {
	// Migrate old workspace/ layout before creating new dirs
	if err := MigrateFromV1Layout(); err != nil {
		return err
	}

	dirs := []string{
		BaseDir(),
		SchedulesDir(),
		PromptsDir(),
		LogsDir(),
		DataDir(),
		AgentsDir(),
		SkillsDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	// Create provider-specific symlinks (.claude/ → .agents/, CLAUDE.md → AGENTS.md)
	if err := EnsureProviderSymlinks(); err != nil {
		return err
	}

	return nil
}
