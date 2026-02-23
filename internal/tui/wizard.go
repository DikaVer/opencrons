// wizard.go implements the job creation and editing wizards.
//
// WizardResult holds the resulting job configuration and prompt content.
// RunAddWizard presents a 7-step form covering name, working directory, schedule
// (presets or custom cron), prompt text, model, effort, allowed tools, timeout,
// and summary toggle. RunEditWizard allows modifying an existing job's
// configuration. Both wizards use the Catppuccin Mocha theme.
package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/DikaVer/opencrons/internal/config"
	"github.com/DikaVer/opencrons/internal/platform"
	"github.com/DikaVer/opencrons/internal/ui"
	"github.com/google/uuid"
)

var theme = func() *huh.Theme {
	t := huh.ThemeCatppuccin()
	return t
}()

// claudeCodeTools defines the known Claude Code tools for the allowed tools selector.
var claudeCodeTools = []struct {
	Name  string
	Label string
}{
	{"Bash", "Bash — execute shell commands"},
	{"Read", "Read — read file contents"},
	{"Write", "Write — create new files"},
	{"Edit", "Edit — modify existing files"},
	{"Glob", "Glob — find files by pattern"},
	{"Grep", "Grep — search file contents"},
	{"WebFetch", "WebFetch — fetch web content"},
	{"WebSearch", "WebSearch — search the web"},
	{"NotebookEdit", "NotebookEdit — edit Jupyter notebooks"},
	{"Task", "Task — spawn subagent tasks"},
}

