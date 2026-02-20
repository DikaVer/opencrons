package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dika-maulidal/cli-scheduler/internal/config"
	"github.com/dika-maulidal/cli-scheduler/internal/platform"
	"github.com/dika-maulidal/cli-scheduler/internal/tui"
	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit [name]",
	Short: "Edit an existing job",
	Long:  "Edit a job's configuration. If no name is given, shows a list to pick from.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runEdit,
}

func init() {
	rootCmd.AddCommand(editCmd)
}

func runEdit(cmd *cobra.Command, args []string) error {
	name, err := resolveJobName(args)
	if err != nil {
		return err
	}
	if name == "" {
		return nil // user cancelled
	}
	return editJob(name)
}

// editJob runs the edit wizard for the given job name.
func editJob(name string) error {
	job, err := config.FindJobByName(platform.SchedulesDir(), name)
	if err != nil {
		return err
	}

	// Read existing prompt
	promptPath := filepath.Join(platform.PromptsDir(), job.PromptFile)
	existingPrompt := ""
	if data, err := os.ReadFile(promptPath); err == nil {
		existingPrompt = string(data)
	}

	result, err := tui.RunEditWizard(job, existingPrompt)
	if err != nil {
		return fmt.Errorf("edit wizard failed: %w", err)
	}

	// Save prompt file
	if err := config.SavePromptFile(platform.PromptsDir(), result.Job.PromptFile, result.PromptContent); err != nil {
		return fmt.Errorf("saving prompt file: %w", err)
	}

	// Save job config
	if err := config.SaveJob(platform.SchedulesDir(), result.Job); err != nil {
		return fmt.Errorf("saving job config: %w", err)
	}

	fmt.Printf("Job %q updated.\n", name)
	return nil
}

// resolveJobName picks a job name from args or shows an interactive picker.
func resolveJobName(args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}

	name, err := tui.RunJobPicker(
		"Select a job",
		"Use arrow keys to navigate, Enter to select.",
	)
	if name == "__add__" {
		return "", nil // not applicable in CLI commands
	}
	return name, err
}
