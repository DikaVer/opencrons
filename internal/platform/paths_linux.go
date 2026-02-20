//go:build !windows

package platform

import (
	"os"
	"path/filepath"
)

func defaultBaseDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "cli-scheduler")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cli-scheduler")
}
