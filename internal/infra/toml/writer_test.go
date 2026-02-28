package toml

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bootforge/internal/domain"
)

func TestWriteConfigRoundTrip(t *testing.T) {
	original := &domain.FullConfig{
		Server: domain.ServerConfig{
			Interface: "eth0",
			DataDir:   "./data",
			Logging:   domain.LogConfig{Level: "info", Format: "pretty"},
		},
		DHCPProxy: domain.DHCPProxyConfig{
			Enabled:   true,
			Port:      67,
			ProxyPort: 4011,
		},
		TFTP: domain.TFTPConfig{
			Enabled:   true,
			Port:      69,
			BlockSize: 512,
			Timeout:   5 * time.Second,
		},
		HTTP: domain.HTTPConfig{
			Enabled:     true,
			Port:        8080,
			ReadTimeout: 30 * time.Second,
		},
		Health: domain.HealthConfig{
			Enabled:      true,
			Interval:     30 * time.Second,
			StartupCheck: true,
		},
		Bootloader: domain.BootloaderConfig{
			Dir:      "bootloader",
			UEFX64:   "ipxe.efi",
			BIOS:     "undionly.kpxe",
			ChainURL: "http://${server_ip}:${http_port}/boot/${mac}/menu.ipxe",
		},
		Menus: []*domain.MenuEntry{
			{
				Name:  "rescue",
				Label: "Rescue System",
				Type:  domain.MenuLive,
				Boot:  domain.BootParams{Kernel: "vmlinuz", Initrd: "initrd"},
				HTTP:  domain.MenuHTTP{Path: "/tools/rescue/"},
			},
			{
				Name:  "local-disk",
				Label: "Boot from local disk",
				Type:  domain.MenuExit,
			},
		},
		Clients: []*domain.Client{
			{
				MAC:     net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01},
				Name:    "server-1",
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
		},
	}

	dir := t.TempDir()

	// Write.
	if err := WriteConfig(original, dir); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	// Verify file exists.
	path := filepath.Join(dir, "bootforge.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written config: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("written config is empty")
	}

	// Read back via LoadDir (round-trip).
	loaded, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir after write: %v", err)
	}

	// Verify key fields survived the round-trip.
	if loaded.Server.Interface != "eth0" {
		t.Errorf("interface = %q, want eth0", loaded.Server.Interface)
	}
	wantDataDir := filepath.Join(dir, "data")
	if loaded.Server.DataDir != wantDataDir {
		t.Errorf("data_dir = %q, want %q", loaded.Server.DataDir, wantDataDir)
	}
	if !loaded.TFTP.Enabled {
		t.Error("tftp should be enabled")
	}
	if loaded.TFTP.Port != 69 {
		t.Errorf("tftp port = %d, want 69", loaded.TFTP.Port)
	}
	if loaded.HTTP.Port != 8080 {
		t.Errorf("http port = %d, want 8080", loaded.HTTP.Port)
	}
	if loaded.Bootloader.UEFX64 != "ipxe.efi" {
		t.Errorf("uefi_x64 = %q, want ipxe.efi", loaded.Bootloader.UEFX64)
	}
	if loaded.Bootloader.ChainURL != "http://${server_ip}:${http_port}/boot/${mac}/menu.ipxe" {
		t.Errorf("chain_url = %q", loaded.Bootloader.ChainURL)
	}
	if len(loaded.Menus) != 2 {
		t.Fatalf("menus = %d, want 2", len(loaded.Menus))
	}
	if loaded.Menus[0].Name != "rescue" {
		t.Errorf("menu[0].name = %q, want rescue", loaded.Menus[0].Name)
	}
	if loaded.Menus[0].Boot.Kernel != "vmlinuz" {
		t.Errorf("menu[0].kernel = %q, want vmlinuz", loaded.Menus[0].Boot.Kernel)
	}
	if loaded.Menus[1].Type != domain.MenuExit {
		t.Errorf("menu[1].type = %v, want exit", loaded.Menus[1].Type)
	}
	if len(loaded.Clients) != 2 {
		t.Fatalf("clients = %d, want 2", len(loaded.Clients))
	}
	if loaded.Clients[0].Name != "server-1" {
		t.Errorf("client[0].name = %q, want server-1", loaded.Clients[0].Name)
	}
	if loaded.Clients[0].Menu.Default != "rescue" {
		t.Errorf("client[0].default = %q, want rescue", loaded.Clients[0].Menu.Default)
	}
	if !loaded.Clients[1].IsWildcard() {
		t.Error("client[1] should be wildcard")
	}
}

func TestWriteConfigValidatesOnReload(t *testing.T) {
	cfg := &domain.FullConfig{
		Server: domain.ServerConfig{
			Interface: "lo",
			DataDir:   "./data",
		},
		Bootloader: domain.BootloaderConfig{
			Dir:      "bootloader",
			ChainURL: "http://localhost/boot/${mac}/menu.ipxe",
		},
		Menus: []*domain.MenuEntry{
			{Name: "local-disk", Label: "Local Disk", Type: domain.MenuExit},
		},
		Clients: []*domain.Client{
			{
				MAC: domain.WildcardMAC, Name: "default", Enabled: true,
				Menu: domain.MenuConfig{Entries: []string{"local-disk"}},
			},
		},
	}

	dir := t.TempDir()
	if err := WriteConfig(cfg, dir); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	loaded, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if err := loaded.Validate(); err != nil {
		t.Errorf("loaded config should validate: %v", err)
	}
}
