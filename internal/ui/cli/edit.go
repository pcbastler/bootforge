package cli

import (
	"bootforge/internal/infra/toml"
	"bootforge/internal/ui/cli/wizard"

	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit an existing configuration interactively",
	Long:  "Load the current configuration and walk through the setup wizard to make changes.\nCreates a backup of the current config before overwriting.",
	RunE:  runEdit,
}

func init() {
	rootCmd.AddCommand(editCmd)
}

func runEdit(cmd *cobra.Command, args []string) error {
	existing, err := toml.LoadDir(cfgDir)
	if err != nil {
		return err
	}

	_, err = wizard.RunEdit(cfgDir, existing)
	return err
}
