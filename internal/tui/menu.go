package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/dika-maulidal/cli-scheduler/internal/config"
	"github.com/dika-maulidal/cli-scheduler/internal/platform"
	"github.com/robfig/cron/v3"
)

// MenuAction represents what the user chose from the main menu.
type MenuAction int

const (
	MenuAddJob MenuAction = iota
	MenuManageJobs
	MenuRunJob
	MenuViewLogs
	MenuDaemonControl
	MenuExit
)

// JobAction represents what the user wants to do with a selected job.
type JobAction int

const (
	JobActionEdit JobAction = iota
	JobActionRun
	JobActionUsage
	JobActionDisable
	JobActionEnable
	JobActionRemove
	JobActionBack
)

// RunMainMenu shows the main TUI menu and returns the selected action.
func RunMainMenu() (MenuAction, error) {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#cba6f7"))
	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6c7086"))

	// Show header
	fmt.Println()
	fmt.Println(titleStyle.Render("  CLI Scheduler — Claude Code Automation"))
	fmt.Println(dimStyle.Render("  Schedule and manage automated Claude Code tasks"))
	fmt.Println()

	// Show quick status
	printQuickStatus()
	fmt.Println()

	var choice int

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title("What would you like to do?").
				Description("Use arrow keys to navigate, Enter to select.").
				Options(
					huh.NewOption("Add new job             Create a new scheduled task", int(MenuAddJob)),
					huh.NewOption("Manage jobs             View, edit, enable/disable, or remove jobs", int(MenuManageJobs)),
					huh.NewOption("Run job now             Execute a job immediately", int(MenuRunJob)),
					huh.NewOption("View logs               See execution history", int(MenuViewLogs)),
					huh.NewOption("Daemon                  Start, stop, or check daemon status", int(MenuDaemonControl)),
					huh.NewOption("Exit", int(MenuExit)),
				).
				Value(&choice),
		),
	).WithTheme(theme)

	if err := form.Run(); err != nil {
		return MenuExit, err
	}

	return MenuAction(choice), nil
}

// RunJobPicker shows a list of jobs and returns the selected job name.
// Returns "__add__" if user chose to add a new job.
// Returns empty string if user chose back or there are no jobs.
func RunJobPicker(title, description string) (string, error) {
	jobs, err := config.LoadAllJobs(platform.SchedulesDir())
	if err != nil {
		return "", fmt.Errorf("loading jobs: %w", err)
	}

	var options []huh.Option[string]
	for _, j := range jobs {
		status := "enabled"
		if !j.Enabled {
			status = "disabled"
		}
		label := fmt.Sprintf("%-20s  %-16s  %s", j.Name, j.Schedule, status)
		options = append(options, huh.NewOption(label, j.Name))
	}
	options = append(options, huh.NewOption("+ Add new job", "__add__"))
	options = append(options, huh.NewOption("<< Back to menu", "__back__"))

	var name string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(title).
				Description(description).
				Options(options...).
				Value(&name),
		),
	).WithTheme(theme)

	if err := form.Run(); err != nil {
		return "", err
	}

	if name == "__back__" {
		return "", nil
	}
	return name, nil
}

// RunJobActionMenu shows actions available for a selected job.
func RunJobActionMenu(jobName string) (JobAction, error) {
	job, err := config.FindJobByName(platform.SchedulesDir(), jobName)
	if err != nil {
		return JobActionBack, err
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#cba6f7"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))

	fmt.Println()
	fmt.Println(titleStyle.Render(fmt.Sprintf("  Job: %s", job.Name)))
	fmt.Printf("  %s %s\n", dimStyle.Render("Schedule:"), job.Schedule)
	fmt.Printf("  %s %s\n", dimStyle.Render("Model:"), job.Model)
	fmt.Printf("  %s %s\n", dimStyle.Render("Directory:"), job.WorkingDir)
	if job.Enabled {
		fmt.Printf("  %s enabled\n", dimStyle.Render("Status:"))
	} else {
		fmt.Printf("  %s disabled\n", dimStyle.Render("Status:"))
	}
	fmt.Println()

	var choice int

	// Build options based on current state
	var options []huh.Option[int]
	options = append(options, huh.NewOption("Edit                   Modify schedule, prompt, model, etc.", int(JobActionEdit)))
	options = append(options, huh.NewOption("Run now                Execute this job immediately", int(JobActionRun)))
	options = append(options, huh.NewOption("Usage                  Token usage and cost per run", int(JobActionUsage)))
	if job.Enabled {
		options = append(options, huh.NewOption("Disable                Pause this job (keep config)", int(JobActionDisable)))
	} else {
		options = append(options, huh.NewOption("Enable                 Resume running on schedule", int(JobActionEnable)))
	}
	options = append(options, huh.NewOption("Remove                 Delete this job and its prompt file", int(JobActionRemove)))
	options = append(options, huh.NewOption("<< Back", int(JobActionBack)))

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title("What would you like to do with this job?").
				Description("Use arrow keys to navigate, Enter to select.").
				Options(options...).
				Value(&choice),
		),
	).WithTheme(theme)

	if err := form.Run(); err != nil {
		return JobActionBack, err
	}

	return JobAction(choice), nil
}

