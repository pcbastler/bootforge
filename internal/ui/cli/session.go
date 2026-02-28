package cli

import (
	"fmt"

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
	rootCmd.AddCommand(sessionCmd)
}

func runSessionList(cmd *cobra.Command, args []string) error {
	// TODO M11: Call GET /api/v1/sessions and display results.
	fmt.Println("Querying active sessions...")
	fmt.Println("  (API not yet implemented — coming in a future release)")
	return nil
}

func runSessionShow(cmd *cobra.Command, args []string) error {
	// TODO M11: Call GET /api/v1/sessions/{id} and display results.
	fmt.Printf("Querying session %s...\n", args[0])
	fmt.Println("  (API not yet implemented — coming in a future release)")
	return nil
}
