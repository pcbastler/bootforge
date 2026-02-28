package httpboot

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"bootforge/internal/domain"
	"bootforge/internal/server"
)

func startTestHTTPServer(t *testing.T) (baseURL string, cleanup func()) {
	t.Helper()

	// Create temp data directory with boot files.
	dataDir := t.TempDir()
	toolDir := filepath.Join(dataDir, "tools", "rescue")
	os.MkdirAll(toolDir, 0755)
	os.WriteFile(filepath.Join(toolDir, "vmlinuz"), []byte("RESCUE-KERNEL"), 0644)
	os.WriteFile(filepath.Join(toolDir, "initrd"), []byte("RESCUE-INITRD"), 0644)

	// Setup server components.
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
	clients := []*domain.Client{
		{
			MAC:     net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01},
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

	registry := server.NewRegistry(clients)
	resolver := server.NewMenuResolver(menus)
	ipxe := server.NewIPXEGenerator()
	logger := slog.Default()

	// Listen on ephemeral port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	port := ln.Addr().(*net.TCPAddr).Port
	vars := server.IPXEVars{
		ServerIP: "127.0.0.1",
		HTTPPort: port,
	}

	mux := http.NewServeMux()
	handler := NewMenuScriptHandler(registry, resolver, ipxe, vars, logger)
	SetupRoutes(mux, handler, dataDir, logger)

	cfg := domain.HTTPConfig{
		Enabled: true,
		Port:    port,
	}
	srv := NewHTTPServer(cfg, mux, logger)

	go srv.StartOnListener(ln)

	return fmt.Sprintf("http://127.0.0.1:%d", port), func() {
		srv.Stop()
	}
}

func TestHTTPServerMenuScript(t *testing.T) {
	baseURL, cleanup := startTestHTTPServer(t)
	defer cleanup()

	resp, err := http.Get(baseURL + "/boot/aa:bb:cc:dd:ee:01/menu.ipxe")
	if err != nil {
		t.Fatalf("GET menu.ipxe: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.HasPrefix(string(body), "#!ipxe") {
		t.Error("body should start with #!ipxe")
	}
}

func TestHTTPServerStaticFiles(t *testing.T) {
	baseURL, cleanup := startTestHTTPServer(t)
	defer cleanup()

	resp, err := http.Get(baseURL + "/data/tools/rescue/vmlinuz")
	if err != nil {
		t.Fatalf("GET vmlinuz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "RESCUE-KERNEL" {
		t.Errorf("body = %q, want %q", string(body), "RESCUE-KERNEL")
	}
}

func TestHTTPServerHealthz(t *testing.T) {
	baseURL, cleanup := startTestHTTPServer(t)
	defer cleanup()

	resp, err := http.Get(baseURL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Errorf("body = %q, want %q", string(body), "ok")
	}
}

func TestHTTPServerPathTraversal(t *testing.T) {
	baseURL, cleanup := startTestHTTPServer(t)
	defer cleanup()

	paths := []string{
		"/data/../etc/passwd",
		"/data/../../etc/shadow",
		"/data/tools/../../etc/hosts",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			resp, err := http.Get(baseURL + path)
			if err != nil {
				t.Fatalf("GET %s: %v", path, err)
			}
			defer resp.Body.Close()

			// Should be 301 (redirect to cleaned path), 403, or 404 — never 200 with /etc content.
			if resp.StatusCode == http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				bodyStr := string(body)
				if strings.Contains(bodyStr, "root:") || strings.Contains(bodyStr, "shadow") {
					t.Errorf("path traversal succeeded for %s, got system file content", path)
				}
			}
		})
	}
}

func TestHTTPServerNotFound(t *testing.T) {
	baseURL, cleanup := startTestHTTPServer(t)
	defer cleanup()

	resp, err := http.Get(baseURL + "/data/nonexistent/file.txt")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestHTTPServerConcurrentRequests(t *testing.T) {
	baseURL, cleanup := startTestHTTPServer(t)
	defer cleanup()

	var wg sync.WaitGroup
	paths := []string{
		"/boot/aa:bb:cc:dd:ee:01/menu.ipxe",
		"/data/tools/rescue/vmlinuz",
		"/healthz",
	}

	for i := 0; i < 5; i++ {
		for _, p := range paths {
			wg.Add(1)
			go func(path string) {
				defer wg.Done()
				resp, err := http.Get(baseURL + path)
				if err != nil {
					t.Errorf("GET %s: %v", path, err)
					return
				}
				resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					t.Errorf("GET %s: status = %d, want %d", path, resp.StatusCode, http.StatusOK)
				}
			}(p)
		}
	}
	wg.Wait()
}
