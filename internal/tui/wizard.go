package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/dika-maulidal/cli-scheduler/internal/config"
	"github.com/dika-maulidal/cli-scheduler/internal/platform"
	"github.com/google/uuid"
)

var theme = func() *huh.Theme {
	t := huh.ThemeCatppuccin()
	return t
}()

// WizardResult holds the output of the interactive wizard.
type WizardResult struct {
	Job           *config.JobConfig
	PromptContent string
}

// RunAddWizard runs the interactive wizard to create a new job.
func RunAddWizard() (*WizardResult, error) {
	var (
		name           string
		workingDir     string
		schedule       string
		presetKey      string
		prompt         string
		model          string
		effort         string
		timeout        string
		summaryEnabled bool
	)

	cwd, _ := os.Getwd()

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#cba6f7")).
		MarginBottom(1)

	fmt.Println(titleStyle.Render("  Add New Scheduled Job"))

	// Step 1: Job identity
	step1 := huh.NewGroup(
		huh.NewInput().
			Title("Job Name").
			Description("Unique identifier for this job. Use lowercase with hyphens (e.g. nightly-review, weekly-audit).").
			Placeholder("nightly-review").
			Value(&name).
			Validate(func(s string) error {
				if err := ValidateJobName(s); err != nil {
					return err
				}
				if config.JobNameExists(platform.SchedulesDir(), s) {
					return fmt.Errorf("job %q already exists — choose a different name", s)
				}
				return nil
			}),
		huh.NewInput().
			Title("Working Directory").
			Description("The project directory where Claude will execute. Claude can only access files in this directory and its subdirectories.").
			Placeholder(cwd).
			Value(&workingDir).
			Validate(func(s string) error {
				if s == "" {
					return nil // will default to cwd
				}
				return ValidateDirectory(s)
			}),
	)

	// Step 2: Schedule
	step2 := huh.NewGroup(
		huh.NewSelect[string]().
			Title("Schedule").
			Description("When should this job run? Uses standard cron format (minute hour day-of-month month day-of-week).").
			Options(
				huh.NewOption("Every hour", "0 * * * *"),
				huh.NewOption("Every 6 hours", "0 */6 * * *"),
				huh.NewOption("Daily at 2 AM", "0 2 * * *"),
				huh.NewOption("Daily at 9 AM", "0 9 * * *"),
				huh.NewOption("Weekly (Monday 9 AM)", "0 9 * * 1"),
				huh.NewOption("Custom cron expression", "custom"),
			).
			Value(&presetKey),
	)

	// Step 3: Prompt
	step3 := huh.NewGroup(
		huh.NewText().
			Title("Prompt").
			Description("What should Claude do? Be specific: describe the task, expected output, and any constraints.\nTip: For polished prompts, use /schedule add inside Claude Code instead.").
			Placeholder("Review all changed files for security vulnerabilities and generate a report...").
			CharLimit(5000).
			Lines(6).
			Value(&prompt).
			Validate(ValidateNonEmpty),
	)

	// Step 4: Model
	step4 := huh.NewGroup(
		huh.NewSelect[string]().
			Title("Model").
			Description("Which Claude model to use. Sonnet is the best balance of speed and capability for most tasks.").
			Options(
				huh.NewOption("Sonnet — fast, capable, cost-effective (recommended)", "sonnet"),
				huh.NewOption("Opus — most capable, best for complex reasoning", "opus"),
				huh.NewOption("Haiku — fastest, cheapest, good for simple tasks", "haiku"),
			).
			Value(&model),
	)

	// Step 5: Effort Level
	step5 := huh.NewGroup(
		huh.NewSelect[string]().
			Title("Effort Level").
			Description("Controls how much thinking effort Claude puts in. Higher = better results but more tokens.\n"+
				"Low saves tokens, medium balances cost/quality, high is full capability (default).").
			Options(
				huh.NewOption("High — full capability, default (recommended)", "high"),
				huh.NewOption("Medium — balanced speed and cost", "medium"),
				huh.NewOption("Low — most token-efficient, good for simple tasks", "low"),
				huh.NewOption("Max — absolute maximum capability (Opus only)", "max"),
			).
			Value(&effort),
	)

	// Step 6: Timeout & Summary
	step6 := huh.NewGroup(
		huh.NewInput().
			Title("Timeout (seconds)").
			Description("Maximum wall-clock time for the job. The process is killed if it exceeds this limit.\n"+
				"Quick tasks: 60-120s. Standard tasks: 300s (5 min). Complex tasks: 600-900s. Default: 300.").
			Placeholder("300").
			Value(&timeout),
		huh.NewConfirm().
			Title("Enable Report Summarization").
			Description(fmt.Sprintf("When enabled, Claude generates a short Telegram-style summary after each run.\n"+
				"Summaries are saved to: %s", platform.SummaryDir())).
			Value(&summaryEnabled),
	)

	form := huh.NewForm(step1, step2, step3, step4, step5, step6).
		WithTheme(theme)

	if err := form.Run(); err != nil {
		return nil, err
	}

	// Handle custom cron
	if presetKey == "custom" {
		var customCron string
		customForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Custom Cron Expression").
					Description("Standard 5-field cron: minute hour day-of-month month day-of-week\n"+
						"Examples: */30 * * * * (every 30 min), 0 9,17 * * 1-5 (9am & 5pm weekdays)").
					Placeholder("*/30 * * * *").
					Value(&customCron).
					Validate(ValidateCron),
			),
		).WithTheme(theme)

		if err := customForm.Run(); err != nil {
			return nil, err
		}
		schedule = customCron
	} else {
		schedule = presetKey
	}

	// Default working dir to cwd
	if workingDir == "" {
		workingDir = cwd
	}

	// Normalize effort: "high" is the default, omit it from config
	effortVal := effort
	if effortVal == "high" {
		effortVal = ""
	}

	// Build job config
	job := &config.JobConfig{
		ID:               uuid.New().String()[:8],
		Name:             name,
		Schedule:         schedule,
		WorkingDir:       workingDir,
		PromptFile:       name + ".md",
		Model:            model,
		Timeout:          parseInt(timeout, 300),
		Effort:           effortVal,
		SummaryEnabled:   summaryEnabled,
		NoSessionPersist: true,
		Enabled:          true,
	}

	return &WizardResult{
		Job:           job,
		PromptContent: strings.TrimSpace(prompt),
	}, nil
}

