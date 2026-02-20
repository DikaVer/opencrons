package platform

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// WritePID writes the current process ID to the PID file.
func WritePID() error {
	pid := os.Getpid()
	return os.WriteFile(PIDFile(), []byte(strconv.Itoa(pid)), 0644)
}

// ReadPID reads the PID from the PID file.
func ReadPID() (int, error) {
	data, err := os.ReadFile(PIDFile())
	if err != nil {
		return 0, fmt.Errorf("reading PID file: %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parsing PID: %w", err)
	}

	return pid, nil
}

// RemovePID removes the PID file.
func RemovePID() error {
	return os.Remove(PIDFile())
}

// IsRunning checks if a process with the given PID is running.
func IsRunning(pid int) bool {
	return isProcessRunning(pid)
}

// CheckDaemonRunning returns the PID if the daemon is already running, 0 otherwise.
func CheckDaemonRunning() (int, bool) {
	pid, err := ReadPID()
	if err != nil {
		return 0, false
	}

	if IsRunning(pid) {
		return pid, true
	}

	// Stale PID file — clean up
	_ = RemovePID()
	return 0, false
}
