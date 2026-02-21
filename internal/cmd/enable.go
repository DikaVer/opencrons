// File enable.go implements the enable command, which activates a disabled job
// by setting Enabled to true and saving the updated config.
package cmd

import (
	"fmt"

	"github.com/DikaVer/opencron/internal/config"
	"github.com/DikaVer/opencron/internal/platform"
	"github.com/spf13/cobra"
)

var enableCmd = &cobra.Command{
	Use:   "enable [name]",
	Short: "Enable a scheduled job",
	Long:  "Enable a previously disabled job so it runs on schedule again. If no name is given, shows a list to pick from.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runEnable,
}

func init() {
	rootCmd.AddCommand(enableCmd)
}

func runEnable(cmd *cobra.Command, args []string) error {
	name, err := resolveJobName(args)
	if err != nil {
		return err
	}
	if name == "" {
		return nil
	}
	return enableJob(name)
}

// enableJob sets a job's enabled flag to true.
func enableJob(name string) error {
	job, err := config.FindJobByName(platform.SchedulesDir(), name)
	if err != nil {
		return err
	}

	if job.Enabled {
		fmt.Printf("Job %q is already enabled.\n", name)
		return nil
	}

	job.Enabled = true
	if err := config.SaveJob(platform.SchedulesDir(), job); err != nil {
		return fmt.Errorf("saving job config: %w", err)
	}

	fmt.Printf("Job %q enabled. It will run on schedule.\n", name)
	return nil
}
