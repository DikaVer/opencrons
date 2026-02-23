// File run.go implements the run command, which executes a job immediately by name.
// It opens the database, runs the executor, and displays results including status,
// duration, cost, token usage, output path, and any errors.
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
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <name>",
	Short: "Execute a job immediately",
	Args:  cobra.ExactArgs(1),
	RunE:  runRun,
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func runRun(cmd *cobra.Command, args []string) error {
	name := args[0]

	if err := platform.EnsureDirs(); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	job, err := config.FindJobByName(platform.SchedulesDir(), name)
	if err != nil {
		return err
	}

	// Validate prompt file exists
	if err := job.ValidatePromptFileExists(platform.PromptsDir()); err != nil {
		return err
	}

	db, err := storage.Open(platform.DBPath())
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() { _ = db.Close() }()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	fmt.Printf("Running job %q...\n", name)

	result, err := executor.Run(ctx, db, job, "manual", 0)
	if err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}

	fmt.Printf("\nStatus:   %s\n", result.Status)
	fmt.Printf("Duration: %s\n", result.Duration.Round(1e9))
	fmt.Printf("Exit:     %d\n", result.ExitCode)
	if result.CostUSD > 0 {
		fmt.Printf("Cost:     $%.4f\n", result.CostUSD)
	}
	if result.InputTokens > 0 || result.OutputTokens > 0 {
		fmt.Printf("Tokens:   %d in / %d out / %d cache read / %d cache write\n",
			result.InputTokens, result.OutputTokens,
			result.CacheReadTokens, result.CacheCreationTokens)
	}
	if result.StdoutPath != "" {
		fmt.Printf("Output:   %s\n", result.StdoutPath)
	}
	if result.ErrorMsg != "" {
		fmt.Printf("Error:    %s\n", result.ErrorMsg)
	}

	// Send output to Telegram if summary is enabled and output is available
	if job.SummaryEnabled && result.Output != "" {
		msgCfg := platform.GetMessengerConfig()
		if msgCfg != nil && msgCfg.Type == "telegram" {
			tgBot, err := telegram.New(db, msgCfg, log.New(os.Stderr, "", 0))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Telegram notification failed: %v\n", err)
			} else {
				tgBot.NotifyJobComplete(ctx, job.Name, result.Status, result.Output)
				fmt.Println("Summary sent to Telegram.")
			}
		}
	}

	if result.Status != "success" {
		os.Exit(1)
	}

	return nil
}
