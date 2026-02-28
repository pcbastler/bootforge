package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Run health checks",
	Long:  "Trigger an on-demand health check against the running server.",
	RunE:  runTest,
}

func init() {
	rootCmd.AddCommand(testCmd)
}

func runTest(cmd *cobra.Command, args []string) error {
	// TODO M11: Call POST /api/v1/test and display results.
	fmt.Println("Running health checks...")
	fmt.Println("  (API not yet implemented — coming in a future release)")
	return nil
}
