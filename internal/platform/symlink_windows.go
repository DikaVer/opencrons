//go:build windows

package platform

import (
	"os"
	"os/exec"
)

// createSymlink creates a symbolic link at linkPath pointing to target.
// On Windows, os.Symlink requires developer mode or admin privileges.
// Falls back to junctions (mklink /J) for directories and hard links for files.
func createSymlink(target, linkPath string) error {
	// Try os.Symlink first (works when developer mode is enabled)
	err := os.Symlink(target, linkPath)
	if err == nil {
		return nil
	}

	// Determine if target is a directory or file
	info, statErr := os.Stat(target)
	if statErr != nil {
		return err // return original symlink error if we can't stat target
	}

	if info.IsDir() {
		// Fallback: use mklink /J for directory junctions
		cmd := exec.Command("cmd", "/C", "mklink", "/J", linkPath, target)
		if jErr := cmd.Run(); jErr != nil {
			return err // return original symlink error
		}
		return nil
	}

	// Fallback: use os.Link (hard link) for files
	if lErr := os.Link(target, linkPath); lErr != nil {
		return err // return original symlink error
	}
	return nil
}
