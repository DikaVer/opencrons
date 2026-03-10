// wizard.go implements the job creation and editing wizards.
//
// WizardResult holds the resulting job configuration and prompt content.
// RunAddWizard presents a multi-step form covering name, working directory,
// schedule (presets or custom cron), prompt text, model+effort, allowed tools,
// timeout, retry, and summary toggle. RunEditWizard modifies an existing job.
// Both wizards use the Catppuccin Mocha theme.
//
// Each huh.Group is built by a named step-builder function so that adding or
// reordering wizard fields requires editing only the relevant builder.
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

// ── Step builder functions ────────────────────────────────────────────────────

// buildNameStep builds the job identity step (name + working directory).
func buildNameStep(name, workingDir *string) *huh.Group {
	return huh.NewGroup(
		huh.NewInput().
			Title("🏷️  Job Name").
			Description("Unique identifier for this job. Use lowercase with hyphens (e.g. nightly-review, weekly-audit).").
			Placeholder("nightly-review").
			Value(name).
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
			Value(workingDir).
			Validate(func(s string) error {
				if s == "" {
					return nil // empty = use managed project folder
				}
				return ui.ValidateDirectory(s)
			}),
	)
}

// buildScheduleStep builds the schedule preset step.
// desc is shown as the field description; pass the current schedule for edit mode.
func buildScheduleStep(presetKey *string, desc string) *huh.Group {
	return huh.NewGroup(
		huh.NewSelect[string]().
			Title("⏰ Schedule").
			Description(desc).
			Options(
				huh.NewOption("🔄 Every hour", "0 * * * *"),
				huh.NewOption("🔄 Every 6 hours", "0 */6 * * *"),
				huh.NewOption("🌙 Daily at 2 AM", "0 2 * * *"),
				huh.NewOption("☀️  Daily at 9 AM", "0 9 * * *"),
				huh.NewOption("📅 Weekly (Monday 9 AM)", "0 9 * * 1"),
				huh.NewOption("✏️  Custom cron expression", "custom"),
			).
			Value(presetKey),
	)
}

// buildPromptStep builds the prompt text entry step.
func buildPromptStep(prompt *string) *huh.Group {
	return huh.NewGroup(
		huh.NewText().
			Title("📝 Prompt").
			Description("What should Claude do? Be specific: describe the task, expected output, and any constraints.\nTip: For polished prompts, use /schedule add inside Claude Code instead.").
			Placeholder("Review all changed files for security vulnerabilities and generate a report...").
			CharLimit(5000).
			Lines(6).
			Value(prompt).
			Validate(ui.ValidateNonEmpty),
	)
}

// buildModelEffortStep builds the combined model and effort selection step.
func buildModelEffortStep(model, effort *string) *huh.Group {
	return huh.NewGroup(
		huh.NewSelect[string]().
			Title("🧠 Model").
			Description("Which Claude model to use. Sonnet is the best balance of speed and capability for most tasks.").
			Options(
				huh.NewOption("⚡ Sonnet — fast, capable, cost-effective (recommended)", "sonnet"),
				huh.NewOption("🧠 Opus — most capable, best for complex reasoning", "opus"),
				huh.NewOption("🐇 Haiku — fastest, cheapest, good for simple tasks", "haiku"),
			).
			Value(model),
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
			Value(effort),
	)
}

// buildToolsStep builds the allowed tools multi-select step.
// options must be pre-built by the caller (selections differ between add and edit).
func buildToolsStep(allowedTools *[]string, options []huh.Option[string]) *huh.Group {
	return huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("🔧 Allowed Tools").
			Description("Selected tools will be AVAILABLE during job execution. Deselect to deny.").
			Options(options...).
			Value(allowedTools).
			Validate(func(t []string) error {
				if len(t) == 0 {
					return fmt.Errorf("at least one tool must be allowed")
				}
				return nil
			}),
	)
}

