// File menu_daemon.go implements TUI daemon menu handlers including handleDebugMenu,
// handleDaemonMenu (start, stop, install service, view logs), and viewDaemonLogs
// which displays the last 50 lines of the debug log file.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
		if pid, running := platform.CheckDaemonRunning(); running {
			fmt.Fprintf(os.Stderr, "  Daemon already running (PID %d)\n", pid)
			tui.PrintPressEnter()
			return
		}
		pid, err := daemon.RunBackground()
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error starting daemon: %v\n", err)
		} else {
			fmt.Printf("  Daemon started in background (PID %d)\n", pid)
		}
		tui.PrintPressEnter()
	case "stop":
		_ = runStop(nil, nil)
	case "install":
		if err := daemon.InstallService(); err != nil {
			fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
		}
		tui.PrintPressEnter()
	case "uninstall":
		ok, err := tui.ConfirmAction(
			"Remove from boot?",
			"This will uninstall the system service so opencrons no longer starts automatically on boot.",
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
			tui.PrintPressEnter()
			return
		}
		if !ok {
			return
		}
		if err := daemon.UninstallService(); err != nil {
			fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
		}
		tui.PrintPressEnter()
	case "killall":
		ok, err := tui.ConfirmAction(
			"Kill all opencrons processes?",
			"This will force-kill every opencrons process running on this machine and remove the PID file.",
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
			tui.PrintPressEnter()
			return
		}
		if !ok {
			return
		}
		killed, err := platform.KillAllDaemonProcesses()
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
		} else if len(killed) == 0 {
			fmt.Println("  No opencrons processes found.")
		} else {
			pids := make([]string, len(killed))
			for i, p := range killed {
				pids[i] = strconv.Itoa(p)
			}
			fmt.Printf("  Killed %d process(es): %s\n", len(killed), strings.Join(pids, ", "))
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
