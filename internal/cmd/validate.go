// File validate.go implements the validate command, which checks all job configs
// and their associated prompt files for correctness. It reports OK, FAIL, or WARN
// status per job.
package cmd

import (
	"fmt"

	"github.com/DikaVer/opencrons/internal/config"
	"github.com/DikaVer/opencrons/internal/platform"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate all job configurations",
	RunE:  runValidate,
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

func runValidate(cmd *cobra.Command, args []string) error {
	jobs, err := config.LoadAllJobs(platform.SchedulesDir())
	if err != nil {
		return fmt.Errorf("loading jobs: %w", err)
	}

	if len(jobs) == 0 {
		fmt.Println("No jobs configured.")
		return nil
	}

	hasErrors := false
	for _, job := range jobs {
		if err := job.Validate(); err != nil {
			fmt.Printf("  FAIL  %s: %v\n", job.Name, err)
			hasErrors = true
			continue
		}

		if err := job.ValidatePromptFileExists(platform.PromptsDir()); err != nil {
			fmt.Printf("  WARN  %s: %v\n", job.Name, err)
			continue
		}

		fmt.Printf("  OK    %s\n", job.Name)
	}

	if hasErrors {
		return fmt.Errorf("validation failed for one or more jobs")
	}

	fmt.Printf("\nAll %d job(s) valid.\n", len(jobs))
	return nil
}
