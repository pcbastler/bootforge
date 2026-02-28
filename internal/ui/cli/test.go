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
	testCmd.Flags().StringVar(&apiAddr, "addr", "http://localhost:8080", "Server API address")
	rootCmd.AddCommand(testCmd)
}

func runTest(cmd *cobra.Command, args []string) error {
	client := newAPIClient(apiAddr)

	var result struct {
		Status string `json:"status"`
		Checks []struct {
			Name     string `json:"name"`
			Status   string `json:"status"`
			Message  string `json:"message"`
			Duration string `json:"duration"`
		} `json:"checks"`
		At string `json:"checked_at"`
	}

	if err := client.post("/api/v1/test", &result); err != nil {
		return fmt.Errorf("running health checks: %w", err)
	}

	fmt.Printf("Health: %s\n\n", result.Status)
	for _, c := range result.Checks {
		indicator := "OK  "
		if c.Status != "ok" {
			indicator = "FAIL"
		}
		fmt.Printf("  %s  %-20s %s (%s)\n", indicator, c.Name, c.Message, c.Duration)
	}

	return nil
}
