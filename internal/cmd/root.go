package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dika-maulidal/cli-scheduler/internal/config"
	"github.com/dika-maulidal/cli-scheduler/internal/daemon"
	"github.com/dika-maulidal/cli-scheduler/internal/executor"
	"github.com/dika-maulidal/cli-scheduler/internal/platform"
	"github.com/dika-maulidal/cli-scheduler/internal/storage"
	"github.com/dika-maulidal/cli-scheduler/internal/tui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "scheduler",
	Short: "CLI scheduler for Claude Code automation",
	Long:  "A scheduler that runs Claude Code tasks on a cron schedule with secure, predefined execution environments.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip setup check for these commands
		name := cmd.Name()
		if name == "setup" || name == "help" || name == "version" {
			return nil
		}

		if !platform.IsSetupComplete() {
			fmt.Println()
			fmt.Println("  First-time setup required. Starting setup wizard...")
			fmt.Println()
			return runSetupWizard()
		}
		return nil
	},
	RunE: runMainMenu,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
}

func runMainMenu(cmd *cobra.Command, args []string) error {
	if err := platform.EnsureDirs(); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	for {
		action, err := tui.RunMainMenu()
		if err != nil {
			return err
		}

		switch action {
		case tui.MenuAddJob:
			if err := runAddInteractive(); err != nil {
				fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
			}
			tui.PrintPressEnter()

		case tui.MenuManageJobs:
			handleManageJobs()

		case tui.MenuRunJob:
			handleRunFromMenu()

		case tui.MenuViewLogs:
			handleViewLogs()

		case tui.MenuDaemonControl:
			handleDaemonMenu()

		case tui.MenuSettings:
			handleSettingsMenu()

		case tui.MenuExit:
			return nil
		}
	}
}

func handleManageJobs() {
	for {
		name, err := tui.RunJobPicker(
			"Select a job to manage",
			"Use arrow keys to navigate, Enter to select.",
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
			return
		}
		if name == "" {
			return // back to main menu
		}
		if name == "__add__" {
			if err := runAddInteractive(); err != nil {
				fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
			}
			tui.PrintPressEnter()
			continue
		}

		action, err := tui.RunJobActionMenu(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
			return
		}

		switch action {
		case tui.JobActionEdit:
			if err := editJob(name); err != nil {
				fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
			}
			tui.PrintPressEnter()

		case tui.JobActionRun:
			runJobByName(name)
			tui.PrintPressEnter()

		case tui.JobActionUsage:
			showJobUsage(name)
			tui.PrintPressEnter()

		case tui.JobActionDisable:
			if err := disableJob(name); err != nil {
				fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
			}
			tui.PrintPressEnter()

		case tui.JobActionEnable:
			if err := enableJob(name); err != nil {
				fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
			}
			tui.PrintPressEnter()

		case tui.JobActionRemove:
			confirmed, err := tui.ConfirmAction(
				fmt.Sprintf("Remove job %q?", name),
				"This will delete the job config and prompt file. This cannot be undone.",
			)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
				continue
			}
			if confirmed {
				if err := config.DeleteJob(platform.SchedulesDir(), platform.PromptsDir(), name, true); err != nil {
					fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
				} else {
					fmt.Printf("  Job %q removed.\n", name)
				}
			} else {
				fmt.Println("  Cancelled.")
			}
			tui.PrintPressEnter()

		case tui.JobActionBack:
			continue
		}
	}
}

func handleRunFromMenu() {
	name, err := tui.RunJobPicker(
		"Select a job to run now",
		"The job will execute immediately, bypassing its schedule.",
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
		return
	}
	if name == "" {
		return
	}
	if name == "__add__" {
		if err := runAddInteractive(); err != nil {
			fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
		}
		tui.PrintPressEnter()
		return
	}

	runJobByName(name)
	tui.PrintPressEnter()
}

