// kill.go provides cross-platform process enumeration and force termination
// for the opencrons daemon executable.
package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

// KillAllDaemonProcesses finds and forcefully kills all running copies of the
// opencrons executable, excluding the current process. It also removes the PID
// file if any processes were killed. Returns the list of killed PIDs.
func KillAllDaemonProcesses() ([]int, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("getting executable path: %w", err)
	}

	exeName := filepath.Base(exePath)
	selfPID := os.Getpid()

	killed, err := killAllByName(exeName, selfPID)
	if err != nil {
		return killed, err
	}

	if len(killed) > 0 {
		_ = RemovePID()
	}

	return killed, nil
}
