// File menu_logs.go implements TUI log viewing handlers including handleViewLogs,
// printLogsTable (styled execution log table), and showJobUsage which displays
// per-run token usage with totals.
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/DikaVer/opencron/internal/platform"
	"github.com/DikaVer/opencron/internal/storage"
	"github.com/DikaVer/opencron/internal/tui"
	"github.com/DikaVer/opencron/internal/ui"
)

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
	fmt.Fprintf(os.Stdout, "  %s  %s  %s  %s  %s  %s\n",
		ui.Title.Width(20).Render("JOB"),
		ui.Title.Width(20).Render("STARTED"),
		ui.Title.Width(10).Render("STATUS"),
		ui.Title.Width(8).Render("TRIGGER"),
		ui.Title.Width(10).Render("COST"),
		ui.Title.Width(16).Render("TOKENS (I/O)"),
	)
	fmt.Println(ui.Dim.Render("  " + strings.Repeat("-", 90)))

	for _, log := range logs {
		var statusStr string
		switch log.Status {
		case "success":
			statusStr = ui.Success.Width(10).Render("success")
		case "failed":
			statusStr = ui.Fail.Width(10).Render("failed")
		case "timeout":
			statusStr = ui.Warn.Width(10).Render("timeout")
		case "running":
			statusStr = ui.Warn.Width(10).Render("running")
		case "cancelled":
			statusStr = ui.Dim.Width(10).Render("cancelled")
		default:
			statusStr = fmt.Sprintf("%-10s", log.Status)
		}

		costStr := "-"
		if log.CostUSD != nil && *log.CostUSD > 0 {
			costStr = fmt.Sprintf("$%.4f", *log.CostUSD)
		}

		tokensStr := "-"
		if log.InputTokens != nil && log.OutputTokens != nil {
			tokensStr = fmt.Sprintf("%s/%s", ui.FormatTokens(*log.InputTokens), ui.FormatTokens(*log.OutputTokens))
		}

		startedStr := log.StartedAt.Format("2006-01-02 15:04:05")

		fmt.Fprintf(os.Stdout, "  %-20s  %-20s  %s  %-8s  %-10s  %s\n",
			ui.Truncate(log.JobName, 20),
			startedStr,
			statusStr,
			log.TriggerType,
			costStr,
			tokensStr,
		)

		if log.ErrorMsg != "" {
			errLines := strings.Split(log.ErrorMsg, "\n")
			fmt.Fprintf(os.Stdout, "  %s %s\n", ui.Dim.Render("Error:"), ui.Truncate(errLines[0], 70))
		}
	}

	fmt.Printf("\n  %s\n", ui.Dim.Render(fmt.Sprintf("Showing %d entries", len(logs))))
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

	fmt.Println()
	fmt.Println(ui.Title.Render(fmt.Sprintf("  Usage: %s", jobName)))
	fmt.Println()

	if len(logs) == 0 {
		fmt.Println("  No execution history found.")
		return
	}

	// Per-run table
	fmt.Fprintf(os.Stdout, "  %s  %s  %s  %s  %s  %s  %s\n",
		ui.Title.Width(20).Render("DATE"),
		ui.Title.Width(8).Render("STATUS"),
		ui.Title.Width(10).Render("INPUT"),
		ui.Title.Width(10).Render("OUTPUT"),
		ui.Title.Width(12).Render("CACHE READ"),
		ui.Title.Width(12).Render("CACHE WRITE"),
		ui.Title.Width(10).Render("COST"),
	)
	fmt.Println(ui.Dim.Render("  " + strings.Repeat("-", 88)))

	for _, log := range logs {
		dateStr := log.StartedAt.Format("2006-01-02 15:04:05")

		statusStr := log.Status
		inputStr := "-"
		outputStr := "-"
		cacheReadStr := "-"
		cacheWriteStr := "-"
		costStr := "-"

		if log.InputTokens != nil {
			inputStr = ui.FormatTokens(*log.InputTokens)
		}
		if log.OutputTokens != nil {
			outputStr = ui.FormatTokens(*log.OutputTokens)
		}
		if log.CacheReadTokens != nil {
			cacheReadStr = ui.FormatTokens(*log.CacheReadTokens)
		}
		if log.CacheCreationTokens != nil {
			cacheWriteStr = ui.FormatTokens(*log.CacheCreationTokens)
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

	fmt.Println(ui.Dim.Render("  " + strings.Repeat("-", 88)))
	fmt.Fprintf(os.Stdout, "  %-20s  %-8s  %10s  %10s  %12s  %12s  %s\n",
		ui.Accent.Render(fmt.Sprintf("TOTAL (%d runs)", usage.TotalRuns)),
		"",
		ui.FormatTokens(usage.TotalInputTokens),
		ui.FormatTokens(usage.TotalOutputTokens),
		ui.FormatTokens(usage.TotalCacheRead),
		ui.FormatTokens(usage.TotalCacheCreation),
		ui.Accent.Render(fmt.Sprintf("$%.4f", usage.TotalCostUSD)),
	)
	fmt.Println()
}
