package cli

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"bootforge/internal/buildinfo"
	"bootforge/internal/infra/toml"

	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the boot server",
	Long:  "Start all configured services (DHCP proxy, TFTP, HTTP) and begin serving boot requests.",
	RunE:  runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	logger := slog.Default()

	// Startup banner.
	fmt.Printf("Bootforge %s\n\n", buildinfo.Short())

	// Load configuration.
	logger.Info("loading configuration", "dir", cfgDir)
	cfg, err := toml.LoadDir(cfgDir)
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	logger.Info("configuration loaded",
		"menus", len(cfg.Menus),
		"clients", len(cfg.Clients),
		"dhcp", cfg.DHCPProxy.Enabled,
		"tftp", cfg.TFTP.Enabled,
		"http", cfg.HTTP.Enabled,
	)

	// TODO M12: Wire all services (DHCP, TFTP, HTTP), start them,
	// handle graceful shutdown and config reload.

	// Wait for shutdown signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("server is ready, waiting for connections")
	sig := <-sigCh
	logger.Info("received signal, shutting down", "signal", sig)

	return nil
}