// Tools that are ALLOWED by default in the add wizard.
var defaultAllowedTools = map[string]bool{
	"Read": true, "Glob": true, "Grep": true, "WebFetch": true, "WebSearch": true,
}

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

	fmt.Println(ui.Title.MarginBottom(1).Render("  ✨ Add New Scheduled Job"))

	// Step 1: Job identity
	step1 := huh.NewGroup(
		huh.NewInput().
			Title("🏷️  Job Name").
			Description("Unique identifier for this job. Use lowercase with hyphens (e.g. nightly-review, weekly-audit).").
			Placeholder("nightly-review").
			Value(&name).
			Validate(func(s string) error {
				if err := ui.ValidateJobName(s); err != nil {
					return err
				}
				if config.JobNameExists(platform.SchedulesDir(), s) {
					return fmt.Errorf("job %q already exists — choose a different name", s)
				}
				return nil
			}),
		huh.NewInput().
			Title("📁 Working Directory").
			Description("Directory where Claude executes. Leave empty to use projects/<job-name>/ (auto-created). Enter a path to use an existing directory.").
			Placeholder("(leave empty for managed project folder)").
			Value(&workingDir).
			Validate(func(s string) error {
				if s == "" {
					return nil // empty = use managed project folder
				}
				return ui.ValidateDirectory(s)
			}),
	)

	// Step 2: Schedule
	step2 := huh.NewGroup(
		huh.NewSelect[string]().
			Title("⏰ Schedule").
			Description("When should this job run? Uses standard cron format (minute hour day-of-month month day-of-week).").
			Options(
				huh.NewOption("🔄 Every hour", "0 * * * *"),
				huh.NewOption("🔄 Every 6 hours", "0 */6 * * *"),
				huh.NewOption("🌙 Daily at 2 AM", "0 2 * * *"),
				huh.NewOption("☀️  Daily at 9 AM", "0 9 * * *"),
				huh.NewOption("📅 Weekly (Monday 9 AM)", "0 9 * * 1"),
				huh.NewOption("✏️  Custom cron expression", "custom"),
			).
			Value(&presetKey),
	)

	// Step 3: Prompt
	step3 := huh.NewGroup(
		huh.NewText().
			Title("📝 Prompt").
			Description("What should Claude do? Be specific: describe the task, expected output, and any constraints.\nTip: For polished prompts, use /schedule add inside Claude Code instead.").
			Placeholder("Review all changed files for security vulnerabilities and generate a report...").
			CharLimit(5000).
			Lines(6).
			Value(&prompt).
			Validate(ui.ValidateNonEmpty),
	)

	// Step 4: Model
	step4 := huh.NewGroup(
		huh.NewSelect[string]().
			Title("🧠 Model").
			Description("Which Claude model to use. Sonnet is the best balance of speed and capability for most tasks.").
			Options(
				huh.NewOption("⚡ Sonnet — fast, capable, cost-effective (recommended)", "sonnet"),
				huh.NewOption("🧠 Opus — most capable, best for complex reasoning", "opus"),
				huh.NewOption("🐇 Haiku — fastest, cheapest, good for simple tasks", "haiku"),
			).
			Value(&model),
	)

	// Step 5: Effort Level
	step5 := huh.NewGroup(
		huh.NewSelect[string]().
			Title("💪 Effort Level").
			Description("Controls how much thinking effort Claude puts in. Higher = better results but more tokens.\n"+
				"Low saves tokens, medium balances cost/quality, high is full capability (default).").
			Options(
				huh.NewOption("🔥 High — full capability, default (recommended)", "high"),
				huh.NewOption("⚖️  Medium — balanced speed and cost", "medium"),
				huh.NewOption("💨 Low — most token-efficient, good for simple tasks", "low"),
				huh.NewOption("💎 Max — absolute maximum capability (Opus only)", "max"),
			).
			Value(&effort),
	)

	// Step 6: Allowed Tools — select which tools Claude can use
	var allowedTools []string
	var toolOptions []huh.Option[string]
	for _, t := range claudeCodeTools {
		opt := huh.NewOption(t.Label, t.Name)
		if defaultAllowedTools[t.Name] {
			opt = opt.Selected(true)
		}
		toolOptions = append(toolOptions, opt)
	}
	step6 := huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("🔧 Allowed Tools").
			Description("Selected tools will be AVAILABLE during job execution. Deselect to deny.").
			Options(toolOptions...).
			Value(&allowedTools).
			Validate(func(t []string) error {
				if len(t) == 0 {
					return fmt.Errorf("at least one tool must be allowed")
				}
				return nil
			}),
	)

	// Step 7: Timeout & Summary
	step7 := huh.NewGroup(
		huh.NewInput().
			Title("⏱️  Timeout (seconds)").
			Description("Maximum wall-clock time for the job. The process is killed if it exceeds this limit.\n"+
				"Quick tasks: 60-120s. Standard tasks: 300s (5 min). Complex tasks: 600-900s. Default: 300.").
			Placeholder("300").
			Value(&timeout),
		huh.NewConfirm().
			Title("📊 Enable Report Summarization").
			Description("When enabled, Claude generates a short Telegram-style summary after each run.").
			Value(&summaryEnabled),
	)

	form := huh.NewForm(step1, step2, step3, step4, step5, step6, step7).
		WithTheme(theme)

	if err := form.Run(); err != nil {
		if IsAborted(err) {
			return nil, nil
		}
		return nil, err
	}

	// Handle custom cron
	if presetKey == "custom" {
		var customCron string
		customForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("✏️  Custom Cron Expression").
					Description("Standard 5-field cron: minute hour day-of-month month day-of-week\n" +
						"Examples: */30 * * * * (every 30 min), 0 9,17 * * 1-5 (9am & 5pm weekdays)").
					Placeholder("*/30 * * * *").
					Value(&customCron).
					Validate(ui.ValidateCron),
			),
		).WithTheme(theme)

		if err := customForm.Run(); err != nil {
			if IsAborted(err) {
				return nil, nil
			}
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
		DisallowedTools:  nilIfEmpty(computeDisallowedTools(allowedTools)),
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
	var schedule string
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

	// Build set of currently disallowed tools to invert for the "allowed" UI
	disabledSet := make(map[string]bool)
	for _, t := range job.DisallowedTools {
		disabledSet[t] = true
	}
	var allowedTools []string

	fmt.Println(ui.Title.MarginBottom(1).Render(fmt.Sprintf("  ✏️  Edit Job: %s", job.Name)))

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
			Title("⏰ Schedule").
			Description(fmt.Sprintf("Current: %s", job.Schedule)).
			Options(
				huh.NewOption("🔄 Every hour", "0 * * * *"),
				huh.NewOption("🔄 Every 6 hours", "0 */6 * * *"),
				huh.NewOption("🌙 Daily at 2 AM", "0 2 * * *"),
				huh.NewOption("☀️  Daily at 9 AM", "0 9 * * *"),
				huh.NewOption("📅 Weekly (Monday 9 AM)", "0 9 * * 1"),
				huh.NewOption("✏️  Custom cron expression", "custom"),
			).
			Value(&presetKey),
	)

	step2 := huh.NewGroup(
		huh.NewText().
			Title("📝 Prompt").
			Description("Edit the prompt. Be specific about the task, expected output, and constraints.").
			CharLimit(5000).
			Lines(6).
			Value(&prompt).
			Validate(ui.ValidateNonEmpty),
	)

	step3 := huh.NewGroup(
		huh.NewSelect[string]().
			Title("🧠 Model").
			Options(
				huh.NewOption("⚡ Sonnet — fast, capable, cost-effective (recommended)", "sonnet"),
				huh.NewOption("🧠 Opus — most capable, best for complex reasoning", "opus"),
				huh.NewOption("🐇 Haiku — fastest, cheapest, good for simple tasks", "haiku"),
			).
			Value(&model),
	)

	step4 := huh.NewGroup(
		huh.NewSelect[string]().
			Title("💪 Effort Level").
			Description("Controls thinking effort. Higher = better results but more tokens.").
			Options(
				huh.NewOption("🔥 High — full capability, default (recommended)", "high"),
				huh.NewOption("⚖️  Medium — balanced speed and cost", "medium"),
				huh.NewOption("💨 Low — most token-efficient, good for simple tasks", "low"),
				huh.NewOption("💎 Max — absolute maximum capability (Opus only)", "max"),
			).
			Value(&effort),
	)

	// Step 5: Allowed Tools — pre-select tools NOT in the disallowed list
	var toolOptions []huh.Option[string]
	for _, t := range claudeCodeTools {
		opt := huh.NewOption(t.Label, t.Name)
		if !disabledSet[t.Name] {
			opt = opt.Selected(true)
		}
		toolOptions = append(toolOptions, opt)
	}
	step5 := huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("🔧 Allowed Tools").
			Description("Selected tools will be AVAILABLE during job execution. Deselect to deny.").
			Options(toolOptions...).
			Value(&allowedTools).
			Validate(func(t []string) error {
				if len(t) == 0 {
					return fmt.Errorf("at least one tool must be allowed")
				}
				return nil
			}),
	)

	// Step 6: Timeout & Summary
	step6 := huh.NewGroup(
		huh.NewInput().
			Title("⏱️  Timeout (seconds)").
			Description("Quick: 60-120s. Standard: 300s. Complex: 600-900s. Default: 300.").
			Placeholder("300").
			Value(&timeout),
		huh.NewConfirm().
			Title("📊 Enable Report Summarization").
			Description("Generates a short Telegram-style summary after each run.\n"+
				"Summary is sent directly to Telegram as the job output.").
			Value(&summaryEnabled),
	)

	form := huh.NewForm(step1, step2, step3, step4, step5, step6).
		WithTheme(theme)

	if err := form.Run(); err != nil {
		if IsAborted(err) {
			return nil, nil
		}
		return nil, err
	}

	// Handle custom cron
	if presetKey == "custom" {
		customCron := job.Schedule
		customForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("✏️  Custom Cron Expression").
					Description("5-field cron: minute hour day-of-month month day-of-week").
					Placeholder(job.Schedule).
					Value(&customCron).
					Validate(ui.ValidateCron),
			),
		).WithTheme(theme)

		if err := customForm.Run(); err != nil {
			if IsAborted(err) {
				return nil, nil
			}
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

	// Compute disallowed from allowed selection, preserving custom patterns from YAML
	disallowed := computeDisallowedTools(allowedTools)
	knownToolNames := make(map[string]bool)
	for _, t := range claudeCodeTools {
		knownToolNames[t.Name] = true
	}
	for _, t := range job.DisallowedTools {
		if !knownToolNames[t] {
			disallowed = append(disallowed, t)
		}
	}

	// Update job config (keep ID, Name, WorkingDir, PromptFile)
	job.Schedule = schedule
	job.Model = model
	job.Effort = effortVal
	job.Timeout = parseInt(timeout, 300)
	job.DisallowedTools = nilIfEmpty(disallowed)
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
	i, err := strconv.Atoi(s)
	if err != nil || i <= 0 {
		return defaultVal
	}
	return i
}

// computeDisallowedTools returns the known tools NOT present in the allowed list.
func computeDisallowedTools(allowed []string) []string {
	allowedSet := make(map[string]bool)
	for _, t := range allowed {
		allowedSet[t] = true
	}
	var disallowed []string
	for _, t := range claudeCodeTools {
		if !allowedSet[t.Name] {
			disallowed = append(disallowed, t.Name)
		}
	}
	return disallowed
}

// nilIfEmpty returns nil if the slice is empty, preserving omitempty behavior.
func nilIfEmpty(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}
