package cmd

import (
	"fmt"

	"github.com/dika-maulidal/cli-scheduler/internal/daemon"
	"github.com/dika-maulidal/cli-scheduler/internal/platform"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the scheduler daemon",
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

	fmt.Println("Starting scheduler daemon...")
	return daemon.Run()
}
