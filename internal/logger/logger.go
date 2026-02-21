// logger.go provides a singleton debug logger gated by platform.IsDebugEnabled.
// Initialization is deferred via sync.Once so the logger is only created on
// first use. When debug is enabled, log output is written to
// logs/opencrons-debug.log inside the platform config directory; if the log
// file cannot be opened, it falls back to stderr. The exported Debug and Info
// functions are no-ops when debug logging is disabled, keeping overhead to zero
// in production.
package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/DikaVer/opencrons/internal/platform"
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

		path := filepath.Join(logsDir, "opencrons-debug.log")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			// Fall back to stderr if we can't open the log file.
			instance = log.New(os.Stderr, "[opencrons] ", log.LstdFlags)
			return
		}

		logFile = f
		instance = log.New(f, "[opencrons] ", log.LstdFlags)
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
