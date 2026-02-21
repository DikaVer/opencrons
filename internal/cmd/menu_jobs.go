// File menu_jobs.go implements TUI job management handlers including handleManageJobs
// (job picker with action menu loop), handleRunFromMenu, and runJobByName which
// executes a job and displays results. These handlers are shared by both the TUI
// menu and CLI command paths.
package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/DikaVer/opencrons/internal/config"
	"github.com/DikaVer/opencrons/internal/executor"
	"github.com/DikaVer/opencrons/internal/messenger/telegram"
	"github.com/DikaVer/opencrons/internal/platform"
	"github.com/DikaVer/opencrons/internal/storage"
	"github.com/DikaVer/opencrons/internal/tui"
	"github.com/DikaVer/opencrons/internal/ui"
)

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
			ui.FormatTokens(result.InputTokens), ui.FormatTokens(result.OutputTokens),
			ui.FormatTokens(result.CacheReadTokens), ui.FormatTokens(result.CacheCreationTokens))
	}
	if result.StdoutPath != "" {
		fmt.Printf("  Output:   %s\n", result.StdoutPath)
	}
	if result.ErrorMsg != "" {
		fmt.Printf("  Error:    %s\n", result.ErrorMsg)
	}

	// Send output to Telegram if summary is enabled and output is available
	if job.SummaryEnabled && result.Output != "" {
		if msgCfg := platform.GetMessengerConfig(); msgCfg != nil && msgCfg.Type == "telegram" {
			tgBot, err := telegram.New(db, msgCfg, log.New(os.Stderr, "", 0))
			if err != nil {
				fmt.Fprintf(os.Stderr, "  Warning: Telegram notification failed: %v\n", err)
			} else {
				tgBot.NotifyJobComplete(ctx, job.Name, result.Status, result.Output)
				fmt.Println("  Summary sent to Telegram.")
			}
		}
	}
}