// RunDaemonMenu shows daemon control options.
func RunDaemonMenu() (string, error) {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#cba6f7"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1"))
	failStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8"))

	fmt.Println()
	fmt.Println(titleStyle.Render("  Daemon Control"))
	fmt.Println()

	// Show daemon status
	pid, running := platform.CheckDaemonRunning()
	if running {
		fmt.Printf("  Status: %s (PID %d)\n", successStyle.Render("running"), pid)
	} else {
		fmt.Printf("  Status: %s\n", failStyle.Render("stopped"))
	}
	fmt.Println()

	// Explain what the daemon does
	fmt.Println(dimStyle.Render("  The daemon is a background process that watches your job schedules"))
	fmt.Println(dimStyle.Render("  and runs them automatically at the configured times. It also"))
	fmt.Println(dimStyle.Render("  hot-reloads when you edit job configs — no restart needed."))
	fmt.Println()
	fmt.Println(dimStyle.Render("  Start:           Run the daemon in the current terminal (foreground)."))
	fmt.Println(dimStyle.Render("                   Press Ctrl+C to stop it."))
	fmt.Println()
	fmt.Println(dimStyle.Render("  Install service: Register as a system service so it starts"))
	fmt.Println(dimStyle.Render("                   automatically on boot. On Windows this creates a"))
	fmt.Println(dimStyle.Render("                   Windows Service; on Linux, a systemd unit."))
	fmt.Println(dimStyle.Render("                   Requires administrator/root privileges."))
	fmt.Println()

	var choice string
	var options []huh.Option[string]

	if running {
		options = append(options, huh.NewOption("Stop daemon            Shut down the background scheduler", "stop"))
	} else {
		options = append(options, huh.NewOption("Start daemon           Run scheduler in this terminal (Ctrl+C to stop)", "start"))
		options = append(options, huh.NewOption("Install as service     Auto-start on boot (requires admin)", "install"))
	}
	options = append(options, huh.NewOption("<< Back", "back"))

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Daemon action").
				Description("Use arrow keys to navigate, Enter to select.").
				Options(options...).
				Value(&choice),
		),
	).WithTheme(theme)

	if err := form.Run(); err != nil {
		return "back", err
	}

	return choice, nil
}

// RunLogsPicker lets the user choose which logs to view.
func RunLogsPicker() (string, error) {
	jobs, err := config.LoadAllJobs(platform.SchedulesDir())
	if err != nil {
		return "", fmt.Errorf("loading jobs: %w", err)
	}

	var options []huh.Option[string]
	options = append(options, huh.NewOption("All jobs               Show recent logs across all jobs", "__all__"))
	for _, j := range jobs {
		label := fmt.Sprintf("%-20s  Show logs for this job only", j.Name)
		options = append(options, huh.NewOption(label, j.Name))
	}
	options = append(options, huh.NewOption("<< Back", "__back__"))

	var name string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("View logs for").
				Description("Use arrow keys to navigate, Enter to select.").
				Options(options...).
				Value(&name),
		),
	).WithTheme(theme)

	if err := form.Run(); err != nil {
		return "", err
	}

	if name == "__back__" {
		return "", nil
	}
	return name, nil
}

// ConfirmAction asks for yes/no confirmation.
func ConfirmAction(title, description string) (bool, error) {
	var confirm bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(title).
				Description(description).
				Value(&confirm),
		),
	).WithTheme(theme)

	if err := form.Run(); err != nil {
		return false, err
	}

	return confirm, nil
}

func printQuickStatus() {
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1"))
	failStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8"))

	// Daemon status
	_, running := platform.CheckDaemonRunning()
	if running {
		fmt.Printf("  %s %s", dimStyle.Render("Daemon:"), successStyle.Render("running"))
	} else {
		fmt.Printf("  %s %s", dimStyle.Render("Daemon:"), failStyle.Render("stopped"))
	}

	// Job count
	jobs, _ := config.LoadAllJobs(platform.SchedulesDir())
	enabledCount := 0
	for _, j := range jobs {
		if j.Enabled {
			enabledCount++
		}
	}
	fmt.Printf("    %s %d total, %d enabled", dimStyle.Render("Jobs:"), len(jobs), enabledCount)

	// Next run
	if enabledCount > 0 {
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		now := time.Now()
		var nextRun time.Time
		var nextJob string
		for _, j := range jobs {
			if !j.Enabled {
				continue
			}
			sched, err := parser.Parse(j.Schedule)
			if err != nil {
				continue
			}
			t := sched.Next(now)
			if nextRun.IsZero() || t.Before(nextRun) {
				nextRun = t
				nextJob = j.Name
			}
		}
		if !nextRun.IsZero() {
			until := time.Until(nextRun).Round(time.Minute)
			fmt.Printf("    %s %s in %s", dimStyle.Render("Next:"), nextJob, until)
		}
	}
	fmt.Println()
}

// PrintPressEnter prints a "press Enter to continue" prompt and waits.
func PrintPressEnter() {
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	fmt.Print(dimStyle.Render("\n  Press Enter to continue..."))
	fmt.Scanln()
}
