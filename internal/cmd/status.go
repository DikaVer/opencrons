package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/dika-maulidal/cli-scheduler/internal/config"
	"github.com/dika-maulidal/cli-scheduler/internal/platform"
	"github.com/robfig/cron/v3"
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
	accentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#cba6f7"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1"))
	failStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))

	// Daemon status
	pid, running := platform.CheckDaemonRunning()
	if running {
		fmt.Printf("  Daemon: %s (PID %d)\n", successStyle.Render("running"), pid)
	} else {
		fmt.Printf("  Daemon: %s\n", failStyle.Render("stopped"))
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
	fmt.Printf("  %s\n", accentStyle.Render("Scheduled Jobs:"))
	fmt.Println(dimStyle.Render("  " + strings.Repeat("-", 60)))

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	now := time.Now()

	for _, job := range jobs {
		if !job.Enabled {
			fmt.Printf("  %-20s  %s\n", job.Name, dimStyle.Render("disabled"))
			continue
		}

		sched, err := parser.Parse(job.Schedule)
		if err != nil {
			fmt.Printf("  %-20s  %s\n", job.Name, failStyle.Render("invalid schedule"))
			continue
		}

		nextRun := sched.Next(now)
		until := time.Until(nextRun).Round(time.Minute)

		fmt.Printf("  %-20s  next: %s (%s)\n",
			job.Name,
			nextRun.Format("2006-01-02 15:04"),
			dimStyle.Render(fmt.Sprintf("in %s", until)),
		)
	}

	return nil
}
