// Package cli implements the Bootforge CLI using cobra.
package cli

import (
	"fmt"
	"log/slog"
	"os"

	"bootforge/internal/buildinfo"

	"github.com/spf13/cobra"
)

var (
	cfgDir string
	debug  bool
)

var rootCmd = &cobra.Command{
	Use:   "bootforge",
	Short: "Bootforge — PXE Boot Server",
	Long:  "Bootforge is a network boot server providing DHCP proxy, TFTP, and HTTP services for PXE booting.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setupLogging()
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Bootforge %s\n", buildinfo.String())
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgDir, "config", "/etc/bootforge", "Configuration directory")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug logging")

	rootCmd.AddCommand(versionCmd)
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func setupLogging() {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(handler))
}
