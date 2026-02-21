//go:build !windows

package daemon

import (
	"os/exec"
	"syscall"
)

// setSysProcAttr starts the child in a new session (Setsid), detaching it
// from the parent's controlling terminal so it survives the parent's exit.
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
}
