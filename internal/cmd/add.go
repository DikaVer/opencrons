// File add.go implements the job creation command. It supports both an interactive
// wizard via RunAddWizard and a non-interactive mode via flags (--name, --schedule,
// --working-dir, --prompt-file, --prompt-content, --model, --effort, --timeout,
// --summary). It validates inputs, saves the prompt file, and writes the job config.
package cmd

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/DikaVer/opencrons/internal/config"
	"github.com/DikaVer/opencrons/internal/logger"
	"github.com/DikaVer/opencrons/internal/platform"
	"github.com/DikaVer/opencrons/internal/tui"
	"github.com/DikaVer/opencrons/internal/ui"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var cmdlog = logger.New("cmd")

// Package-level typed flag values for non-interactive mode.
var (
	flagName       = ui.NewJobNameValue("")
	flagSchedule   = ui.NewCronValue("")
	flagWorkingDir = ui.NewDirValue("")
	flagModel      = ui.NewModelValue("sonnet")
	flagEffort     = ui.NewEffortValue("")
	flagTimeout    = ui.NewTimeoutValue(300)
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
	addCmd.Flags().Var(flagName, "name", "job name (required)")
	addCmd.Flags().Var(flagSchedule, "schedule", "cron schedule expression (required)")
	addCmd.Flags().Var(flagWorkingDir, "working-dir", "working directory (default: projects/<name>/)")
	addCmd.Flags().String("prompt-file", "", "prompt file name (default: <name>.md)")
	addCmd.Flags().String("prompt-content", "", "prompt content (written to prompt file)")
	addCmd.Flags().Var(flagModel, "model", "Claude model: sonnet, opus, haiku")
	addCmd.Flags().Var(flagEffort, "effort", "effort level: low, medium, high (default), max")
	addCmd.Flags().Var(flagTimeout, "timeout", "timeout in seconds")
	addCmd.Flags().Bool("summary", false, "enable Telegram-style summary after each run")
	addCmd.Flags().StringArray("disallowed-tools", nil, "tools to deny (repeatable, e.g. --disallowed-tools \"Bash(git:*)\" --disallowed-tools \"Edit\")")
	addCmd.Flags().Int("max-retries", 0, "number of retries on failure (0-10)")
	addCmd.Flags().String("retry-backoff", "exponential", "retry delay strategy: exponential (default) or linear")
	addCmd.Flags().StringArray("on-success", nil, "jobs to trigger on success (repeatable, e.g. --on-success job-b --on-success job-c)")

	// Shell completion for enum flags
	modelKeys := make([]string, 0, len(ui.ValidModels))
	for k := range ui.ValidModels {
		modelKeys = append(modelKeys, k)
	}
	sort.Strings(modelKeys)
	_ = addCmd.RegisterFlagCompletionFunc("model", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return modelKeys, cobra.ShellCompDirectiveNoFileComp
	})

	effortKeys := make([]string, 0, len(ui.ValidEfforts))
	for k := range ui.ValidEfforts {
		effortKeys = append(effortKeys, k)
	}
	sort.Strings(effortKeys)
	_ = addCmd.RegisterFlagCompletionFunc("effort", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return effortKeys, cobra.ShellCompDirectiveNoFileComp
	})

	_ = addCmd.RegisterFlagCompletionFunc("working-dir", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveFilterDirs
	})
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

	cmdlog.Info("job created", "name", result.Job.Name, "schedule", result.Job.Schedule)
	fmt.Printf("\nJob %q created successfully!\n", result.Job.Name)
	fmt.Printf("  Config:   %s\n", filepath.Join(platform.SchedulesDir(), result.Job.Name+".yml"))
	fmt.Printf("  Prompt:   %s\n", filepath.Join(platform.PromptsDir(), result.Job.PromptFile))
	fmt.Printf("  Schedule: %s\n", result.Job.Schedule)

	return nil
}

func runAddNonInteractive(cmd *cobra.Command) error {
	name := flagName.String()
	schedule := flagSchedule.String()
	workingDir := flagWorkingDir.String()
	model := flagModel.String()
	effort := flagEffort.String()
	timeout := flagTimeout.Int()
	promptFile, _ := cmd.Flags().GetString("prompt-file")
	promptContent, _ := cmd.Flags().GetString("prompt-content")
	summaryEnabled, _ := cmd.Flags().GetBool("summary")
	disallowedTools, _ := cmd.Flags().GetStringArray("disallowed-tools")
	maxRetries, _ := cmd.Flags().GetInt("max-retries")
	retryBackoff, _ := cmd.Flags().GetString("retry-backoff")
	// BackoffExponential ("") is the default; omit from config for clean YAML.
	if retryBackoff == "exponential" {
		retryBackoff = config.BackoffExponential
	}
	onSuccess, _ := cmd.Flags().GetStringArray("on-success")

	// Validate required flags
	var missing []string
	if !cmd.Flags().Changed("name") {
		missing = append(missing, "--name")
	}
	if !cmd.Flags().Changed("schedule") {
		missing = append(missing, "--schedule")
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
		MaxRetries:       maxRetries,
		RetryBackoff:     retryBackoff,
		NoSessionPersist: true,
		Enabled:          true,
		OnSuccess:        onSuccess,
	}

	// Validate the job config (defense-in-depth)
	if err := job.Validate(); err != nil {
		return err
	}

	// Verify on-success targets exist at creation time
	for _, target := range onSuccess {
		if !config.JobNameExists(platform.SchedulesDir(), target) {
			return fmt.Errorf("--on-success references unknown job %q", target)
		}
	}

	if err := config.SaveJob(platform.SchedulesDir(), job); err != nil {
		return fmt.Errorf("saving job config: %w", err)
	}

	cmdlog.Info("job created", "name", name, "schedule", schedule, "model", model)

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
	if len(onSuccess) > 0 {
		fmt.Printf("  On success: %s\n", strings.Join(onSuccess, ", "))
	}
	return nil
}