func runJobByName(name string) {
	job, err := config.FindJobByName(platform.SchedulesDir(), name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
		return
	}

	if err := job.ValidatePromptFileExists(platform.PromptsDir()); err != nil {
		fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
		return
	}

	db, err := storage.Open(platform.DBPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Error opening database: %v\n", err)
		return
	}
	defer db.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	fmt.Printf("  Running job %q...\n", name)

	result, err := executor.Run(ctx, db, job, "manual")
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Execution failed: %v\n", err)
		return
	}

	fmt.Printf("\n  Status:   %s\n", result.Status)
	fmt.Printf("  Duration: %s\n", result.Duration.Round(1e9))
	fmt.Printf("  Exit:     %d\n", result.ExitCode)
	if result.CostUSD > 0 {
		fmt.Printf("  Cost:     $%.4f\n", result.CostUSD)
	}
	if result.InputTokens > 0 || result.OutputTokens > 0 {
		fmt.Printf("  Tokens:   %s in / %s out / %s cache read / %s cache write\n",
			formatTokens(result.InputTokens), formatTokens(result.OutputTokens),
			formatTokens(result.CacheReadTokens), formatTokens(result.CacheCreationTokens))
	}
	if result.StdoutPath != "" {
		fmt.Printf("  Output:   %s\n", result.StdoutPath)
	}
	if result.ErrorMsg != "" {
		fmt.Printf("  Error:    %s\n", result.ErrorMsg)
	}
}

func handleViewLogs() {
	choice, err := tui.RunLogsPicker()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
		return
	}
	if choice == "" {
		return
	}

	db, err := storage.Open(platform.DBPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Error opening database: %v\n", err)
		return
	}
	defer db.Close()

	var logs []storage.ExecutionLog
	if choice == "__all__" {
		logs, err = db.GetRecentLogs(20)
	} else {
		logs, err = db.GetLogsByJobName(choice, 20)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Error querying logs: %v\n", err)
		return
	}

	if len(logs) == 0 {
		fmt.Println("  No execution logs found.")
		tui.PrintPressEnter()
		return
	}

	printLogsTable(logs)
	tui.PrintPressEnter()
}

// printLogsTable renders a styled table of execution logs.
func printLogsTable(logs []storage.ExecutionLog) {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#cba6f7"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1"))
	failStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#fab387"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))

	fmt.Fprintf(os.Stdout, "  %s  %s  %s  %s  %s  %s\n",
		headerStyle.Width(20).Render("JOB"),
		headerStyle.Width(20).Render("STARTED"),
		headerStyle.Width(10).Render("STATUS"),
		headerStyle.Width(8).Render("TRIGGER"),
		headerStyle.Width(10).Render("COST"),
		headerStyle.Width(16).Render("TOKENS (I/O)"),
	)
	fmt.Println(dimStyle.Render("  " + strings.Repeat("-", 90)))

	for _, log := range logs {
		var statusStr string
		switch log.Status {
		case "success":
			statusStr = successStyle.Width(10).Render("success")
		case "failed":
			statusStr = failStyle.Width(10).Render("failed")
		case "timeout":
			statusStr = warnStyle.Width(10).Render("timeout")
		case "running":
			statusStr = warnStyle.Width(10).Render("running")
		case "cancelled":
			statusStr = dimStyle.Width(10).Render("cancelled")
		default:
			statusStr = fmt.Sprintf("%-10s", log.Status)
		}

		costStr := "-"
		if log.CostUSD != nil && *log.CostUSD > 0 {
			costStr = fmt.Sprintf("$%.4f", *log.CostUSD)
		}

		tokensStr := "-"
		if log.InputTokens != nil && log.OutputTokens != nil {
			tokensStr = fmt.Sprintf("%s/%s", formatTokens(*log.InputTokens), formatTokens(*log.OutputTokens))
		}

		startedStr := log.StartedAt.Format("2006-01-02 15:04:05")

		fmt.Fprintf(os.Stdout, "  %-20s  %-20s  %s  %-8s  %-10s  %s\n",
			truncateStr(log.JobName, 20),
			startedStr,
			statusStr,
			log.TriggerType,
			costStr,
			tokensStr,
		)

		if log.ErrorMsg != "" {
			errLines := strings.Split(log.ErrorMsg, "\n")
			fmt.Fprintf(os.Stdout, "  %s %s\n", dimStyle.Render("Error:"), truncateStr(errLines[0], 70))
		}
	}

	fmt.Printf("\n  %s\n", dimStyle.Render(fmt.Sprintf("Showing %d entries", len(logs))))
}

