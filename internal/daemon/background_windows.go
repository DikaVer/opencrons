//go:build windows

package daemon

import (
	"os/exec"
	"syscall"
)

// setSysProcAttr detaches the child from the parent's console window on
// Windows. CREATE_NEW_PROCESS_GROUP prevents Ctrl+C propagation;
// CREATE_NO_WINDOW suppresses any console allocation for the child.
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x08000000, // 0x08000000 = CREATE_NO_WINDOW
	}
}
