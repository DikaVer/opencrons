// File status.go implements the status command, which shows whether the daemon
// is running and lists the next scheduled run times for all enabled jobs.
package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/DikaVer/opencron/internal/config"
	"github.com/DikaVer/opencron/internal/platform"
	"github.com/DikaVer/opencron/internal/ui"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status and next scheduled runs",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Daemon status
	pid, running := platform.CheckDaemonRunning()
	if running {
		fmt.Printf("  Daemon: %s (PID %d)\n", ui.Success.Render("running"), pid)
	} else {
		fmt.Printf("  Daemon: %s\n", ui.Fail.Render("stopped"))
	}

	// Show jobs and next run times
	jobs, err := config.LoadAllJobs(platform.SchedulesDir())
	if err != nil {
		return fmt.Errorf("loading jobs: %w", err)
	}

	if len(jobs) == 0 {
		fmt.Println("\n  No jobs configured.")
		return nil
	}

	fmt.Println()
	fmt.Printf("  %s\n", ui.Title.Render("Scheduled Jobs:"))
	fmt.Println(ui.Dim.Render("  " + strings.Repeat("-", 60)))

	now := time.Now()

	for _, job := range jobs {
		if !job.Enabled {
			fmt.Printf("  %-20s  %s\n", job.Name, ui.Dim.Render("disabled"))
			continue
		}

		sched, err := ui.CronParser.Parse(job.Schedule)
		if err != nil {
			fmt.Printf("  %-20s  %s\n", job.Name, ui.Fail.Render("invalid schedule"))
			continue
		}

		nextRun := sched.Next(now)
		until := time.Until(nextRun).Round(time.Minute)

		fmt.Printf("  %-20s  next: %s (%s)\n",
			job.Name,
			nextRun.Format("2006-01-02 15:04"),
			ui.Dim.Render(fmt.Sprintf("in %s", until)),
		)
	}

	return nil
}
