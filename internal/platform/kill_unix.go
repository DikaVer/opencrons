//go:build !windows

// kill_unix.go implements process enumeration via pgrep and force termination
// via os.Process.Kill for Linux and macOS.
//
// Note: pgrep -x matches against the process comm name, which is limited to
// 15 characters on Linux. The binary name "opencrons" (9 chars) is within
// this limit, but renaming the binary to something longer would break matching
// on Linux.
package platform

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// killAllByName finds all processes whose name exactly matches exeName and
// kills them, skipping selfPID to avoid killing the current CLI process.
func killAllByName(exeName string, selfPID int) ([]int, error) {
	out, err := exec.Command("pgrep", "-x", exeName).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			// pgrep exit code 1 means no matching processes — not an error.
			return nil, nil
		}
		return nil, fmt.Errorf("pgrep: %w", err)
	}

	var killed []int
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err != nil || pid == selfPID {
			continue
		}
		// os.FindProcess always succeeds on Unix (no OS call, just wraps the PID).
		proc, _ := os.FindProcess(pid)
		if err := proc.Kill(); err == nil {
			killed = append(killed, pid)
		}
	}

	return killed, nil
}
