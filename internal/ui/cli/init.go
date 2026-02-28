package cli

import (
	"fmt"
	"os"

	"bootforge/internal/ui/cli/wizard"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new configuration directory",
	Long:  "Run the interactive setup wizard to create a new Bootforge configuration.\nUses the --config flag to set the target directory.",
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	// Check if directory already exists and has a config.
	entries, err := os.ReadDir(cfgDir)
	if err == nil && len(entries) > 0 {
		// Check for existing bootforge.toml specifically.
		for _, e := range entries {
			if e.Name() == "bootforge.toml" {
				return fmt.Errorf("configuration already exists in %s (use 'bootforge edit' to modify)", cfgDir)
			}
		}
	}

	_, err = wizard.Run(cfgDir)
	return err
}
