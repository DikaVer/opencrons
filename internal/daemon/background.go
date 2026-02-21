// background.go provides RunBackground, which re-spawns the current binary
// as a detached background process running "start --foreground". The parent
// returns immediately with the child's PID. Platform-specific process
// detachment (new session on Unix, DETACHED_PROCESS on Windows) is handled
// by setSysProcAttr defined in background_unix.go / background_windows.go.
package daemon

import (
	"fmt"
	"os"
	"os/exec"
)

// RunBackground re-launches the current binary with "start --foreground" as a
// detached child process and returns its PID. The caller should exit or return
// so the terminal is freed; the child continues running independently.
func RunBackground() (int, error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("resolving executable path: %w", err)
	}

	cmd := exec.Command(exe, "start", "--foreground")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	setSysProcAttr(cmd) // platform-specific: detach from terminal

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("spawning background process: %w", err)
	}

	// Release the child so it outlives the parent.
	if err := cmd.Process.Release(); err != nil {
		return 0, fmt.Errorf("releasing child process: %w", err)
	}

	return cmd.Process.Pid, nil
}
