package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dika-maulidal/cli-scheduler/internal/config"
	"github.com/dika-maulidal/cli-scheduler/internal/platform"
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
		fmt.Println("No jobs configured. Use 'scheduler add' to create one.")
		return nil
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#cba6f7"))
	enabledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1"))
	disabledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))

	// Header
	fmt.Fprintf(os.Stdout, "  %s  %s  %s  %s  %s\n",
		headerStyle.Width(20).Render("NAME"),
		headerStyle.Width(18).Render("SCHEDULE"),
		headerStyle.Width(10).Render("MODEL"),
		headerStyle.Width(16).Render("MODE"),
		headerStyle.Width(8).Render("STATUS"),
	)

	fmt.Println(dimStyle.Render("  "+strings.Repeat("-", 76)))

	for _, job := range jobs {
		status := enabledStyle.Render("enabled")
		if !job.Enabled {
			status = disabledStyle.Render("disabled")
		}

		fmt.Fprintf(os.Stdout, "  %-20s  %-18s  %-10s  %-16s  %s\n",
			truncate(job.Name, 20),
			truncate(job.Schedule, 18),
			truncate(job.Model, 10),
			truncate(job.PermissionMode, 16),
			status,
		)
	}

	fmt.Printf("\n  %s %d job(s) configured\n", dimStyle.Render("Total:"), len(jobs))
	return nil
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
}
