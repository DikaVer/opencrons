// File debug.go implements the debug logging toggle command. With no arguments
// it displays the current debug state; with "on" or "off" it enables or disables
// debug logging by updating the platform settings.
package cmd

import (
	"fmt"

	"github.com/DikaVer/opencron/internal/platform"
	"github.com/DikaVer/opencron/internal/ui"
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
	if len(args) == 0 {
		// Show current state
		if platform.IsDebugEnabled() {
			fmt.Printf("  Debug logging: %s\n", ui.Success.Render("on"))
		} else {
			fmt.Printf("  Debug logging: %s\n", ui.Fail.Render("off"))
		}
		fmt.Println(ui.Dim.Render("  Use 'opencron debug on' or 'opencron debug off' to toggle."))
		return nil
	}

	switch args[0] {
	case "on":
		if err := platform.SetDebug(true); err != nil {
			return fmt.Errorf("saving settings: %w", err)
		}
		fmt.Printf("  Debug logging: %s\n", ui.Success.Render("on"))
		fmt.Printf("  Logs: %s\n", ui.Dim.Render(platform.LogsDir()+"/opencron-debug.log"))
	case "off":
		if err := platform.SetDebug(false); err != nil {
			return fmt.Errorf("saving settings: %w", err)
		}
		fmt.Printf("  Debug logging: %s\n", ui.Fail.Render("off"))
	default:
		return fmt.Errorf("invalid argument %q: use 'on' or 'off'", args[0])
	}

	return nil
}
