// File start.go implements the start daemon command.
// Default behaviour (no flags): spawns the daemon as a detached background
// process and returns immediately so the terminal/TUI is not blocked.
// --foreground: run blocking in the current process (for service managers).
// --install: register as an OS service instead of starting directly.
package cmd

import (
	"fmt"

	"github.com/DikaVer/opencrons/internal/daemon"
	"github.com/DikaVer/opencrons/internal/platform"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the OpenCron daemon",
	RunE:  runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().Bool("install", false, "install as a service (user service by default, no sudo needed)")
	startCmd.Flags().Bool("system", false, "install as system-wide service (requires sudo, use with --install)")
	startCmd.Flags().Bool("foreground", false, "run in foreground (blocks until stopped)")
}

func runStart(cmd *cobra.Command, args []string) error {
	if err := platform.EnsureDirs(); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	install, _ := cmd.Flags().GetBool("install")
	if install {
		system, _ := cmd.Flags().GetBool("system")
		if system {
			return daemon.InstallSystemService()
		}
		return daemon.InstallService()
	}

	// Check if already running before doing anything else.
	if pid, running := platform.CheckDaemonRunning(); running {
		return fmt.Errorf("daemon already running (PID %d)", pid)
	}

	foreground, _ := cmd.Flags().GetBool("foreground")
	if foreground {
		cmdlog.Info("daemon start requested", "mode", "foreground")
		fmt.Println("Starting OpenCron daemon (foreground)...")
		return daemon.Run()
	}

	cmdlog.Info("daemon start requested", "mode", "background")

	// Default: spawn detached background process and return.
	pid, err := daemon.RunBackground()
	if err != nil {
		return fmt.Errorf("starting daemon in background: %w", err)
	}
	fmt.Printf("Daemon started in background (PID %d)\n", pid)
	return nil
}
