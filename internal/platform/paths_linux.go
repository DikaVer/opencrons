//go:build !windows

// Package platform provides the default base directory resolution for Linux
// and other Unix-like systems.
//
// The configuration directory is determined by XDG_CONFIG_HOME if set,
// otherwise it falls back to ~/.opencron.
package platform

import (
	"os"
	"path/filepath"
)

func defaultBaseDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "opencron")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".opencron")
}
