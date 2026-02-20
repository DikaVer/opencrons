//go:build windows

package platform

import (
	"os"
	"path/filepath"
)

func defaultBaseDir() string {
	if appdata := os.Getenv("APPDATA"); appdata != "" {
		return filepath.Join(appdata, "cli-scheduler")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "AppData", "Roaming", "cli-scheduler")
}
