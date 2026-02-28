package cli

import (
	"fmt"

	"bootforge/internal/infra/toml"

	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration files",
	Long:  "Load and validate all configuration files in the config directory without starting services.",
	RunE:  runValidate,
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

func runValidate(cmd *cobra.Command, args []string) error {
	fmt.Printf("Validating configuration in %s ...\n\n", cfgDir)

	cfg, err := toml.LoadDir(cfgDir)
	if err != nil {
		fmt.Printf("  FAIL  Configuration loading failed:\n")
		fmt.Printf("        %v\n\n", err)
		return fmt.Errorf("validation failed")
	}

	if err := cfg.Validate(); err != nil {
		fmt.Printf("  FAIL  Configuration validation failed:\n")
		fmt.Printf("        %v\n\n", err)
		return fmt.Errorf("validation failed")
	}

	fmt.Printf("  OK    Server:     interface=%s, data_dir=%s\n", cfg.Server.Interface, cfg.Server.DataDir)
	fmt.Printf("  OK    DHCP Proxy: enabled=%t\n", cfg.DHCPProxy.Enabled)
	fmt.Printf("  OK    TFTP:       enabled=%t\n", cfg.TFTP.Enabled)
	fmt.Printf("  OK    HTTP:       enabled=%t\n", cfg.HTTP.Enabled)
	fmt.Printf("  OK    Menus:      %d entries\n", len(cfg.Menus))
	fmt.Printf("  OK    Clients:    %d configured\n", len(cfg.Clients))
	fmt.Println()
	fmt.Println("Configuration is valid.")

	return nil
}
