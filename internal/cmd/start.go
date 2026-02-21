// File start.go implements the start daemon command. It supports a --install flag
// for system service installation and checks whether the daemon is already running
// before launching.
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
	startCmd.Flags().Bool("install", false, "install as system service")
	startCmd.Flags().Bool("foreground", false, "run in foreground (default)")
}

func runStart(cmd *cobra.Command, args []string) error {
	if err := platform.EnsureDirs(); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	install, _ := cmd.Flags().GetBool("install")

	if install {
		return daemon.InstallService()
	}

	// Check if already running
	if pid, running := platform.CheckDaemonRunning(); running {
		return fmt.Errorf("daemon already running (PID %d)", pid)
	}

	fmt.Println("Starting OpenCron daemon...")
	return daemon.Run()
}
