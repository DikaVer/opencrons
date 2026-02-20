package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/dika-maulidal/cli-scheduler/internal/platform"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the scheduler daemon",
	RunE:  runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
	pid, running := platform.CheckDaemonRunning()
	if !running {
		fmt.Println("Daemon is not running.")
		return nil
	}

	fmt.Printf("Stopping daemon (PID %d)...\n", pid)

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("finding process: %w", err)
	}

	// Send interrupt signal
	if err := proc.Signal(os.Interrupt); err != nil {
		// On Windows, Interrupt might not work — try Kill
		if err := proc.Kill(); err != nil {
			return fmt.Errorf("killing process: %w", err)
		}
	}

	// Wait for process to exit
	done := make(chan struct{})
	go func() {
		proc.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("Daemon stopped.")
	case <-time.After(10 * time.Second):
		fmt.Println("Timed out waiting — force killing...")
		proc.Kill()
	}

	_ = platform.RemovePID()
	return nil
}