// buildContainerStep builds the container runtime selection step.
func buildContainerStep(container, containerImage *string) *huh.Group {
	return huh.NewGroup(
		huh.NewSelect[string]().
			Title("📦 Container Runtime").
			Description("Run the job inside a container for isolation. Requires the runtime to be installed.\n"+
				"The working directory is bind-mounted into the container automatically.").
			Options(
				huh.NewOption("🖥️  None — run directly on host (default)", ""),
				huh.NewOption("🐳 Docker — run inside a Docker container", "docker"),
				huh.NewOption("🦭 Podman — run inside a Podman container (rootless)", "podman"),
			).
			Value(container),
		huh.NewInput().
			Title("🖼️  Container Image").
			Description("Docker/Podman image to use. Must have claude CLI installed.\nRequired when a container runtime is selected above. Ignored when 'None' is selected.").
			Placeholder("claude-runner:latest").
			Value(containerImage).
			Validate(func(s string) error {
				if *container != "" && strings.TrimSpace(s) == "" {
					return fmt.Errorf("container image is required when a container runtime is selected")
				}
				return nil
			}),
	)
}

// buildTimeoutRetryStep builds the timeout, summary, and retry configuration step.
func buildTimeoutRetryStep(timeout, maxRetries, retryBackoff *string, summaryEnabled *bool) *huh.Group {
	return huh.NewGroup(
		huh.NewInput().
			Title("⏱️  Timeout (seconds)").
			Description("Maximum wall-clock time for the job. The process is killed if it exceeds this limit.\n"+
				"Quick: 60-120s. Standard: 300s (5 min). Complex: 600-900s. Enter 0 to disable the timeout. Default: 300.").
			Placeholder("300").
			Value(timeout),
		huh.NewConfirm().
			Title("📊 Enable Report Summarization").
			Description("When enabled, Claude generates a short Telegram-style summary after each run.").
			Value(summaryEnabled),
		huh.NewInput().
			Title("🔁 Max Retries").
			Description("How many times to retry on failure (0 = no retry, max 10). Exponential backoff: 30s→60s→120s…").
			Placeholder("0").
			Value(maxRetries).
			Validate(func(s string) error {
				if s == "" {
					return nil
				}
				n, err := strconv.Atoi(strings.TrimSpace(s))
				if err != nil || n < 0 || n > 10 {
					return fmt.Errorf("must be an integer between 0 and 10")
				}
				return nil
			}),
		huh.NewSelect[string]().
			Title("📈 Retry Backoff").
			Description("Delay strategy between retries.").
			Options(
				huh.NewOption("📈 Exponential — 30s, 60s, 120s… (recommended)", "exponential"),
				huh.NewOption("📏 Linear — 30s, 60s, 90s…", "linear"),
			).
			Value(retryBackoff),
	)
}

// buildOnSuccessStep builds the job-chaining multi-select step.
// options must be pre-built by the caller (depends on existing jobs).
func buildOnSuccessStep(onSuccess *[]string, options []huh.Option[string]) *huh.Group {
	return huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("🔗 On Success: Chain To Jobs").
			Description("Jobs to run automatically when this job completes successfully.\nUseful for multi-step workflows (leave empty to skip).").
			Options(options...).
			Value(onSuccess),
	)
}

// ── Wizard entry points ───────────────────────────────────────────────────────

