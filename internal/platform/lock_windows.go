//go:build windows

// Package platform provides Windows process detection using the Win32 API.
//
// Process liveness is checked by calling windows.OpenProcess with
// PROCESS_QUERY_LIMITED_INFORMATION access. If the process handle can be
// opened successfully, the process is considered running. The handle is
// closed immediately after the check.
package platform

import (
	"golang.org/x/sys/windows"
)

// isProcessRunning checks if a process with the given PID is running.
// On Windows, we attempt to open the process with limited query access.
func isProcessRunning(pid int) bool {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	windows.CloseHandle(handle)
	return true
}
