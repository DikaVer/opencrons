// Package logger provides structured logging via log/slog.
//
// The logger is a leaf package with no internal dependencies. Call Init once
// at startup to open the log file and set the initial level. Use New to
// obtain a per-component *slog.Logger. Info/Warn/Error are always written
// to file; Debug is gated by SetDebug. The underlying slog.LevelVar allows
// atomic, lock-free level toggling at runtime.
package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

var (
	level   slog.LevelVar // default Info; toggled to Debug via SetDebug
	handler slog.Handler
	logFile *os.File
	once    sync.Once
)

// Init opens the log file in logDir and configures the global slog handler.
// It is safe to call multiple times — only the first call takes effect.
// When debug is true, the minimum level is set to Debug; otherwise Info.
func Init(logDir string, debug bool) {
	once.Do(func() {
		_ = os.MkdirAll(logDir, 0755)

		path := filepath.Join(logDir, "opencrons.log")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			// Fall back to stderr if we can't open the log file.
			handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: &level})
			setLevel(debug)
			slog.SetDefault(slog.New(handler))
			return
		}

		logFile = f
		handler = slog.NewTextHandler(f, &slog.HandlerOptions{Level: &level})
		setLevel(debug)
		slog.SetDefault(slog.New(handler))
	})
}

// New returns a *slog.Logger tagged with the given component name.
// If Init has not been called, returns a no-op logger that discards output.
func New(component string) *slog.Logger {
	if handler == nil {
		return slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return slog.New(handler).With("component", component)
}

// SetDebug changes the minimum log level at runtime.
// When enabled is true, Debug messages are written; otherwise only Info and above.
func SetDebug(enabled bool) {
	setLevel(enabled)
}

// Close flushes and closes the log file. Safe to call if Init was never called.
func Close() {
	if logFile != nil {
		logFile.Close()
	}
}

func setLevel(debug bool) {
	if debug {
		level.Set(slog.LevelDebug)
	} else {
		level.Set(slog.LevelInfo)
	}
}
