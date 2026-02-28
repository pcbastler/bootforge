package cli

import (
	"fmt"

	"bootforge/internal/infra/toml"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the loaded configuration summary",
	RunE:  runConfigShow,
}

func init() {
	configCmd.AddCommand(configShowCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, err := toml.LoadDir(cfgDir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	fmt.Printf("Configuration: %s\n\n", cfgDir)

	fmt.Printf("Server:\n")
	fmt.Printf("  Interface: %s\n", cfg.Server.Interface)
	fmt.Printf("  Data Dir:  %s\n", cfg.Server.DataDir)
	fmt.Printf("  Log Level: %s\n", cfg.Server.Logging.Level)
	fmt.Printf("  Log Format: %s\n", cfg.Server.Logging.Format)

	fmt.Printf("\nDHCP Proxy:\n")
	fmt.Printf("  Enabled:    %t\n", cfg.DHCPProxy.Enabled)
	if cfg.DHCPProxy.Enabled {
		fmt.Printf("  Port:       %d\n", cfg.DHCPProxy.Port)
		fmt.Printf("  Proxy Port: %d\n", cfg.DHCPProxy.ProxyPort)
	}

	fmt.Printf("\nTFTP:\n")
	fmt.Printf("  Enabled: %t\n", cfg.TFTP.Enabled)
	if cfg.TFTP.Enabled {
		fmt.Printf("  Port:    %d\n", cfg.TFTP.Port)
	}

	fmt.Printf("\nHTTP:\n")
	fmt.Printf("  Enabled: %t\n", cfg.HTTP.Enabled)
	if cfg.HTTP.Enabled {
		fmt.Printf("  Port:    %d\n", cfg.HTTP.Port)
		fmt.Printf("  TLS:     %t\n", cfg.HTTP.TLS.Enabled)
	}

	fmt.Printf("\nBootloader:\n")
	fmt.Printf("  Directory: %s\n", cfg.Bootloader.Dir)
	fmt.Printf("  Chain URL: %s\n", cfg.Bootloader.ChainURL)

	fmt.Printf("\nMenus: %d entries\n", len(cfg.Menus))
	for _, m := range cfg.Menus {
		fmt.Printf("  - %s (%s): %s\n", m.Name, m.Type, m.Label)
	}

	fmt.Printf("\nClients: %d configured\n", len(cfg.Clients))
	for _, c := range cfg.Clients {
		mac := c.MAC.String()
		if c.IsWildcard() {
			mac = "* (default)"
		}
		fmt.Printf("  - %s: %s\n", mac, c.Name)
	}

	return nil
}
