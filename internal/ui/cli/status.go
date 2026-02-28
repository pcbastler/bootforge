package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show server status",
	Long:  "Query the running Bootforge server for its current status.",
	RunE:  runStatus,
}

var apiAddr string

func init() {
	statusCmd.Flags().StringVar(&apiAddr, "addr", "http://localhost:8080", "Server API address")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	// TODO M11: Call GET /api/v1/status and display results.
	fmt.Println("Querying server status...")
	fmt.Printf("  API: %s/api/v1/status\n", apiAddr)
	fmt.Println("  (API not yet implemented — coming in a future release)")
	return nil
}
