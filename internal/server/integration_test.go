package server_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bootforge/internal/domain"
	"bootforge/internal/infra/store"
	"bootforge/internal/server"
	"bootforge/internal/service/httpboot"
	tftpsvc "bootforge/internal/service/tftp"
	"bootforge/internal/ui/api"

	"github.com/pin/tftp/v3"
)

// TestFullBootFlow tests the complete boot flow:
// TFTP bootloader read -> HTTP iPXE menu script -> HTTP boot file
func TestFullBootFlow(t *testing.T) {
	// Setup: create data directory with bootloader and boot files.
	dataDir := t.TempDir()
	blDir := filepath.Join(dataDir, "bootloader")
	os.MkdirAll(blDir, 0755)
	os.WriteFile(filepath.Join(blDir, "ipxe.efi"), []byte("UEFI-BOOTLOADER-CONTENT"), 0644)
	os.WriteFile(filepath.Join(blDir, "undionly.kpxe"), []byte("BIOS-BOOTLOADER-CONTENT"), 0644)

	toolDir := filepath.Join(dataDir, "tools", "rescue")
	os.MkdirAll(toolDir, 0755)
	os.WriteFile(filepath.Join(toolDir, "vmlinuz"), []byte("RESCUE-KERNEL-BINARY"), 0644)
	os.WriteFile(filepath.Join(toolDir, "initrd"), []byte("RESCUE-INITRD-BINARY"), 0644)

	// Domain setup.
	menus := []*domain.MenuEntry{
		{
			Name:  "rescue",
			Label: "Rescue System",
			Type:  domain.MenuLive,
			HTTP:  domain.MenuHTTP{Path: "/data/tools/rescue/"},
			Boot:  domain.BootParams{Kernel: "vmlinuz", Initrd: "initrd"},
		},
		{
			Name:  "local-disk",
			Label: "Boot from local disk",
			Type:  domain.MenuExit,
		},
	}

	clientMAC := net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01}
	clients := []*domain.Client{
		{
			MAC:     clientMAC,
			Name:    "test-server",
			Enabled: true,
			Menu: domain.MenuConfig{
				Entries: []string{"rescue", "local-disk"},
				Default: "rescue",
				Timeout: 10,
			},
		},
		{
			MAC:     domain.WildcardMAC,
			Name:    "default",
			Enabled: true,
			Menu: domain.MenuConfig{
				Entries: []string{"local-disk"},
			},
		},
	}

	// Server core.
	registry := server.NewRegistry(clients)
	resolver := server.NewMenuResolver(menus)
	sessionStore := store.NewMemorySessionStore()
	sessions := server.NewSessionManager(sessionStore)
	ipxe := server.NewIPXEGenerator()
	logger := slog.Default()

	// --- Start TFTP server on ephemeral port ---
	tftpCfg := domain.TFTPConfig{
		Enabled:   true,
		Port:      0,
		Timeout:   5 * time.Second,
		BlockSize: 512,
	}
	tftpSrv := tftpsvc.NewTFTPServer(tftpCfg, dataDir, "bootloader", logger)

	tftpConn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("TFTP listen: %v", err)
	}
	tftpAddr := tftpConn.LocalAddr().String()

	tftpInternal := tftp.NewServer(tftpSrv.ReadHandler(), nil)
	tftpInternal.SetTimeout(5 * time.Second)
	go tftpInternal.Serve(tftpConn)
	defer tftpInternal.Shutdown()

	// --- Start HTTP server on ephemeral port ---
	httpLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("HTTP listen: %v", err)
	}
	httpPort := httpLn.Addr().(*net.TCPAddr).Port
	httpBase := fmt.Sprintf("http://127.0.0.1:%d", httpPort)

	vars := server.IPXEVars{
		ServerIP: "127.0.0.1",
		HTTPPort: httpPort,
	}

	mux := http.NewServeMux()
	menuHandler := httpboot.NewMenuScriptHandler(registry, resolver, ipxe, vars, logger)
	httpboot.SetupRoutes(mux, menuHandler, dataDir, logger)

	// API routes.
	apiDeps := &api.APIDeps{
		Registry:  registry,
		Menus:     resolver,
		Sessions:  sessions,
		StartedAt: time.Now(),
	}
	apiMux := api.NewRouter(apiDeps, logger)
	mux.Handle("/api/", apiMux)

	httpCfg := domain.HTTPConfig{Enabled: true, Port: httpPort}
	httpSrv := httpboot.NewHTTPServer(httpCfg, mux, logger)
	go httpSrv.StartOnListener(httpLn)
	defer httpSrv.Stop()

	// === Test 1: TFTP Read bootloader ===
	t.Run("TFTP read bootloader", func(t *testing.T) {
		client, err := tftp.NewClient(tftpAddr)
		if err != nil {
			t.Fatalf("TFTP client: %v", err)
		}
		client.SetTimeout(3 * time.Second)

		wt, err := client.Receive("ipxe.efi", "octet")
		if err != nil {
			t.Fatalf("TFTP receive: %v", err)
		}

		var buf bytes.Buffer
		wt.WriteTo(&buf)
		if buf.String() != "UEFI-BOOTLOADER-CONTENT" {
			t.Errorf("TFTP data = %q, want %q", buf.String(), "UEFI-BOOTLOADER-CONTENT")
		}
	})

	// === Test 2: HTTP iPXE menu script ===
	t.Run("HTTP iPXE menu script", func(t *testing.T) {
		resp, err := http.Get(httpBase + "/boot/aa:bb:cc:dd:ee:01/menu.ipxe")
		if err != nil {
			t.Fatalf("GET menu.ipxe: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		script := string(body)

		if !strings.HasPrefix(script, "#!ipxe") {
			t.Error("script should start with #!ipxe")
		}
		if !strings.Contains(script, "menu Bootforge Boot Menu") {
			t.Error("script should contain menu header")
		}
		if !strings.Contains(script, "rescue") {
			t.Error("script should contain rescue entry")
		}
		if !strings.Contains(script, "local-disk") {
			t.Error("script should contain local-disk entry")
		}
		if !strings.Contains(script, fmt.Sprintf("127.0.0.1:%d", httpPort)) {
			t.Error("script should contain server address")
		}
	})

	// === Test 3: HTTP boot file (kernel) ===
	t.Run("HTTP boot file", func(t *testing.T) {
		resp, err := http.Get(httpBase + "/data/tools/rescue/vmlinuz")
		if err != nil {
			t.Fatalf("GET vmlinuz: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if string(body) != "RESCUE-KERNEL-BINARY" {
			t.Errorf("kernel = %q, want %q", string(body), "RESCUE-KERNEL-BINARY")
		}
	})

	// === Test 4: Boot override ===
	t.Run("boot override", func(t *testing.T) {
		registry.SetBootOverride(clientMAC, "rescue")

		// First request: should get override (direct boot).
		resp, err := http.Get(httpBase + "/boot/aa:bb:cc:dd:ee:01/menu.ipxe")
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if !strings.Contains(string(body), "One-time boot override") {
			t.Error("first request should be an override")
		}

		// Second request: override consumed, should get menu.
		resp2, _ := http.Get(httpBase + "/boot/aa:bb:cc:dd:ee:01/menu.ipxe")
		body2, _ := io.ReadAll(resp2.Body)
		resp2.Body.Close()

		if strings.Contains(string(body2), "One-time boot override") {
			t.Error("second request should NOT be an override (consumed)")
		}
		if !strings.Contains(string(body2), "menu Bootforge Boot Menu") {
			t.Error("second request should be a normal menu")
		}
	})

	// === Test 5: API status ===
	t.Run("API status", func(t *testing.T) {
		resp, err := http.Get(httpBase + "/api/v1/status")
		if err != nil {
			t.Fatalf("GET status: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}

		var status map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&status)
		if _, ok := status["version"]; !ok {
			t.Error("status should include version")
		}
	})

	// === Test 6: API clients ===
	t.Run("API clients", func(t *testing.T) {
		resp, err := http.Get(httpBase + "/api/v1/clients")
		if err != nil {
			t.Fatalf("GET clients: %v", err)
		}
		defer resp.Body.Close()

		var clients []map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&clients)
		if len(clients) < 2 {
			t.Errorf("expected at least 2 clients, got %d", len(clients))
		}
	})

	// === Test 7: API menus ===
	t.Run("API menus", func(t *testing.T) {
		resp, err := http.Get(httpBase + "/api/v1/menus")
		if err != nil {
			t.Fatalf("GET menus: %v", err)
		}
		defer resp.Body.Close()

		var menus []map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&menus)
		if len(menus) != 2 {
			t.Errorf("expected 2 menus, got %d", len(menus))
		}
	})

	// === Test 8: Health endpoint ===
	t.Run("healthz", func(t *testing.T) {
		resp, err := http.Get(httpBase + "/healthz")
		if err != nil {
			t.Fatalf("GET healthz: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if string(body) != "ok" {
			t.Errorf("body = %q, want %q", string(body), "ok")
		}
	})

	// === Test 9: Wildcard client ===
	t.Run("wildcard client", func(t *testing.T) {
		resp, err := http.Get(httpBase + "/boot/11:22:33:44:55:66/menu.ipxe")
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "exit 0") {
			t.Error("wildcard client should get local-disk (exit) menu")
		}
	})
}