// RunAddWizard runs the interactive wizard to create a new job.
func RunAddWizard() (*WizardResult, error) {
	var (
		name           string
		workingDir     string
		presetKey      string
		prompt         string
		model          string
		effort         string
		container      string
		containerImage string
		timeout        string
		summaryEnabled bool
		maxRetries     string
		retryBackoff   = "exponential"
		allowedTools   []string
		onSuccess      []string
	)

	fmt.Println(ui.Title.MarginBottom(1).Render("  ✨ Add New Scheduled Job"))

	// Build tool options with default selections.
	var toolOptions []huh.Option[string]
	for _, t := range claudeCodeTools {
		opt := huh.NewOption(t.Label, t.Name)
		if defaultAllowedTools[t.Name] {
			opt = opt.Selected(true)
		}
		toolOptions = append(toolOptions, opt)
	}

	// Build on-success options from enabled jobs.
	existingJobs, _ := config.LoadAllJobs(platform.SchedulesDir())
	var onSuccessOptions []huh.Option[string]
	for _, j := range existingJobs {
		if j.Enabled {
			onSuccessOptions = append(onSuccessOptions, huh.NewOption(j.Name, j.Name))
		}
	}

	const scheduleDesc = "When should this job run? Uses standard cron format (minute hour day-of-month month day-of-week)."
	groups := []*huh.Group{
		buildNameStep(&name, &workingDir),
		buildScheduleStep(&presetKey, scheduleDesc),
		buildPromptStep(&prompt),
		buildModelEffortStep(&model, &effort),
		buildToolsStep(&allowedTools, toolOptions),
		buildContainerStep(&container, &containerImage),
		buildTimeoutRetryStep(&timeout, &maxRetries, &retryBackoff, &summaryEnabled),
	}
	if len(onSuccessOptions) > 0 {
		groups = append(groups, buildOnSuccessStep(&onSuccess, onSuccessOptions))
	}

	form := huh.NewForm(groups...).WithTheme(theme)
	if err := form.Run(); err != nil {
		if IsAborted(err) {
			return nil, nil
		}
		return nil, err
	}

	// Handle custom cron expression.
	schedule, err := runCustomCronForm(presetKey, "*/30 * * * *",
		"Standard 5-field cron: minute hour day-of-month month day-of-week\n"+
			"Examples: */30 * * * * (every 30 min), 0 9,17 * * 1-5 (9am & 5pm weekdays)")
	if err != nil {
		return nil, err
	}
	if schedule == "" {
		if presetKey == "custom" {
			return nil, nil // aborted in custom cron form
		}
		schedule = presetKey
	}

	// Normalize effort: "high" is the default, omit it from config.
	effortVal := effort
	if effortVal == "high" {
		effortVal = ""
	}

	job := &config.JobConfig{
		ID:               uuid.New().String()[:8],
		Name:             name,
		Schedule:         schedule,
		WorkingDir:       workingDir,
		PromptFile:       name + ".md",
		Model:            model,
		Timeout:          parseInt(timeout, 300),
		Effort:           effortVal,
		Container:        container,
		ContainerImage:   containerImage,
		DisallowedTools:  nilIfEmpty(computeDisallowedTools(allowedTools)),
		SummaryEnabled:   summaryEnabled,
		MaxRetries:       parseInt(maxRetries, 0),
		RetryBackoff:     retryBackoffValue(maxRetries, retryBackoff),
		NoSessionPersist: true,
		Enabled:          true,
		OnSuccess:        nilIfEmpty(onSuccess),
	}

	return &WizardResult{
		Job:           job,
		PromptContent: strings.TrimSpace(prompt),
	}, nil
}

