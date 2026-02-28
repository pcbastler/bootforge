package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show server logs",
	Long:  "Query recent log entries from the running server.",
	RunE:  runLogs,
}

var logsLimit int
var logsService string
var logsMAC string

func init() {
	logsCmd.Flags().StringVar(&apiAddr, "addr", "http://localhost:8080", "Server API address")
	logsCmd.Flags().IntVarP(&logsLimit, "lines", "n", 50, "Number of log entries to show")
	logsCmd.Flags().StringVar(&logsService, "service", "", "Filter by service (dhcp, tftp, http, health)")
	logsCmd.Flags().StringVar(&logsMAC, "mac", "", "Filter by MAC address")
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	client := newAPIClient(apiAddr)

	var logs []struct {
		Time    string `json:"time"`
		Level   string `json:"level"`
		Message string `json:"message"`
		Service string `json:"service"`
		MAC     string `json:"mac"`
	}

	path := fmt.Sprintf("/api/v1/logs?limit=%d", logsLimit)
	if logsService != "" {
		path += "&service=" + logsService
	}
	if logsMAC != "" {
		path += "&mac=" + logsMAC
	}

	if err := client.get(path, &logs); err != nil {
		return fmt.Errorf("querying logs: %w", err)
	}

	if len(logs) == 0 {
		fmt.Println("No log entries.")
		return nil
	}

	for _, l := range logs {
		svc := l.Service
		if svc == "" {
			svc = "---"
		}
		mac := l.MAC
		if mac == "" {
			mac = ""
		} else {
			mac = " " + mac
		}
		fmt.Printf("%-5s %-8s %s%s\n", l.Level, svc, l.Message, mac)
	}

	return nil
}
