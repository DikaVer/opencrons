// File list.go implements the list command, which displays all configured jobs
// in a styled table showing name, schedule, model, effort, and enabled status.
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/DikaVer/opencrons/internal/config"
	"github.com/DikaVer/opencrons/internal/platform"
	"github.com/DikaVer/opencrons/internal/ui"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all scheduled jobs",
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	jobs, err := config.LoadAllJobs(platform.SchedulesDir())
	if err != nil {
		return fmt.Errorf("loading jobs: %w", err)
	}

	if len(jobs) == 0 {
		fmt.Println("No jobs configured. Use 'opencrons add' to create one.")
		return nil
	}

	// Header
	fmt.Fprintf(os.Stdout, "  %s  %s  %s  %s  %s\n",
		ui.Title.Width(20).Render("NAME"),
		ui.Title.Width(18).Render("SCHEDULE"),
		ui.Title.Width(10).Render("MODEL"),
		ui.Title.Width(10).Render("EFFORT"),
		ui.Title.Width(8).Render("STATUS"),
	)

	fmt.Println(ui.Dim.Render("  " + strings.Repeat("-", 70)))

	for _, job := range jobs {
		status := ui.Success.Render("enabled")
		if !job.Enabled {
			status = ui.Fail.Render("disabled")
		}

		effort := job.Effort
		if effort == "" {
			effort = "high"
		}

		fmt.Fprintf(os.Stdout, "  %-20s  %-18s  %-10s  %-10s  %s\n",
			ui.Truncate(job.Name, 20),
			ui.Truncate(job.Schedule, 18),
			ui.Truncate(job.Model, 10),
			effort,
			status,
		)
	}

	fmt.Printf("\n  %s %d job(s) configured\n", ui.Dim.Render("Total:"), len(jobs))
	return nil
}
