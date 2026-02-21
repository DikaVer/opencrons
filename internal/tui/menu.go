// Package tui provides interactive terminal UI components built on charmbracelet/huh.
//
// menu.go implements the main TUI menu system. It defines MenuAction and JobAction
// enums for routing user selections, and provides functions for displaying the main
// menu with a status bar, job picker, per-job action menu, daemon control menu,
// log source picker, yes/no confirmation prompts, and debug toggle. The
// printQuickStatus function renders a compact overview of daemon state, job count,
// messenger connectivity, and next scheduled run. PrintPressEnter blocks until the
// user acknowledges output.
package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/DikaVer/opencrons/internal/config"
	"github.com/DikaVer/opencrons/internal/platform"
	"github.com/DikaVer/opencrons/internal/ui"
)

// MenuAction represents what the user chose from the main menu.
type MenuAction int

const (
	MenuAddJob MenuAction = iota
	MenuManageJobs
	MenuRunJob
	MenuViewLogs
	MenuDaemonControl
	MenuSettings
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

// PrintHeader renders the app header with status bar and an optional page title.
// If page is empty, no breadcrumb is shown (main menu mode).
func PrintHeader(page string) {
	ClearScreen()
	fmt.Println()
	fmt.Println(ui.Title.Render("  🚀 OpenCron — Claude Code Automation"))
	fmt.Println(ui.Dim.Render("  Schedule and manage automated Claude Code tasks"))
	fmt.Println()
	printQuickStatus()
	if page != "" {
		fmt.Println()
		fmt.Println(ui.Title.Render("  " + page))
		fmt.Println()
	}
}

// RunMainMenu shows the main TUI menu and returns the selected action.
func RunMainMenu() (MenuAction, error) {
	PrintHeader("")
	fmt.Println()

	var choice int

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title("What would you like to do?").
				Options(
					huh.NewOption("➕ Add new job", int(MenuAddJob)),
					huh.NewOption("📂 Manage jobs", int(MenuManageJobs)),
					huh.NewOption("▶️  Run job now", int(MenuRunJob)),
					huh.NewOption("📜 View logs", int(MenuViewLogs)),
					huh.NewOption("🤖 Daemon", int(MenuDaemonControl)),
					huh.NewOption("⚙️  Settings", int(MenuSettings)),
					huh.NewOption("👋 Exit", int(MenuExit)),
				).
				Value(&choice),
		),
	).WithTheme(theme)

	if err := form.Run(); err != nil {
		if IsAborted(err) {
			return MenuExit, nil
		}
		return MenuExit, err
	}

	return MenuAction(choice), nil
}

// RunJobPicker shows a list of jobs and returns the selected job name.
// Returns "__add__" if user chose to add a new job.
// Returns empty string if user chose back, pressed Escape, or there are no jobs.
func RunJobPicker(title, description string) (string, error) {
	PrintHeader("📂 Manage Jobs")

	jobs, err := config.LoadAllJobs(platform.SchedulesDir())
	if err != nil {
		return "", fmt.Errorf("loading jobs: %w", err)
	}

	var options []huh.Option[string]
	for _, j := range jobs {
		status := "✅"
		if !j.Enabled {
			status = "⏸️"
		}
		label := fmt.Sprintf("%s %-20s  %-16s", status, j.Name, j.Schedule)
		options = append(options, huh.NewOption(label, j.Name))
	}
	options = append(options, huh.NewOption("➕ Add new job", "__add__"))
	options = append(options, huh.NewOption("◀️  Back to menu", "__back__"))

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
		if IsAborted(err) {
			return "", nil
		}
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

	PrintHeader(fmt.Sprintf("📋 Job: %s", job.Name))
	fmt.Printf("  %s %s\n", ui.Dim.Render("⏰ Schedule:"), job.Schedule)
	fmt.Printf("  %s %s\n", ui.Dim.Render("🧠 Model:"), job.Model)
	fmt.Printf("  %s %s\n", ui.Dim.Render("📁 Directory:"), job.WorkingDir)
	if job.Enabled {
		fmt.Printf("  %s %s\n", ui.Dim.Render("Status:"), ui.Success.Render("✅ enabled"))
	} else {
		fmt.Printf("  %s %s\n", ui.Dim.Render("Status:"), ui.Fail.Render("⏸️  disabled"))
	}
	fmt.Println()

	var choice int

	// Build options based on current state
	var options []huh.Option[int]
	options = append(options, huh.NewOption("✏️  Edit", int(JobActionEdit)))
	options = append(options, huh.NewOption("▶️  Run now", int(JobActionRun)))
	options = append(options, huh.NewOption("📊 Usage", int(JobActionUsage)))
	if job.Enabled {
		options = append(options, huh.NewOption("⏸️  Disable", int(JobActionDisable)))
	} else {
		options = append(options, huh.NewOption("✅ Enable", int(JobActionEnable)))
	}
	options = append(options, huh.NewOption("🗑️  Remove", int(JobActionRemove)))
	options = append(options, huh.NewOption("◀️  Back", int(JobActionBack)))

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title("What would you like to do?").
				Options(options...).
				Value(&choice),
		),
	).WithTheme(theme)

	if err := form.Run(); err != nil {
		if IsAborted(err) {
			return JobActionBack, nil
		}
		return JobActionBack, err
	}

	return JobAction(choice), nil
}

