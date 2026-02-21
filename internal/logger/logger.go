package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/dika-maulidal/cli-scheduler/internal/platform"
)

var (
	instance *log.Logger
	once     sync.Once
	logFile  *os.File
)

// init initializes the logger lazily on first use.
func get() *log.Logger {
	once.Do(func() {
		logsDir := platform.LogsDir()
		_ = os.MkdirAll(logsDir, 0755)

		path := filepath.Join(logsDir, "scheduler-debug.log")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			// Fall back to stderr if we can't open the log file.
			instance = log.New(os.Stderr, "[scheduler] ", log.LstdFlags)
			return
		}

		logFile = f
		instance = log.New(f, "[scheduler] ", log.LstdFlags)
	})
	return instance
}

// Debug logs a message only when debug mode is enabled in settings.
func Debug(format string, args ...any) {
	if !platform.IsDebugEnabled() {
		return
	}
	get().Output(2, fmt.Sprintf("[DEBUG] "+format, args...))
}

// Info logs a message to the debug log file when debug mode is enabled.
// In the daemon context, messages are always logged to stdout via the daemon's own logger.
func Info(format string, args ...any) {
	if !platform.IsDebugEnabled() {
		return
	}
	get().Output(2, fmt.Sprintf("[INFO] "+format, args...))
}
