package httpboot

import (
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bootforge/internal/domain"
	"bootforge/internal/server"
)

func testSetup() (*server.Registry, *server.MenuResolver, *server.IPXEGenerator, server.IPXEVars) {
	menus := []*domain.MenuEntry{
		{
			Name:  "rescue",
			Label: "Rescue System",
			Type:  domain.MenuLive,
			HTTP:  domain.MenuHTTP{Path: "/tools/rescue/"},
			Boot:  domain.BootParams{Kernel: "vmlinuz", Initrd: "initrd"},
		},
		{
			Name:  "ubuntu-install",
			Label: "Ubuntu Install",
			Type:  domain.MenuInstall,
			HTTP:  domain.MenuHTTP{Path: "/installers/ubuntu/"},
			Boot:  domain.BootParams{Kernel: "vmlinuz", Initrd: "initrd", Cmdline: "auto url=http://${server_ip}:${http_port}/preseed/${mac}"},
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
			Name:    "server-1",
			Enabled: true,
			Menu: domain.MenuConfig{
				Entries: []string{"rescue", "ubuntu-install", "local-disk"},
				Default: "rescue",
				Timeout: 10,
			},
		},
		{
			MAC:     net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x02},
			Name:    "disabled-server",
			Enabled: false,
			Menu: domain.MenuConfig{
				Entries: []string{"rescue"},
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
	vars := server.IPXEVars{
		ServerIP: "192.168.1.10",
		HTTPPort: 8080,
	}

	return registry, resolver, ipxe, vars
}

func TestMenuScriptKnownMAC(t *testing.T) {
	registry, resolver, ipxe, vars := testSetup()
	handler := NewMenuScriptHandler(registry, resolver, ipxe, vars, slog.Default())

	req := httptest.NewRequest("GET", "/boot/aa:bb:cc:dd:ee:01/menu.ipxe", nil)
	req.SetPathValue("mac", "aa:bb:cc:dd:ee:01")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
	body := rec.Body.String()
	if !strings.HasPrefix(body, "#!ipxe") {
		t.Errorf("body should start with #!ipxe, got %q", body[:min(50, len(body))])
	}
	if !strings.Contains(body, "menu Bootforge Boot Menu") {
		t.Error("body should contain menu header")
	}
	if !strings.Contains(body, "rescue") {
		t.Error("body should contain rescue entry")
	}
	if !strings.Contains(body, "ubuntu-install") {
		t.Error("body should contain ubuntu-install entry")
	}
	if !strings.Contains(body, "local-disk") {
		t.Error("body should contain local-disk entry")
	}
}

func TestMenuScriptUnknownMACWithWildcard(t *testing.T) {
	registry, resolver, ipxe, vars := testSetup()
	handler := NewMenuScriptHandler(registry, resolver, ipxe, vars, slog.Default())

	req := httptest.NewRequest("GET", "/boot/11:22:33:44:55:66/menu.ipxe", nil)
	req.SetPathValue("mac", "11:22:33:44:55:66")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	// Wildcard client has only "local-disk" (exit type) — should produce menu with exit.
	if !strings.HasPrefix(body, "#!ipxe") {
		t.Error("body should start with #!ipxe")
	}
	if !strings.Contains(body, "exit 0") {
		t.Error("wildcard default should produce exit command")
	}
}

func TestMenuScriptUnknownMACNoWildcard(t *testing.T) {
	// Create registry without wildcard.
	clients := []*domain.Client{
		{
			MAC:     net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01},
			Name:    "server-1",
			Enabled: true,
			Menu: domain.MenuConfig{
				Entries: []string{"rescue"},
			},
		},
	}
	menus := []*domain.MenuEntry{
		{Name: "rescue", Label: "Rescue", Type: domain.MenuLive, Boot: domain.BootParams{Kernel: "vmlinuz"}},
	}

	registry := server.NewRegistry(clients)
	resolver := server.NewMenuResolver(menus)
	ipxe := server.NewIPXEGenerator()
	vars := server.IPXEVars{ServerIP: "192.168.1.10", HTTPPort: 8080}
	handler := NewMenuScriptHandler(registry, resolver, ipxe, vars, slog.Default())

	req := httptest.NewRequest("GET", "/boot/11:22:33:44:55:66/menu.ipxe", nil)
	req.SetPathValue("mac", "11:22:33:44:55:66")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestMenuScriptInvalidMAC(t *testing.T) {
	registry, resolver, ipxe, vars := testSetup()
	handler := NewMenuScriptHandler(registry, resolver, ipxe, vars, slog.Default())

	invalidMACs := []string{"not-a-mac", "zz:zz:zz:zz:zz:zz", "123", ""}
	for _, mac := range invalidMACs {
		t.Run(mac, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/boot/"+mac+"/menu.ipxe", nil)
			req.SetPathValue("mac", mac)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d for mac %q", rec.Code, http.StatusBadRequest, mac)
			}
		})
	}
}

