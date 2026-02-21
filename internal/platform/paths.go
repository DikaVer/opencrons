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

// SummaryDir returns the path to the summary directory (execution summaries).
func SummaryDir() string {
	return filepath.Join(BaseDir(), "summary")
}

// DataDir returns the path to the data directory (SQLite DB).
func DataDir() string {
	return filepath.Join(BaseDir(), "data")
}

// DBPath returns the full path to the SQLite database file.
func DBPath() string {
	return filepath.Join(DataDir(), "scheduler.db")
}

// PIDFile returns the path to the PID file.
func PIDFile() string {
	return filepath.Join(BaseDir(), "scheduler.pid")
}

// WorkspaceDir returns the path to the workspace directory (CLAUDE.md + .claude/).
func WorkspaceDir() string {
	return filepath.Join(BaseDir(), "workspace")
}

// EnsureDirs creates all required directories if they don't exist.
func EnsureDirs() error {
	dirs := []string{
		BaseDir(),
		SchedulesDir(),
		PromptsDir(),
		LogsDir(),
		SummaryDir(),
		DataDir(),
		WorkspaceDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}
