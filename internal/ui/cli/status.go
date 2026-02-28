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
	client := newAPIClient(apiAddr)

	var status struct {
		Version string `json:"version"`
		Uptime  string `json:"uptime"`
		Health  *struct {
			Status string `json:"status"`
			Checks []struct {
				Name     string `json:"name"`
				Status   string `json:"status"`
				Message  string `json:"message"`
				Duration string `json:"duration"`
			} `json:"checks"`
			At string `json:"checked_at"`
		} `json:"health,omitempty"`
	}

	if err := client.get("/api/v1/status", &status); err != nil {
		return fmt.Errorf("querying server: %w", err)
	}

	fmt.Printf("Version:  %s\n", status.Version)
	fmt.Printf("Uptime:   %s\n", status.Uptime)

	if status.Health != nil {
		fmt.Printf("Health:   %s\n", status.Health.Status)
		for _, c := range status.Health.Checks {
			fmt.Printf("  %-20s %s  %s\n", c.Name, c.Status, c.Message)
		}
	}

	return nil
}
