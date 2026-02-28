package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage boot sessions",
}

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active boot sessions",
	RunE:  runSessionList,
}

var sessionShowCmd = &cobra.Command{
	Use:   "show [id]",
	Short: "Show details for a boot session",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionShow,
}

func init() {
	sessionCmd.AddCommand(sessionListCmd, sessionShowCmd)
	sessionListCmd.Flags().StringVar(&apiAddr, "addr", "http://localhost:8080", "Server API address")
	sessionShowCmd.Flags().StringVar(&apiAddr, "addr", "http://localhost:8080", "Server API address")
	rootCmd.AddCommand(sessionCmd)
}

func runSessionList(cmd *cobra.Command, args []string) error {
	client := newAPIClient(apiAddr)

	var sessions []struct {
		ID    string `json:"id"`
		MAC   string `json:"mac"`
		State string `json:"state"`
		Arch  string `json:"arch"`
		At    string `json:"started_at"`
	}

	if err := client.get("/api/v1/sessions", &sessions); err != nil {
		return fmt.Errorf("querying sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No active sessions.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tMAC\tSTATE\tARCH\tSTARTED")
	for _, s := range sessions {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.ID, s.MAC, s.State, s.Arch, s.At)
	}
	return w.Flush()
}

func runSessionShow(cmd *cobra.Command, args []string) error {
	// The API currently only supports listing all sessions.
	// Show the list and filter client-side.
	client := newAPIClient(apiAddr)
	id := args[0]

	var sessions []struct {
		ID    string `json:"id"`
		MAC   string `json:"mac"`
		State string `json:"state"`
		Arch  string `json:"arch"`
		At    string `json:"started_at"`
	}

	if err := client.get("/api/v1/sessions", &sessions); err != nil {
		return fmt.Errorf("querying sessions: %w", err)
	}

	for _, s := range sessions {
		if s.ID == id {
			fmt.Printf("Session:  %s\n", s.ID)
			fmt.Printf("MAC:      %s\n", s.MAC)
			fmt.Printf("State:    %s\n", s.State)
			fmt.Printf("Arch:     %s\n", s.Arch)
			fmt.Printf("Started:  %s\n", s.At)
			return nil
		}
	}

	return fmt.Errorf("session %s not found", id)
}