// RunEditWizard runs the edit wizard for an existing job.
func RunEditWizard(job *config.JobConfig, existingPrompt string) (*WizardResult, error) {
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
	maxRetries := ""
	if job.MaxRetries > 0 {
		maxRetries = fmt.Sprintf("%d", job.MaxRetries)
	}
	retryBackoff := job.RetryBackoff
	if retryBackoff == "" {
		retryBackoff = "exponential"
	}

	// Build tool options: pre-select tools NOT in the disallowed list.
	disabledSet := make(map[string]bool)
	for _, t := range job.DisallowedTools {
		disabledSet[t] = true
	}
	var allowedTools []string
	var toolOptions []huh.Option[string]
	for _, t := range claudeCodeTools {
		opt := huh.NewOption(t.Label, t.Name)
		if !disabledSet[t.Name] {
			opt = opt.Selected(true)
		}
		toolOptions = append(toolOptions, opt)
	}

	fmt.Println(ui.Title.MarginBottom(1).Render(fmt.Sprintf("  ✏️  Edit Job: %s", job.Name)))

	// Determine if the current schedule matches a preset; if not, show custom.
	presets := []string{"0 * * * *", "0 */6 * * *", "0 2 * * *", "0 9 * * *", "0 9 * * 1"}
	isPreset := false
	for _, p := range presets {
		if job.Schedule == p {
			isPreset = true
			break
		}
	}
	if !isPreset {
		presetKey = "custom"
	}

	// Build on-success options, excluding self and disabled jobs.
	onSuccess := make([]string, len(job.OnSuccess))
	copy(onSuccess, job.OnSuccess)
	currentOnSuccess := make(map[string]bool)
	for _, n := range job.OnSuccess {
		currentOnSuccess[n] = true
	}
	existingJobs, _ := config.LoadAllJobs(platform.SchedulesDir())
	var onSuccessOptions []huh.Option[string]
	for _, j := range existingJobs {
		if j.Name == job.Name || !j.Enabled {
			continue
		}
		opt := huh.NewOption(j.Name, j.Name)
		if currentOnSuccess[j.Name] {
			opt = opt.Selected(true)
		}
		onSuccessOptions = append(onSuccessOptions, opt)
	}

	// Container settings
	container := job.Container
	containerImage := job.ContainerImage

	// keepChain is only consulted when all chain targets are unavailable.
	keepChain := true
	editGroups := []*huh.Group{
		buildScheduleStep(&presetKey, fmt.Sprintf("Current: %s", job.Schedule)),
		buildPromptStep(&prompt),
		buildModelEffortStep(&model, &effort),
		buildToolsStep(&allowedTools, toolOptions),
		buildContainerStep(&container, &containerImage),
		buildTimeoutRetryStep(&timeout, &maxRetries, &retryBackoff, &summaryEnabled),
	}
	if len(onSuccessOptions) > 0 {
		editGroups = append(editGroups, buildOnSuccessStep(&onSuccess, onSuccessOptions))
	} else if len(job.OnSuccess) > 0 {
		// All target jobs are disabled/removed; let the user decide whether to clear the chain.
		step := huh.NewGroup(
			huh.NewConfirm().
				Title("🔗 On Success: Chain Targets Unavailable").
				Description(fmt.Sprintf("This job chains to: %s\nNone of these jobs are currently enabled. Keep the chain configuration?",
					strings.Join(job.OnSuccess, ", "))).
				Value(&keepChain),
		)
		editGroups = append(editGroups, step)
	}

	form := huh.NewForm(editGroups...).WithTheme(theme)
	if err := form.Run(); err != nil {
		if IsAborted(err) {
			return nil, nil
		}
		return nil, err
	}

	// Handle custom cron expression.
	schedule, err := runCustomCronForm(presetKey, job.Schedule, "5-field cron: minute hour day-of-month month day-of-week")
	if err != nil {
		return nil, err
	}
	if schedule == "" {
		if presetKey == "custom" {
			return nil, nil // aborted in custom cron form
		}
		schedule = presetKey
	}

	// Normalize effort: "high" is the default, omit it from config.
	effortVal := effort
	if effortVal == "high" {
		effortVal = ""
	}

	// Compute disallowed from allowed selection, preserving custom patterns from YAML.
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

	// If all chain targets were unavailable and user chose not to keep them, clear the list.
	if len(onSuccessOptions) == 0 && !keepChain {
		onSuccess = nil
	}

	// Update job config (keep ID, Name, WorkingDir, PromptFile).
	job.Schedule = schedule
	job.Model = model
	job.Effort = effortVal
	job.Timeout = parseInt(timeout, 300)
	job.Container = container
	job.ContainerImage = containerImage
	job.DisallowedTools = nilIfEmpty(disallowed)
	job.SummaryEnabled = summaryEnabled
	job.MaxRetries = parseInt(maxRetries, 0)
	job.RetryBackoff = retryBackoffValue(maxRetries, retryBackoff)
	job.OnSuccess = nilIfEmpty(onSuccess)

	return &WizardResult{
		Job:           job,
		PromptContent: strings.TrimSpace(prompt),
	}, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// runCustomCronForm shows an input form for a custom cron expression when
// presetKey == "custom". Returns the entered cron string, or "" if aborted or
// presetKey is not "custom".
func runCustomCronForm(presetKey, initialValue, description string) (string, error) {
	if presetKey != "custom" {
		return "", nil
	}
	customCron := initialValue
	customForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("✏️  Custom Cron Expression").
				Description(description).
				Placeholder("*/30 * * * *").
				Value(&customCron).
				Validate(ui.ValidateCron),
		),
	).WithTheme(theme)

	if err := customForm.Run(); err != nil {
		if IsAborted(err) {
			return "", nil
		}
		return "", err
	}
	return customCron, nil
}

func parseInt(s string, defaultVal int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return defaultVal
	}
	i, err := strconv.Atoi(s)
	if err != nil {
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

// retryBackoffValue returns the backoff strategy to store in config.
// Returns config.BackoffExponential ("") when no retries are configured or
// when the strategy is exponential (the default), keeping the YAML clean.
func retryBackoffValue(maxRetriesStr, backoff string) string {
	if parseInt(maxRetriesStr, 0) == 0 {
		return config.BackoffExponential
	}
	if backoff == config.BackoffLinear {
		return config.BackoffLinear
	}
	return config.BackoffExponential
}
