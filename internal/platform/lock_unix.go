//go:build !windows

package platform

import (
	"os"
	"syscall"
)

// isProcessRunning checks if a process with the given PID is running.
// On Unix, FindProcess always succeeds; we send signal 0 to probe.
func isProcessRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
