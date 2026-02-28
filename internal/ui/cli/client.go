package cli

import (
	"fmt"
	"text/tabwriter"
	"os"

	"bootforge/internal/infra/toml"

	"github.com/spf13/cobra"
)

var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "Manage PXE clients",
}

var clientListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured clients",
	RunE:  runClientList,
}

var clientShowCmd = &cobra.Command{
	Use:   "show [mac]",
	Short: "Show details for a specific client",
	Args:  cobra.ExactArgs(1),
	RunE:  runClientShow,
}

func init() {
	clientCmd.AddCommand(clientListCmd, clientShowCmd)
	rootCmd.AddCommand(clientCmd)
}

func runClientList(cmd *cobra.Command, args []string) error {
	cfg, err := toml.LoadDir(cfgDir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "MAC\tNAME\tENABLED\tENTRIES\tDEFAULT\tSOURCE")
	for _, c := range cfg.Clients {
		mac := c.MAC.String()
		if c.IsWildcard() {
			mac = "*"
		}
		entries := fmt.Sprintf("%d", len(c.Menu.Entries))
		def := c.Menu.Default
		if def == "" {
			def = "-"
		}
		enabled := "yes"
		if !c.Enabled {
			enabled = "no"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", mac, c.Name, enabled, entries, def, c.SourceFile)
	}
	w.Flush()
	return nil
}

func runClientShow(cmd *cobra.Command, args []string) error {
	cfg, err := toml.LoadDir(cfgDir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	macStr := args[0]
	for _, c := range cfg.Clients {
		cMAC := c.MAC.String()
		if c.IsWildcard() {
			cMAC = "*"
		}
		if cMAC == macStr || c.MAC.String() == macStr {
			fmt.Printf("Client: %s\n", c.Name)
			fmt.Printf("  MAC:     %s\n", cMAC)
			fmt.Printf("  Enabled: %t\n", c.Enabled)
			fmt.Printf("  Source:  %s\n", c.SourceFile)
			fmt.Printf("  Menu:\n")
			fmt.Printf("    Entries: %v\n", c.Menu.Entries)
			fmt.Printf("    Default: %s\n", c.Menu.Default)
			fmt.Printf("    Timeout: %d\n", c.Menu.Timeout)
			if len(c.Vars) > 0 {
				fmt.Printf("  Vars:\n")
				for k, v := range c.Vars {
					fmt.Printf("    %s = %s\n", k, v)
				}
			}
			return nil
		}
	}
	return fmt.Errorf("client %q not found", macStr)
}