// RunDaemonMenu shows daemon control options.
func RunDaemonMenu() (string, error) {
	PrintHeader("🤖 Daemon Control")

	// Show daemon status
	pid, running := platform.CheckDaemonRunning()
	if running {
		fmt.Printf("  Status: %s (PID %d)\n", ui.Success.Render("🟢 running"), pid)
	} else {
		fmt.Printf("  Status: %s\n", ui.Fail.Render("🔴 stopped"))
	}
	fmt.Println()

	// Explain what the daemon does
	fmt.Println(ui.Dim.Render("  The daemon is a background process that watches your job schedules"))
	fmt.Println(ui.Dim.Render("  and runs them automatically at the configured times. It also"))
	fmt.Println(ui.Dim.Render("  hot-reloads when you edit job configs — no restart needed."))
	fmt.Println()
	fmt.Println(ui.Dim.Render("  ▶️  Start:          Run the daemon in the current terminal (foreground)."))
	fmt.Println(ui.Dim.Render("                      Press Ctrl+C to stop it."))
	fmt.Println()
	fmt.Println(ui.Dim.Render("  🔧 Install service: Register as a system service so it starts"))
	fmt.Println(ui.Dim.Render("                      automatically on boot. On Windows this creates a"))
	fmt.Println(ui.Dim.Render("                      Windows Service; on Linux, a systemd unit."))
	fmt.Println(ui.Dim.Render("                      Requires administrator/root privileges."))
	fmt.Println()

	var choice string
	var options []huh.Option[string]

	if running {
		options = append(options, huh.NewOption("⏹️  Stop daemon", "stop"))
	} else {
		options = append(options, huh.NewOption("▶️  Start daemon", "start"))
		options = append(options, huh.NewOption("🔧 Install as service", "install"))
	}
	options = append(options, huh.NewOption("📜 View logs", "logs"))
	options = append(options, huh.NewOption("◀️  Back", "back"))

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Daemon action").
				Options(options...).
				Value(&choice),
		),
	).WithTheme(theme)

	if err := form.Run(); err != nil {
		if IsAborted(err) {
			return "back", nil
		}
		return "back", err
	}

	return choice, nil
}

// RunLogsPicker lets the user choose which logs to view.
func RunLogsPicker() (string, error) {
	PrintHeader("📜 View Logs")

	jobs, err := config.LoadAllJobs(platform.SchedulesDir())
	if err != nil {
		return "", fmt.Errorf("loading jobs: %w", err)
	}

	var options []huh.Option[string]
	options = append(options, huh.NewOption("📋 All jobs", "__all__"))
	for _, j := range jobs {
		options = append(options, huh.NewOption(fmt.Sprintf("📄 %s", j.Name), j.Name))
	}
	options = append(options, huh.NewOption("◀️  Back", "__back__"))

	var name string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("📜 View logs for").
				Options(options...).
				Value(&name),
		),
	).WithTheme(theme)

	if err := form.Run(); err != nil {
		if IsAborted(err) {
			return "", nil
		}
		return "", err
	}

	if name == "__back__" {
		return "", nil
	}
	return name, nil
}

// ConfirmAction asks for yes/no confirmation. Escape returns false.
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
		if IsAborted(err) {
			return false, nil
		}
		return false, err
	}

	return confirm, nil
}

func printQuickStatus() {
	// Daemon status
	_, running := platform.CheckDaemonRunning()
	if running {
		fmt.Printf("  %s %s", ui.Dim.Render("Daemon:"), ui.Success.Render("🟢 running"))
	} else {
		fmt.Printf("  %s %s", ui.Dim.Render("Daemon:"), ui.Fail.Render("🔴 stopped"))
	}

	// Job count
	jobs, _ := config.LoadAllJobs(platform.SchedulesDir())
	enabledCount := 0
	for _, j := range jobs {
		if j.Enabled {
			enabledCount++
		}
	}
	fmt.Printf("    %s %d total, %d enabled", ui.Dim.Render("📋 Jobs:"), len(jobs), enabledCount)

	// Messenger status
	msgCfg := platform.GetMessengerConfig()
	if msgCfg != nil {
		if running {
			fmt.Printf("    %s %s", ui.Dim.Render("💬 Chat:"), ui.Success.Render(msgCfg.Type))
		} else {
			fmt.Printf("    %s %s", ui.Dim.Render("💬 Chat:"), ui.Dim.Render(msgCfg.Type+" (start daemon to chat)"))
		}
	}

	// Next run
	if enabledCount > 0 {
		now := time.Now()
		var nextRun time.Time
		var nextJob string
		for _, j := range jobs {
			if !j.Enabled {
				continue
			}
			sched, err := ui.CronParser.Parse(j.Schedule)
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
			fmt.Printf("    %s %s in %s", ui.Dim.Render("⏰ Next:"), nextJob, until)
		}
	}
	fmt.Println()
}

// RunDebugMenu shows the debug toggle menu.
func RunDebugMenu(debugEnabled bool) (bool, error) {
	PrintHeader("🐛 Debug Logging")

	if debugEnabled {
		fmt.Printf("  Current state: %s\n", ui.Success.Render("✅ on"))
	} else {
		fmt.Printf("  Current state: %s\n", ui.Fail.Render("❌ off"))
	}
	fmt.Println(ui.Dim.Render("  When enabled, detailed logs are written to logs/opencrons-debug.log"))
	fmt.Println()

	action := "toggle"
	if debugEnabled {
		action = "Disable debug logging"
	} else {
		action = "Enable debug logging"
	}

	confirmed, err := ConfirmAction(action, "")
	if err != nil {
		return debugEnabled, err
	}

	if confirmed {
		return !debugEnabled, nil
	}
	return debugEnabled, nil
}

// PrintPressEnter prints a "press Enter to continue" prompt and waits.
func PrintPressEnter() {
	fmt.Print(ui.Dim.Render("\n  ⏎ Press Enter to continue..."))
	fmt.Scanln()
}
