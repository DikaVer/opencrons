// File root.go implements the root Cobra command and interactive TUI main menu loop.
// It defines rootCmd with a PersistentPreRunE hook for first-run setup detection
// (skipping setup, help, and version commands). Execute is the CLI entry point.
// runMainMenu loops the TUI main menu, dispatching to handlers for add, manage,
// run, logs, daemon, settings, and exit.
package cmd

import (
	"fmt"
	"os"

	"github.com/DikaVer/opencrons/internal/platform"
	"github.com/DikaVer/opencrons/internal/tui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "opencrons",
	Short: "OpenCron — Claude Code automation scheduler",
	Long:  "OpenCron runs Claude Code tasks on cron schedules with secure, predefined execution environments.",
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