// showJobUsage displays per-run token usage and cost for a specific job.
func showJobUsage(jobName string) {
	db, err := storage.Open(platform.DBPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Error opening database: %v\n", err)
		return
	}
	defer db.Close()

	logs, err := db.GetLogsByJobName(jobName, 20)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Error querying logs: %v\n", err)
		return
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#cba6f7"))
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#cba6f7"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	accentStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f5c2e7"))

	fmt.Println()
	fmt.Println(titleStyle.Render(fmt.Sprintf("  Usage: %s", jobName)))
	fmt.Println()

	if len(logs) == 0 {
		fmt.Println("  No execution history found.")
		return
	}

	// Per-run table
	fmt.Fprintf(os.Stdout, "  %s  %s  %s  %s  %s  %s  %s\n",
		headerStyle.Width(20).Render("DATE"),
		headerStyle.Width(8).Render("STATUS"),
		headerStyle.Width(10).Render("INPUT"),
		headerStyle.Width(10).Render("OUTPUT"),
		headerStyle.Width(12).Render("CACHE READ"),
		headerStyle.Width(12).Render("CACHE WRITE"),
		headerStyle.Width(10).Render("COST"),
	)
	fmt.Println(dimStyle.Render("  " + strings.Repeat("-", 88)))

	for _, log := range logs {
		dateStr := log.StartedAt.Format("2006-01-02 15:04:05")

		statusStr := log.Status
		inputStr := "-"
		outputStr := "-"
		cacheReadStr := "-"
		cacheWriteStr := "-"
		costStr := "-"

		if log.InputTokens != nil {
			inputStr = formatTokens(*log.InputTokens)
		}
		if log.OutputTokens != nil {
			outputStr = formatTokens(*log.OutputTokens)
		}
		if log.CacheReadTokens != nil {
			cacheReadStr = formatTokens(*log.CacheReadTokens)
		}
		if log.CacheCreationTokens != nil {
			cacheWriteStr = formatTokens(*log.CacheCreationTokens)
		}
		if log.CostUSD != nil && *log.CostUSD > 0 {
			costStr = fmt.Sprintf("$%.4f", *log.CostUSD)
		}

		fmt.Fprintf(os.Stdout, "  %-20s  %-8s  %10s  %10s  %12s  %12s  %10s\n",
			dateStr, statusStr, inputStr, outputStr, cacheReadStr, cacheWriteStr, costStr)
	}

	// Totals
	usage, err := db.GetUsageByJobName(jobName)
	if err != nil {
		return
	}

	fmt.Println(dimStyle.Render("  " + strings.Repeat("-", 88)))
	fmt.Fprintf(os.Stdout, "  %-20s  %-8s  %10s  %10s  %12s  %12s  %s\n",
		accentStyle.Render(fmt.Sprintf("TOTAL (%d runs)", usage.TotalRuns)),
		"",
		formatTokens(usage.TotalInputTokens),
		formatTokens(usage.TotalOutputTokens),
		formatTokens(usage.TotalCacheRead),
		formatTokens(usage.TotalCacheCreation),
		accentStyle.Render(fmt.Sprintf("$%.4f", usage.TotalCostUSD)),
	)
	fmt.Println()
}

func formatTokens(n int) string {
	if n == 0 {
		return "0"
	}
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func truncateStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
}

func handleDebugMenu() {
	current := platform.IsDebugEnabled()
	newState, err := tui.RunDebugMenu(current)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
		tui.PrintPressEnter()
		return
	}

	if newState != current {
		if err := platform.SetDebug(newState); err != nil {
			fmt.Fprintf(os.Stderr, "  Error saving settings: %v\n", err)
		} else if newState {
			fmt.Println("  Debug logging enabled.")
			fmt.Printf("  Logs: %s/scheduler-debug.log\n", platform.LogsDir())
		} else {
			fmt.Println("  Debug logging disabled.")
		}
	} else {
		fmt.Println("  No change.")
	}
	tui.PrintPressEnter()
}

func handleDaemonMenu() {
	choice, err := tui.RunDaemonMenu()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
		return
	}

	switch choice {
	case "start":
		fmt.Println("  Starting scheduler daemon... (press Ctrl+C to stop)")
		fmt.Println()
		if err := daemon.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "  Daemon error: %v\n", err)
		}
	case "stop":
		runStop(nil, nil)
	case "install":
		if err := daemon.InstallService(); err != nil {
			fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
		}
		tui.PrintPressEnter()
	case "back":
		return
	}
}

