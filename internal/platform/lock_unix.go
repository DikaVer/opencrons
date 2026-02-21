//go:build !windows

// Package platform provides Unix process detection using a signal 0 probe.
//
// On Unix systems, os.FindProcess always succeeds regardless of whether the
// process exists. The actual liveness check is performed by sending signal 0
// via syscall.Signal(0), which returns nil only if the process is alive and
// the caller has permission to signal it.
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
