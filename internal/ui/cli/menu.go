package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"bootforge/internal/infra/toml"

	"github.com/spf13/cobra"
)

var menuCmd = &cobra.Command{
	Use:   "menu",
	Short: "Manage boot menus",
}

var menuListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured menu entries",
	RunE:  runMenuList,
}

var menuShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show details for a menu entry",
	Args:  cobra.ExactArgs(1),
	RunE:  runMenuShow,
}

func init() {
	menuCmd.AddCommand(menuListCmd, menuShowCmd)
	rootCmd.AddCommand(menuCmd)
}

func runMenuList(cmd *cobra.Command, args []string) error {
	cfg, err := toml.LoadDir(cfgDir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tLABEL\tTYPE\tKERNEL\tSOURCE")
	for _, m := range cfg.Menus {
		kernel := m.Boot.Kernel
		if kernel == "" {
			kernel = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", m.Name, m.Label, m.Type, kernel, m.SourceFile)
	}
	w.Flush()
	return nil
}

func runMenuShow(cmd *cobra.Command, args []string) error {
	cfg, err := toml.LoadDir(cfgDir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	name := args[0]
	for _, m := range cfg.Menus {
		if m.Name == name {
			fmt.Printf("Menu Entry: %s\n", m.Name)
			fmt.Printf("  Label:       %s\n", m.Label)
			fmt.Printf("  Type:        %s\n", m.Type)
			fmt.Printf("  Description: %s\n", m.Description)
			fmt.Printf("  Source:      %s\n", m.SourceFile)
			if m.Type != 3 { // not exit
				fmt.Printf("  Boot:\n")
				if m.Boot.Kernel != "" {
					fmt.Printf("    Kernel:  %s\n", m.Boot.Kernel)
				}
				if m.Boot.Initrd != "" {
					fmt.Printf("    Initrd:  %s\n", m.Boot.Initrd)
				}
				if m.Boot.Cmdline != "" {
					fmt.Printf("    Cmdline: %s\n", m.Boot.Cmdline)
				}
				if m.Boot.Binary != "" {
					fmt.Printf("    Binary:  %s\n", m.Boot.Binary)
				}
				if m.Boot.Loader != "" {
					fmt.Printf("    Loader:  %s\n", m.Boot.Loader)
				}
			}
			if m.HTTP.Path != "" || m.HTTP.Files != "" {
				fmt.Printf("  HTTP:\n")
				if m.HTTP.Path != "" {
					fmt.Printf("    Path:  %s\n", m.HTTP.Path)
				}
				if m.HTTP.Files != "" {
					fmt.Printf("    Files: %s\n", m.HTTP.Files)
				}
			}
			return nil
		}
	}
	return fmt.Errorf("menu entry %q not found", name)
}
