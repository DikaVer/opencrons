// File disable.go implements the disable command, which deactivates a job
// by setting Enabled to false and saving the updated config.
package cmd

import (
	"fmt"

	"github.com/dika-maulidal/opencron/internal/config"
	"github.com/dika-maulidal/opencron/internal/platform"
	"github.com/spf13/cobra"
)

var disableCmd = &cobra.Command{
	Use:   "disable [name]",
	Short: "Disable a scheduled job",
	Long:  "Disable a job so it won't run on schedule. The config is preserved. If no name is given, shows a list to pick from.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runDisable,
}

func init() {
	rootCmd.AddCommand(disableCmd)
}

func runDisable(cmd *cobra.Command, args []string) error {
	name, err := resolveJobName(args)
	if err != nil {
		return err
	}
	if name == "" {
		return nil
	}
	return disableJob(name)
}

// disableJob sets a job's enabled flag to false.
func disableJob(name string) error {
	job, err := config.FindJobByName(platform.SchedulesDir(), name)
	if err != nil {
		return err
	}

	if !job.Enabled {
		fmt.Printf("Job %q is already disabled.\n", name)
		return nil
	}

	job.Enabled = false
	if err := config.SaveJob(platform.SchedulesDir(), job); err != nil {
		return fmt.Errorf("saving job config: %w", err)
	}

	fmt.Printf("Job %q disabled. It will be skipped by the daemon.\n", name)
	return nil
}
