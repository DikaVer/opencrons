package logger

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// testLogDir and testLogPath are set once in TestMain and shared by all tests.
// Because Init uses sync.Once, the entire test binary shares a single log file.
var (
	testLogDir  string
	testLogPath string
)

// TestMain creates a fresh temp directory, resets the package-level sync.Once
// so that Init can fire in test context, calls Init(dir, false), runs all
// tests, then closes and removes the temp directory.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "opencrons-logger-test-*")
	if err != nil {
		panic("failed to create temp dir: " + err.Error())
	}
	testLogDir = dir
	testLogPath = filepath.Join(dir, "opencrons.log")

	// Reset once so Init fires normally during tests. This is valid because the
	// test binary is in the same package (package logger).
	once = sync.Once{}
	Init(dir, false)

	code := m.Run()

	Close()
	os.RemoveAll(dir)
	os.Exit(code)
}

// readLog returns the full contents of the shared log file.
// It syncs the underlying file first to ensure all buffered writes are visible.
func readLog(t *testing.T) string {
	t.Helper()
	if logFile != nil {
		logFile.Sync()
	}
	data, err := os.ReadFile(testLogPath)
	if err != nil {
		t.Fatalf("readLog: os.ReadFile: %v", err)
	}
	return string(data)
}

// TestInit_CreatesLogFile verifies that Init created opencrons.log inside the
// directory that was passed to it.
func TestInit_CreatesLogFile(t *testing.T) {
	if _, err := os.Stat(testLogPath); os.IsNotExist(err) {
		t.Fatalf("expected %q to exist after Init, but it does not", testLogPath)
	}
}

// TestNew_ReturnsTaggedLogger verifies that every line produced by a logger
// returned from New carries the "component=<name>" attribute.
func TestNew_ReturnsTaggedLogger(t *testing.T) {
	lg := New("test")
	lg.Info("tagged-message-for-component-check")

	content := readLog(t)
	if !strings.Contains(content, "component=test") {
		t.Errorf("log output should contain %q\ngot:\n%s", "component=test", content)
	}
	if !strings.Contains(content, "tagged-message-for-component-check") {
		t.Errorf("log output should contain the message text\ngot:\n%s", content)
	}
}

// TestSetDebug_TogglesLevel verifies the runtime level switch:
//   - a Debug line written while debug=false must NOT appear in the file,
//   - a Debug line written after SetDebug(true) MUST appear.
//
// The test restores debug=false when done so subsequent tests see the same
// baseline that TestMain established.
func TestSetDebug_TogglesLevel(t *testing.T) {
	lg := New("toggle-debug")

	// Ensure we start from the known baseline established by TestMain.
	SetDebug(false)
	lg.Debug("toggle-debug-sentinel-before")

	// Nothing written while debug is off should reach the file.
	SetDebug(true)
	lg.Debug("toggle-debug-sentinel-after")

	// Restore for subsequent tests.
	SetDebug(false)

	content := readLog(t)
	if strings.Contains(content, "toggle-debug-sentinel-before") {
		t.Errorf("debug message written while debug=false must not appear in log\ngot:\n%s", content)
	}
	if !strings.Contains(content, "toggle-debug-sentinel-after") {
		t.Errorf("debug message written while debug=true must appear in log\ngot:\n%s", content)
	}
}

// TestLevels_InfoAlwaysLogged verifies that Info messages are persisted
// regardless of the debug flag.
func TestLevels_InfoAlwaysLogged(t *testing.T) {
	SetDebug(false)
	lg := New("level-info")
	lg.Info("info-level-always-visible")

	content := readLog(t)
	if !strings.Contains(content, "info-level-always-visible") {
		t.Errorf("Info message should always be logged, not found in:\n%s", content)
	}
}

// TestLevels_WarnAlwaysLogged verifies that Warn messages are persisted
// regardless of the debug flag.
func TestLevels_WarnAlwaysLogged(t *testing.T) {
	SetDebug(false)
	lg := New("level-warn")
	lg.Warn("warn-level-always-visible")

	content := readLog(t)
	if !strings.Contains(content, "warn-level-always-visible") {
		t.Errorf("Warn message should always be logged, not found in:\n%s", content)
	}
}

// TestLevels_ErrorAlwaysLogged verifies that Error messages are persisted
// regardless of the debug flag.
func TestLevels_ErrorAlwaysLogged(t *testing.T) {
	SetDebug(false)
	lg := New("level-error")
	lg.Error("error-level-always-visible")

	content := readLog(t)
	if !strings.Contains(content, "error-level-always-visible") {
		t.Errorf("Error message should always be logged, not found in:\n%s", content)
	}
}

// TestLevels_DebugGatedOff verifies that a Debug message written while the
// level is Info (debug=false) does not reach the log file.
func TestLevels_DebugGatedOff(t *testing.T) {
	SetDebug(false)
	lg := New("level-debug-gated")
	lg.Debug("debug-gated-off-sentinel")

	content := readLog(t)
	if strings.Contains(content, "debug-gated-off-sentinel") {
		t.Errorf("Debug message written while debug=false must not appear in log\ngot:\n%s", content)
	}
}