// RunEditWizard runs the edit wizard for an existing job.
func RunEditWizard(job *config.JobConfig, existingPrompt string) (*WizardResult, error) {
	schedule := job.Schedule
	presetKey := job.Schedule
	prompt := existingPrompt
	model := job.Model
	effort := job.Effort
	if effort == "" {
		effort = "high" // default for the UI
	}
	timeout := ""
	if job.Timeout > 0 {
		timeout = fmt.Sprintf("%d", job.Timeout)
	}
	summaryEnabled := job.SummaryEnabled

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#cba6f7")).
		MarginBottom(1)

	fmt.Println(titleStyle.Render(fmt.Sprintf("  Edit Job: %s", job.Name)))

	// Check if current schedule matches a preset
	isPreset := false
	presets := []string{"0 * * * *", "0 */6 * * *", "0 2 * * *", "0 9 * * *", "0 9 * * 1"}
	for _, p := range presets {
		if job.Schedule == p {
			isPreset = true
			break
		}
	}
	if !isPreset {
		presetKey = "custom"
	}

	step1 := huh.NewGroup(
		huh.NewSelect[string]().
			Title("Schedule").
			Description(fmt.Sprintf("Current: %s\nUse arrow keys to navigate, Enter to select.", job.Schedule)).
			Options(
				huh.NewOption("Every hour", "0 * * * *"),
				huh.NewOption("Every 6 hours", "0 */6 * * *"),
				huh.NewOption("Daily at 2 AM", "0 2 * * *"),
				huh.NewOption("Daily at 9 AM", "0 9 * * *"),
				huh.NewOption("Weekly (Monday 9 AM)", "0 9 * * 1"),
				huh.NewOption("Custom cron expression", "custom"),
			).
			Value(&presetKey),
	)

	step2 := huh.NewGroup(
		huh.NewText().
			Title("Prompt").
			Description("Edit the prompt. Be specific about the task, expected output, and constraints.").
			CharLimit(5000).
			Lines(6).
			Value(&prompt).
			Validate(ValidateNonEmpty),
	)

	step3 := huh.NewGroup(
		huh.NewSelect[string]().
			Title("Model").
			Description("Use arrow keys to navigate, Enter to select.").
			Options(
				huh.NewOption("Sonnet — fast, capable, cost-effective (recommended)", "sonnet"),
				huh.NewOption("Opus — most capable, best for complex reasoning", "opus"),
				huh.NewOption("Haiku — fastest, cheapest, good for simple tasks", "haiku"),
			).
			Value(&model),
	)

	step4 := huh.NewGroup(
		huh.NewSelect[string]().
			Title("Effort Level").
			Description("Controls thinking effort. Higher = better results but more tokens.").
			Options(
				huh.NewOption("High — full capability, default (recommended)", "high"),
				huh.NewOption("Medium — balanced speed and cost", "medium"),
				huh.NewOption("Low — most token-efficient, good for simple tasks", "low"),
				huh.NewOption("Max — absolute maximum capability (Opus only)", "max"),
			).
			Value(&effort),
	)

	step5 := huh.NewGroup(
		huh.NewInput().
			Title("Timeout (seconds)").
			Description("Quick: 60-120s. Standard: 300s. Complex: 600-900s. Default: 300.").
			Placeholder("300").
			Value(&timeout),
		huh.NewConfirm().
			Title("Enable Report Summarization").
			Description(fmt.Sprintf("Generates a short Telegram-style summary after each run.\n"+
				"Summaries saved to: %s", platform.SummaryDir())).
			Value(&summaryEnabled),
	)

	form := huh.NewForm(step1, step2, step3, step4, step5).
		WithTheme(theme)

	if err := form.Run(); err != nil {
		return nil, err
	}

	// Handle custom cron
	if presetKey == "custom" {
		customCron := job.Schedule
		customForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Custom Cron Expression").
					Description("5-field cron: minute hour day-of-month month day-of-week").
					Placeholder(job.Schedule).
					Value(&customCron).
					Validate(ValidateCron),
			),
		).WithTheme(theme)

		if err := customForm.Run(); err != nil {
			return nil, err
		}
		schedule = customCron
	} else {
		schedule = presetKey
	}

	// Normalize effort: "high" is the default, omit it from config
	effortVal := effort
	if effortVal == "high" {
		effortVal = ""
	}

	// Update job config (keep ID, Name, WorkingDir, PromptFile)
	job.Schedule = schedule
	job.Model = model
	job.Effort = effortVal
	job.Timeout = parseInt(timeout, 300)
	job.SummaryEnabled = summaryEnabled

	return &WizardResult{
		Job:           job,
		PromptContent: strings.TrimSpace(prompt),
	}, nil
}

func parseInt(s string, defaultVal int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return defaultVal
	}
	var i int
	fmt.Sscanf(s, "%d", &i)
	return i
}
