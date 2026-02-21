// File remove.go implements the remove command, which deletes a job after
// confirmation. It supports --force to skip the confirmation prompt and
// --keep-prompt to preserve the associated prompt file.
package cmd

import (
	"fmt"

	"github.com/DikaVer/opencron/internal/config"
	"github.com/DikaVer/opencron/internal/platform"
	"github.com/DikaVer/opencron/internal/tui"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove [name]",
	Short: "Remove a scheduled job",
	Long:  "Remove a scheduled job. If no name is given, shows a list to pick from.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runRemove,
}

func init() {
	rootCmd.AddCommand(removeCmd)
	removeCmd.Flags().BoolP("force", "f", false, "skip confirmation")
	removeCmd.Flags().Bool("keep-prompt", false, "keep the prompt file")
}

func runRemove(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")
	keepPrompt, _ := cmd.Flags().GetBool("keep-prompt")

	name, err := resolveJobName(args)
	if err != nil {
		return err
	}
	if name == "" {
		return nil
	}

	// Check job exists
	_, err = config.FindJobByName(platform.SchedulesDir(), name)
	if err != nil {
		return err
	}

	if !force {
		confirmed, err := tui.ConfirmAction(
			fmt.Sprintf("Remove job %q?", name),
			"This will delete the job config and prompt file.",
		)
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	deletePrompt := !keepPrompt
	if err := config.DeleteJob(platform.SchedulesDir(), platform.PromptsDir(), name, deletePrompt); err != nil {
		return fmt.Errorf("removing job: %w", err)
	}

	fmt.Printf("Job %q removed.\n", name)
	return nil
}
