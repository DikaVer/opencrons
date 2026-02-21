//go:build !windows

package platform

import "os"

// createSymlink creates a symbolic link at linkPath pointing to target.
func createSymlink(target, linkPath string) error {
	return os.Symlink(target, linkPath)
}
