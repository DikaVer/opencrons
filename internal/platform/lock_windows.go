//go:build windows

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
