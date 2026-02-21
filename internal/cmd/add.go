// File add.go implements the job creation command. It supports both an interactive
// wizard via RunAddWizard and a non-interactive mode via flags (--name, --schedule,
// --working-dir, --prompt-file, --prompt-content, --model, --effort, --timeout,
// --summary). It validates inputs, saves the prompt file, and writes the job config.
package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dika-maulidal/opencron/internal/config"
	"github.com/dika-maulidal/opencron/internal/platform"
	"github.com/dika-maulidal/opencron/internal/tui"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new scheduled job",
	Long:  "Create a new scheduled job interactively or via flags with --non-interactive.",
	RunE:  runAdd,
}

func init() {
	rootCmd.AddCommand(addCmd)

	addCmd.Flags().Bool("non-interactive", false, "non-interactive mode (all params via flags)")
	addCmd.Flags().String("name", "", "job name (required)")
	addCmd.Flags().String("schedule", "", "cron schedule expression (required)")
	addCmd.Flags().String("working-dir", "", "working directory (required)")
	addCmd.Flags().String("prompt-file", "", "prompt file name (default: <name>.md)")
	addCmd.Flags().String("prompt-content", "", "prompt content (written to prompt file)")
	addCmd.Flags().String("model", "sonnet", "Claude model: sonnet, opus, haiku")
	addCmd.Flags().String("effort", "", "effort level: low, medium, high (default), max")
	addCmd.Flags().Int("timeout", 300, "timeout in seconds")
	addCmd.Flags().Bool("summary", false, "enable Telegram-style summary after each run")
	addCmd.Flags().StringArray("disallowed-tools", nil, "tools to deny (repeatable, e.g. --disallowed-tools \"Bash(git:*)\" --disallowed-tools \"Edit\")")
}

func runAdd(cmd *cobra.Command, args []string) error {
	if err := platform.EnsureDirs(); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	nonInteractive, _ := cmd.Flags().GetBool("non-interactive")

	if nonInteractive {
		return runAddNonInteractive(cmd)
	}

	return runAddInteractive()
}

func runAddInteractive() error {
	result, err := tui.RunAddWizard()
	if err != nil {
		return fmt.Errorf("wizard failed: %w", err)
	}
	if result == nil {
		fmt.Println("  Cancelled.")
		return nil
	}

	// Save prompt file
	if err := config.SavePromptFile(platform.PromptsDir(), result.Job.PromptFile, result.PromptContent); err != nil {
		return fmt.Errorf("saving prompt file: %w", err)
	}

	// Save job config
	if err := config.SaveJob(platform.SchedulesDir(), result.Job); err != nil {
		return fmt.Errorf("saving job config: %w", err)
	}

	fmt.Printf("\nJob %q created successfully!\n", result.Job.Name)
	fmt.Printf("  Config:   %s\n", filepath.Join(platform.SchedulesDir(), result.Job.Name+".yml"))
	fmt.Printf("  Prompt:   %s\n", filepath.Join(platform.PromptsDir(), result.Job.PromptFile))
	fmt.Printf("  Schedule: %s\n", result.Job.Schedule)

	return nil
}

func runAddNonInteractive(cmd *cobra.Command) error {
	name, _ := cmd.Flags().GetString("name")
	schedule, _ := cmd.Flags().GetString("schedule")
	promptFile, _ := cmd.Flags().GetString("prompt-file")
	promptContent, _ := cmd.Flags().GetString("prompt-content")
	model, _ := cmd.Flags().GetString("model")
	effort, _ := cmd.Flags().GetString("effort")
	timeout, _ := cmd.Flags().GetInt("timeout")
	workingDir, _ := cmd.Flags().GetString("working-dir")
	summaryEnabled, _ := cmd.Flags().GetBool("summary")
	disallowedTools, _ := cmd.Flags().GetStringArray("disallowed-tools")

	// Validate required flags
	var missing []string
	if name == "" {
		missing = append(missing, "--name")
	}
	if schedule == "" {
		missing = append(missing, "--schedule")
	}
	if workingDir == "" {
		missing = append(missing, "--working-dir")
	}
	if len(missing) > 0 {
		return fmt.Errorf("required flags missing: %s", strings.Join(missing, ", "))
	}

	// Check for duplicate name
	if config.JobNameExists(platform.SchedulesDir(), name) {
		return fmt.Errorf("a job named %q already exists", name)
	}

	// Default prompt file name
	if promptFile == "" {
		promptFile = name + ".md"
	}

	// Normalize effort: "high" is the default, omit from config
	if effort == "high" {
		effort = ""
	}

	// Write prompt content if provided
	if promptContent != "" {
		if err := config.SavePromptFile(platform.PromptsDir(), promptFile, promptContent); err != nil {
			return fmt.Errorf("saving prompt file: %w", err)
		}
	}

	job := &config.JobConfig{
		ID:               uuid.New().String()[:8],
		Name:             name,
		Schedule:         schedule,
		WorkingDir:       workingDir,
		PromptFile:       promptFile,
		Model:            model,
		Effort:           effort,
		Timeout:          timeout,
		DisallowedTools:  disallowedTools,
		SummaryEnabled:   summaryEnabled,
		NoSessionPersist: true,
		Enabled:          true,
	}

	// Validate the job config (catches invalid model, effort, cron, etc.)
	if err := job.Validate(); err != nil {
		return err
	}

	if err := config.SaveJob(platform.SchedulesDir(), job); err != nil {
		return fmt.Errorf("saving job config: %w", err)
	}

	// Print detailed creation summary
	fmt.Printf("Job %q created successfully.\n", name)
	fmt.Printf("  Config:   %s\n", filepath.Join(platform.SchedulesDir(), name+".yml"))
	fmt.Printf("  Prompt:   %s\n", filepath.Join(platform.PromptsDir(), promptFile))
	fmt.Printf("  Schedule: %s\n", schedule)
	fmt.Printf("  Model:    %s\n", model)
	if effort != "" {
		fmt.Printf("  Effort:   %s\n", effort)
	}
	fmt.Printf("  Timeout:  %ds\n", timeout)
	if len(disallowedTools) > 0 {
		fmt.Printf("  Denied:   %s\n", strings.Join(disallowedTools, ", "))
	}
	if summaryEnabled {
		fmt.Printf("  Summary:  enabled\n")
	}
	return nil
}