func TestMenuScriptDisabledClient(t *testing.T) {
	registry, resolver, ipxe, vars := testSetup()
	handler := NewMenuScriptHandler(registry, resolver, ipxe, vars, slog.Default())

	req := httptest.NewRequest("GET", "/boot/aa:bb:cc:dd:ee:02/menu.ipxe", nil)
	req.SetPathValue("mac", "aa:bb:cc:dd:ee:02")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestMenuScriptOverride(t *testing.T) {
	registry, resolver, ipxe, vars := testSetup()
	handler := NewMenuScriptHandler(registry, resolver, ipxe, vars, slog.Default())

	mac := net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01}
	registry.SetBootOverride(mac, "ubuntu-install")

	// First request: should get override (direct boot).
	req := httptest.NewRequest("GET", "/boot/aa:bb:cc:dd:ee:01/menu.ipxe", nil)
	req.SetPathValue("mac", "aa:bb:cc:dd:ee:01")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "One-time boot override") {
		t.Error("first request should be an override script")
	}

	// Second request: override consumed, should get normal menu.
	req2 := httptest.NewRequest("GET", "/boot/aa:bb:cc:dd:ee:01/menu.ipxe", nil)
	req2.SetPathValue("mac", "aa:bb:cc:dd:ee:01")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec2.Code, http.StatusOK)
	}
	body2 := rec2.Body.String()
	if strings.Contains(body2, "One-time boot override") {
		t.Error("second request should NOT be an override (consumed)")
	}
	if !strings.Contains(body2, "menu Bootforge Boot Menu") {
		t.Error("second request should be a normal menu")
	}
}

func TestMenuScriptVariableSubstitution(t *testing.T) {
	registry, resolver, ipxe, vars := testSetup()
	handler := NewMenuScriptHandler(registry, resolver, ipxe, vars, slog.Default())

	req := httptest.NewRequest("GET", "/boot/aa:bb:cc:dd:ee:01/menu.ipxe", nil)
	req.SetPathValue("mac", "aa:bb:cc:dd:ee:01")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	// The ubuntu-install entry has cmdline with ${server_ip}, ${http_port}, ${mac}.
	if strings.Contains(body, "${server_ip}") {
		t.Error("${server_ip} should be substituted")
	}
	if strings.Contains(body, "${http_port}") {
		t.Error("${http_port} should be substituted")
	}
	if strings.Contains(body, "${mac}") {
		t.Error("${mac} should be substituted")
	}
	if !strings.Contains(body, "192.168.1.10") {
		t.Error("body should contain server IP")
	}
	if !strings.Contains(body, "8080") {
		t.Error("body should contain HTTP port")
	}
	if !strings.Contains(body, "aa:bb:cc:dd:ee:01") {
		t.Error("body should contain client MAC")
	}
}

func TestMenuScriptContentType(t *testing.T) {
	registry, resolver, ipxe, vars := testSetup()
	handler := NewMenuScriptHandler(registry, resolver, ipxe, vars, slog.Default())

	req := httptest.NewRequest("GET", "/boot/aa:bb:cc:dd:ee:01/menu.ipxe", nil)
	req.SetPathValue("mac", "aa:bb:cc:dd:ee:01")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/plain; charset=utf-8")
	}
}
