// File menu_daemon.go implements TUI daemon menu handlers including handleDebugMenu,
// handleDaemonMenu (start, stop, install service, view logs), and viewDaemonLogs
// which displays the last 50 lines of the debug log file.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DikaVer/opencrons/internal/daemon"
	"github.com/DikaVer/opencrons/internal/platform"
	"github.com/DikaVer/opencrons/internal/tui"
	"github.com/DikaVer/opencrons/internal/ui"
)

func handleDebugMenu() {
	current := platform.IsDebugEnabled()
	newState, err := tui.RunDebugMenu(current)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
		tui.PrintPressEnter()
		return
	}

	if newState != current {
		if err := platform.SetDebug(newState); err != nil {
			fmt.Fprintf(os.Stderr, "  Error saving settings: %v\n", err)
		} else if newState {
			fmt.Println("  Debug logging enabled.")
			fmt.Printf("  Logs: %s/opencrons-debug.log\n", platform.LogsDir())
		} else {
			fmt.Println("  Debug logging disabled.")
		}
	} else {
		fmt.Println("  No change.")
	}
	tui.PrintPressEnter()
}

func handleDaemonMenu() {
	choice, err := tui.RunDaemonMenu()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
		return
	}

	switch choice {
	case "start":
		fmt.Println("  Starting OpenCron daemon... (press Ctrl+C to stop)")
		fmt.Println()
		if err := daemon.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "  Daemon error: %v\n", err)
		}
	case "stop":
		runStop(nil, nil)
	case "install":
		if err := daemon.InstallService(); err != nil {
			fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
		}
		tui.PrintPressEnter()
	case "logs":
		viewDaemonLogs()
		tui.PrintPressEnter()
	case "back":
		return
	}
}

func viewDaemonLogs() {
	logFile := filepath.Join(platform.LogsDir(), "opencrons-debug.log")

	data, err := os.ReadFile(logFile)
	if err != nil {
		fmt.Println()
		fmt.Println(ui.Dim.Render("  No daemon logs available."))
		fmt.Println(ui.Dim.Render("  Enable debug logging in Settings > Debug to capture logs."))
		fmt.Printf("  Log path: %s\n", logFile)
		return
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")

	// Show last 50 lines
	start := 0
	if len(lines) > 50 {
		start = len(lines) - 50
		fmt.Printf("\n  %s\n\n", ui.Dim.Render(fmt.Sprintf("... showing last 50 of %d lines ...", len(lines))))
	} else {
		fmt.Println()
	}

	for _, line := range lines[start:] {
		if line != "" {
			fmt.Println("  " + line)
		}
	}
	fmt.Println()
}
