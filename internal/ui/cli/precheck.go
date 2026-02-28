package cli

import (
	"fmt"
	"os"

	"bootforge/internal/domain"
	"bootforge/internal/health"
	"bootforge/internal/infra/toml"

	"github.com/spf13/cobra"
)

var precheckCmd = &cobra.Command{
	Use:   "precheck",
	Short: "Run pre-flight checks without starting the server",
	Long:  "Validate configuration and check that the system is ready to run:\nnetwork interface, port availability, bootloader files, data directory.",
	RunE:  runPrecheck,
}

func init() {
	rootCmd.AddCommand(precheckCmd)
}

func runPrecheck(cmd *cobra.Command, args []string) error {
	cfg, err := toml.LoadDir(cfgDir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config validation: %w", err)
	}

	fmt.Println("Pre-flight checks")
	fmt.Println()

	results, allOK := health.RunPreflight(cfg)

	for _, r := range results {
		indicator := "OK  "
		switch r.Status {
		case domain.StatusFail:
			indicator = "FAIL"
		case domain.StatusWarn:
			indicator = "WARN"
		}
		fmt.Printf("  %s  %-24s %s\n", indicator, r.Name, r.Message)
	}

	fmt.Println()

	if !allOK {
		fmt.Println("Some checks failed. Fix the issues above before starting the server.")
		os.Exit(1)
	}

	fmt.Println("All checks passed. Ready to start.")
	return nil
}
