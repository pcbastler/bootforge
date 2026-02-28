package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show server logs",
	Long:  "Stream or query recent log entries from the running server.",
	RunE:  runLogs,
}

func init() {
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	// TODO M11: Call GET /api/v1/logs and display results.
	fmt.Println("Querying server logs...")
	fmt.Println("  (API not yet implemented — coming in a future release)")
	return nil
}
