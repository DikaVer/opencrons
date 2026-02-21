// File logs.go implements the logs command, which displays execution logs as JSON.
// It supports an optional job name filter and a -n flag to limit the number of
// entries returned.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/dika-maulidal/opencron/internal/platform"
	"github.com/dika-maulidal/opencron/internal/storage"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs [name]",
	Short: "Show execution history",
	Long:  "Show execution logs in JSON format. Optionally filter by job name.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runLogs,
}

func init() {
	rootCmd.AddCommand(logsCmd)
	logsCmd.Flags().IntP("limit", "n", 20, "number of entries to show")
}

func runLogs(cmd *cobra.Command, args []string) error {
	if err := platform.EnsureDirs(); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	limit, _ := cmd.Flags().GetInt("limit")

	db, err := storage.Open(platform.DBPath())
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	var logs []storage.ExecutionLog
	if len(args) > 0 {
		logs, err = db.GetLogsByJobName(args[0], limit)
	} else {
		logs, err = db.GetRecentLogs(limit)
	}
	if err != nil {
		return fmt.Errorf("querying logs: %w", err)
	}

	if len(logs) == 0 {
		fmt.Fprintln(os.Stdout, "[]")
		return nil
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(logs)
}
