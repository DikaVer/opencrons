package cmd

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/dika-maulidal/cli-scheduler/internal/platform"
	"github.com/spf13/cobra"
)

var debugCmd = &cobra.Command{
	Use:   "debug [on|off]",
	Short: "Show or toggle debug logging",
	Long:  "With no arguments, shows the current debug state. Use 'on' or 'off' to toggle.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runDebug,
}

func init() {
	rootCmd.AddCommand(debugCmd)
}

func runDebug(cmd *cobra.Command, args []string) error {
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1"))
	failStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))

	if len(args) == 0 {
		// Show current state
		if platform.IsDebugEnabled() {
			fmt.Printf("  Debug logging: %s\n", successStyle.Render("on"))
		} else {
			fmt.Printf("  Debug logging: %s\n", failStyle.Render("off"))
		}
		fmt.Println(dimStyle.Render("  Use 'scheduler debug on' or 'scheduler debug off' to toggle."))
		return nil
	}

	switch args[0] {
	case "on":
		if err := platform.SetDebug(true); err != nil {
			return fmt.Errorf("saving settings: %w", err)
		}
		fmt.Printf("  Debug logging: %s\n", successStyle.Render("on"))
		fmt.Printf("  Logs: %s\n", dimStyle.Render(platform.LogsDir()+"/scheduler-debug.log"))
	case "off":
		if err := platform.SetDebug(false); err != nil {
			return fmt.Errorf("saving settings: %w", err)
		}
		fmt.Printf("  Debug logging: %s\n", failStyle.Render("off"))
	default:
		return fmt.Errorf("invalid argument %q: use 'on' or 'off'", args[0])
	}

	return nil
}
