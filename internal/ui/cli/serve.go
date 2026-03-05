package cli

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"bootforge/internal/buildinfo"
	"bootforge/internal/health"
	"bootforge/internal/infra/store"
	"bootforge/internal/infra/toml"
	"bootforge/internal/server"
	dhcpsvc "bootforge/internal/service/dhcp"
	"bootforge/internal/service/httpboot"
	tftpsvc "bootforge/internal/service/tftp"
	"bootforge/internal/ui/api"

	"github.com/spf13/cobra"
)

var forceStart bool

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the boot server",
	Long:  "Start all configured services (DHCP proxy, TFTP, HTTP) and begin serving boot requests.",
	RunE:  runServe,
}

func init() {
	serveCmd.Flags().BoolVar(&forceStart, "force", false, "Skip pre-flight checks")
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

	// Pre-flight checks.
	if cfg.Health.StartupCheck && !forceStart {
		fmt.Println("Pre-Flight Checks")
		fmt.Println()
		results, allOK := health.RunPreflight(cfg)
		for _, r := range results {
			switch r.Status {
			case 0: // OK
				fmt.Printf("  OK    %s\n", r.Message)
			case 1: // Warn
				fmt.Printf("  WARN  %s\n", r.Message)
			default: // Fail
				fmt.Printf("  FAIL  %s\n", r.Message)
			}
		}
		fmt.Println()
		if !allOK {
			return fmt.Errorf("pre-flight checks failed (use --force to skip)")
		}
		fmt.Println("All checks passed. Starting services...")
		fmt.Println()
	}

	// Build server core.
	registry := server.NewRegistry(cfg.Clients)
	menus := server.NewMenuResolver(cfg.Menus)
	sessionStore := store.NewMemorySessionStore()
	sessions := server.NewSessionManager(sessionStore)
	ipxe := server.NewIPXEGenerator()
	logBuffer := store.NewLogBuffer(1000)

	// Resolve server IP from interface.
	serverIP := resolveInterfaceIP(cfg.Server.Interface, logger)

	startedAt := time.Now()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start DHCP proxy server (requires root for port 67).
	if cfg.DHCPProxy.Enabled {
		srvCore := &server.Server{
			Registry: registry,
			Menus:    menus,
			Sessions: sessions,
			Events:   server.NewEventBus(64),
			IPXE:     ipxe,
			Logger:   logger,
		}
		dhcpSrv := dhcpsvc.NewProxyServer(cfg.DHCPProxy, cfg.Bootloader, net.ParseIP(serverIP), cfg.Server.Interface, cfg.HTTP.Port, srvCore, logger)
		if err := dhcpSrv.Start(ctx); err != nil {
			return fmt.Errorf("starting DHCP proxy: %w", err)
		}
		logger.Info("DHCP proxy started", "port", cfg.DHCPProxy.Port, "proxy_port", cfg.DHCPProxy.ProxyPort)
	}

	// Start TFTP server.
	if cfg.TFTP.Enabled {
		tftpSrv := tftpsvc.NewTFTPServer(cfg.TFTP, cfg.Server.DataDir, cfg.Bootloader.Dir, logger)
		if cfg.HTTP.Enabled && cfg.Bootloader.ChainURL != "" {
			tftpSrv.SetAutoexec(tftpsvc.AutoexecConfig{
				ServerIP: serverIP,
				HTTPPort: cfg.HTTP.Port,
				ChainURL: cfg.Bootloader.ChainURL,
			})
		}
		go func() {
			if err := tftpSrv.Start(ctx); err != nil {
				logger.Error("TFTP server failed", "error", err)
			}
		}()
		logger.Info("TFTP server started", "port", cfg.TFTP.Port)
	}

	// Start HTTP boot server + API.
	var httpSrv *httpboot.HTTPServer
	var apiDeps *api.APIDeps
	if cfg.HTTP.Enabled {
		vars := server.IPXEVars{
			ServerIP: serverIP,
			HTTPPort: cfg.HTTP.Port,
		}

		mux := http.NewServeMux()

		// Boot routes (menu scripts, static files, healthz).
		menuHandler := httpboot.NewMenuScriptHandler(registry, menus, ipxe, vars, logger)
		httpboot.SetupRoutes(mux, menuHandler, cfg.Server.DataDir, logger)

		// API routes.
		apiDeps = &api.APIDeps{
			Registry:  registry,
			Menus:     menus,
			Sessions:  sessions,
			LogBuffer: logBuffer,
			StartedAt: startedAt,
			ReloadFn: func() error {
				newCfg, err := toml.LoadDir(cfgDir)
				if err != nil {
					return err
				}
				if err := newCfg.Validate(); err != nil {
					return err
				}
				registry.Reload(newCfg.Clients)
				menus.Reload(newCfg.Menus)
				logger.Info("configuration reloaded",
					"menus", len(newCfg.Menus),
					"clients", len(newCfg.Clients))
				return nil
			},
		}
		apiMux := api.NewRouter(apiDeps, logger)
		mux.Handle("/api/", apiMux)

		httpSrv = httpboot.NewHTTPServer(cfg.HTTP, mux, logger)
		go func() {
			if err := httpSrv.Start(); err != nil {
				logger.Error("HTTP server failed", "error", err)
			}
		}()
		logger.Info("HTTP boot server started", "port", cfg.HTTP.Port)
	}

	// Start health checker.
	if cfg.Health.Enabled {
		var probes []health.Probe

		blDir := filepath.Join(cfg.Server.DataDir, cfg.Bootloader.Dir)
		var blFiles []string
		for _, f := range []string{cfg.Bootloader.UEFX64, cfg.Bootloader.UEFX86, cfg.Bootloader.BIOS, cfg.Bootloader.ARM64} {
			if f != "" {
				blFiles = append(blFiles, filepath.Join(blDir, f))
			}
		}
		if len(blFiles) > 0 {
			probes = append(probes, health.NewFileProbe("bootloader-files", blFiles))
		}
		probes = append(probes, health.NewDiskProbe(cfg.Server.DataDir, 0))

		if cfg.HTTP.Enabled {
			healthURL := fmt.Sprintf("http://127.0.0.1:%d/healthz", cfg.HTTP.Port)
			probes = append(probes, health.NewHTTPProbe(healthURL, 5*time.Second))
		}

		resultStore := health.NewResultStore(100)
		checker := health.NewChecker(probes, resultStore, cfg.Health.Interval, logger)
		go checker.Start(ctx)

		// Wire health checker into API deps if HTTP is running.
		if apiDeps != nil {
			apiDeps.Health = checker
		}
	}

	// Print startup summary.
	fmt.Println("Services running:")
	if cfg.TFTP.Enabled {
		fmt.Printf("  TFTP:  :%d\n", cfg.TFTP.Port)
	}
	if cfg.HTTP.Enabled {
		fmt.Printf("  HTTP:  :%d\n", cfg.HTTP.Port)
		fmt.Printf("  API:   :%d/api/v1/\n", cfg.HTTP.Port)
	}
	if cfg.DHCPProxy.Enabled {
		fmt.Printf("  DHCP:  :%d (proxy :%d) -- requires root\n", cfg.DHCPProxy.Port, cfg.DHCPProxy.ProxyPort)
	}
	fmt.Println()

	// Wait for shutdown signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("server is ready, waiting for connections")
	sig := <-sigCh
	fmt.Println("\nShutting down...")
	logger.Info("received signal, shutting down", "signal", sig)

	// Graceful shutdown in reverse order.
	cancel()

	if httpSrv != nil {
		if err := httpSrv.Stop(); err != nil {
			logger.Error("HTTP shutdown error", "error", err)
		}
	}

	logger.Info("shutdown complete")
	return nil
}

// resolveInterfaceIP gets the first IPv4 address from the named interface.
func resolveInterfaceIP(ifName string, logger *slog.Logger) string {
	iface, err := net.InterfaceByName(ifName)
	if err != nil {
		logger.Warn("could not find interface, using 127.0.0.1", "interface", ifName, "error", err)
		return "127.0.0.1"
	}

	addrs, err := iface.Addrs()
	if err != nil {
		logger.Warn("could not get interface addresses, using 127.0.0.1", "interface", ifName, "error", err)
		return "127.0.0.1"
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil {
			return ipNet.IP.String()
		}
	}

	logger.Warn("no IPv4 address found on interface, using 127.0.0.1", "interface", ifName)
	return "127.0.0.1"
}
