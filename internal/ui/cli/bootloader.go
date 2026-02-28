package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"bootforge/internal/infra/toml"

	"github.com/spf13/cobra"
)

var bootloaderCmd = &cobra.Command{
	Use:   "bootloader",
	Short: "Manage bootloader files",
}

var bootloaderListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured bootloader files",
	RunE:  runBootloaderList,
}

var bootloaderCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check if bootloader files exist",
	RunE:  runBootloaderCheck,
}

func init() {
	bootloaderCmd.AddCommand(bootloaderListCmd, bootloaderCheckCmd)
	rootCmd.AddCommand(bootloaderCmd)
}

func runBootloaderList(cmd *cobra.Command, args []string) error {
	cfg, err := toml.LoadDir(cfgDir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	bl := cfg.Bootloader
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ARCH\tFILENAME")
	fmt.Fprintf(w, "UEFI x64\t%s\n", bl.UEFX64)
	fmt.Fprintf(w, "UEFI x86\t%s\n", bl.UEFX86)
	fmt.Fprintf(w, "BIOS\t%s\n", bl.BIOS)
	fmt.Fprintf(w, "ARM64\t%s\n", bl.ARM64)
	w.Flush()

	fmt.Printf("\nDirectory: %s\n", bl.Dir)
	fmt.Printf("Chain URL: %s\n", bl.ChainURL)
	return nil
}

func runBootloaderCheck(cmd *cobra.Command, args []string) error {
	cfg, err := toml.LoadDir(cfgDir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	bl := cfg.Bootloader
	blDir := filepath.Join(cfg.Server.DataDir, bl.Dir)
	files := map[string]string{
		"UEFI x64": bl.UEFX64,
		"UEFI x86": bl.UEFX86,
		"BIOS":     bl.BIOS,
		"ARM64":    bl.ARM64,
	}

	allOK := true
	for arch, filename := range files {
		if filename == "" {
			fmt.Printf("  SKIP  %s: not configured\n", arch)
			continue
		}
		path := filepath.Join(blDir, filename)
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			fmt.Printf("  FAIL  %s: %s not found\n", arch, path)
			allOK = false
		} else if err != nil {
			fmt.Printf("  FAIL  %s: %v\n", arch, err)
			allOK = false
		} else {
			fmt.Printf("  OK    %s: %s (%d bytes)\n", arch, path, info.Size())
		}
	}

	if !allOK {
		return fmt.Errorf("some bootloader files are missing")
	}
	fmt.Println("\nAll bootloader files present.")
	return nil
}
