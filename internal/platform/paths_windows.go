//go:build windows

// Package platform provides the default base directory resolution for Windows.
//
// The configuration directory is determined by the APPDATA environment variable
// (typically %APPDATA%\opencron). If APPDATA is not set, it falls back to
// ~/AppData/Roaming/opencron.
package platform

import (
	"os"
	"path/filepath"
)

func defaultBaseDir() string {
	if appdata := os.Getenv("APPDATA"); appdata != "" {
		return filepath.Join(appdata, "opencron")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "AppData", "Roaming", "opencron")
}
