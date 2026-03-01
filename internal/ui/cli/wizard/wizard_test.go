package wizard

import (
	"net"
	"testing"
	"time"

	"bootforge/internal/domain"
	"bootforge/internal/infra/toml"
)

func TestDefaultState(t *testing.T) {
	s := DefaultState()

	if s.DataDir != "./data" {
		t.Errorf("DataDir = %q, want ./data", s.DataDir)
	}
	if !s.DHCPEnabled {
		t.Error("DHCP should be enabled by default")
	}
	if s.DHCPPort != 67 {
		t.Errorf("DHCPPort = %d, want 67", s.DHCPPort)
	}
	if s.TFTPPort != 69 {
		t.Errorf("TFTPPort = %d, want 69", s.TFTPPort)
	}
	if s.HTTPPort != 8080 {
		t.Errorf("HTTPPort = %d, want 8080", s.HTTPPort)
	}
	if len(s.Menus) != 1 || s.Menus[0].Name != "local-disk" {
		t.Error("should have local-disk menu by default")
	}
	if len(s.Clients) != 1 || s.Clients[0].MAC != "*" {
		t.Error("should have wildcard client by default")
	}
}

func TestStateToConfigRoundTrip(t *testing.T) {
	state := &WizardState{
		Interface:     "eth0",
		DataDir:       "./data",
		DHCPEnabled:   true,
		DHCPPort:      67,
		DHCPProxyPort: 4011,
		TFTPEnabled:   true,
		TFTPPort:      69,
		HTTPEnabled:   true,
		HTTPPort:      8080,
		BootloaderDir: "bootloader",
		IPXEArchs:     []string{"uefi_x64", "bios"},
		Menus: []MenuState{
			{Name: "rescue", Label: "Rescue System", Type: "live", Kernel: "vmlinuz", Initrd: "initrd"},
			{Name: "local-disk", Label: "Boot from local disk", Type: "exit"},
		},
		Clients: []ClientState{
			{MAC: "aa:bb:cc:dd:ee:01", Name: "server-1", Entries: []string{"rescue", "local-disk"}, Default: "rescue", Timeout: 10},
			{MAC: "*", Name: "default", Entries: []string{"local-disk"}},
		},
	}

	cfg := StateToConfig(state)

	// Verify domain config.
	if cfg.Server.Interface != "eth0" {
		t.Errorf("Interface = %q", cfg.Server.Interface)
	}
	if !cfg.DHCPProxy.Enabled {
		t.Error("DHCP should be enabled")
	}
	if cfg.DHCPProxy.Port != 67 {
		t.Errorf("DHCPPort = %d", cfg.DHCPProxy.Port)
	}
	if cfg.Bootloader.UEFX64 != "ipxe.efi" {
		t.Errorf("UEFX64 = %q", cfg.Bootloader.UEFX64)
	}
	if cfg.Bootloader.BIOS != "undionly.kpxe" {
		t.Errorf("BIOS = %q", cfg.Bootloader.BIOS)
	}
	if cfg.Bootloader.UEFX86 != "" {
		t.Errorf("UEFX86 should be empty, got %q", cfg.Bootloader.UEFX86)
	}
	if len(cfg.Menus) != 2 {
		t.Fatalf("Menus = %d, want 2", len(cfg.Menus))
	}
	if cfg.Menus[0].Name != "rescue" {
		t.Errorf("Menu[0].Name = %q", cfg.Menus[0].Name)
	}
	if cfg.Menus[0].Type != domain.MenuLive {
		t.Errorf("Menu[0].Type = %v, want live", cfg.Menus[0].Type)
	}
	if cfg.Menus[1].Type != domain.MenuExit {
		t.Errorf("Menu[1].Type = %v, want exit", cfg.Menus[1].Type)
	}
	if len(cfg.Clients) != 2 {
		t.Fatalf("Clients = %d, want 2", len(cfg.Clients))
	}
	if cfg.Clients[0].Name != "server-1" {
		t.Errorf("Client[0].Name = %q", cfg.Clients[0].Name)
	}
	if !cfg.Clients[1].IsWildcard() {
		t.Error("Client[1] should be wildcard")
	}
}

func TestConfigToState(t *testing.T) {
	cfg := &domain.FullConfig{
		Server: domain.ServerConfig{
			Interface: "enp3s0",
			DataDir:   "/srv/bootforge",
		},
		DHCPProxy: domain.DHCPProxyConfig{
			Enabled:   true,
			Port:      67,
			ProxyPort: 4011,
		},
		TFTP: domain.TFTPConfig{
			Enabled: true,
			Port:    69,
		},
		HTTP: domain.HTTPConfig{
			Enabled: true,
			Port:    9090,
		},
		Bootloader: domain.BootloaderConfig{
			Dir:    "bl",
			UEFX64: "ipxe.efi",
			BIOS:   "undionly.kpxe",
		},
		Menus: []*domain.MenuEntry{
			{Name: "ubuntu", Label: "Ubuntu 24.04", Type: domain.MenuInstall,
				Boot: domain.BootParams{Kernel: "vmlinuz", Initrd: "initrd.gz", Cmdline: "auto=true"}},
			{Name: "local-disk", Label: "Local Disk", Type: domain.MenuExit},
		},
		Clients: []*domain.Client{
			{
				MAC: net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01},
				Name: "web-1", Enabled: true,
				Menu: domain.MenuConfig{Entries: []string{"ubuntu", "local-disk"}, Default: "ubuntu", Timeout: 15},
			},
			{
				MAC: domain.WildcardMAC, Name: "default", Enabled: true,
				Menu: domain.MenuConfig{Entries: []string{"local-disk"}},
			},
		},
	}

	state := ConfigToState(cfg)

	if state.Interface != "enp3s0" {
		t.Errorf("Interface = %q", state.Interface)
	}
	if state.DataDir != "/srv/bootforge" {
		t.Errorf("DataDir = %q", state.DataDir)
	}
	if state.HTTPPort != 9090 {
		t.Errorf("HTTPPort = %d", state.HTTPPort)
	}
	if len(state.IPXEArchs) != 2 {
		t.Fatalf("IPXEArchs = %v, want 2", state.IPXEArchs)
	}
	if state.BootloaderDir != "bl" {
		t.Errorf("BootloaderDir = %q", state.BootloaderDir)
	}
	if len(state.Menus) != 2 {
		t.Fatalf("Menus = %d", len(state.Menus))
	}
	if state.Menus[0].Cmdline != "auto=true" {
		t.Errorf("Menu[0].Cmdline = %q", state.Menus[0].Cmdline)
	}
	if len(state.Clients) != 2 {
		t.Fatalf("Clients = %d", len(state.Clients))
	}
	if state.Clients[0].Timeout != 15 {
		t.Errorf("Client[0].Timeout = %d", state.Clients[0].Timeout)
	}
	if state.Clients[1].MAC != "*" {
		t.Errorf("Client[1].MAC = %q, want *", state.Clients[1].MAC)
	}
}

func TestStateRoundTripThroughConfig(t *testing.T) {
	// State → Config → State should preserve key data.
	original := &WizardState{
		Interface:     "eth0",
		DataDir:       "./data",
		DHCPEnabled:   true,
		DHCPPort:      67,
		DHCPProxyPort: 4011,
		TFTPEnabled:   true,
		TFTPPort:      69,
		HTTPEnabled:   true,
		HTTPPort:      8080,
		BootloaderDir: "bootloader",
		IPXEArchs:     []string{"uefi_x64", "bios"},
		Menus: []MenuState{
			{Name: "local-disk", Label: "Boot from local disk", Type: "exit"},
		},
		Clients: []ClientState{
			{MAC: "*", Name: "default", Entries: []string{"local-disk"}},
		},
	}

	cfg := StateToConfig(original)
	result := ConfigToState(cfg)

	if result.Interface != original.Interface {
		t.Errorf("Interface = %q, want %q", result.Interface, original.Interface)
	}
	if result.DHCPPort != original.DHCPPort {
		t.Errorf("DHCPPort = %d, want %d", result.DHCPPort, original.DHCPPort)
	}
	if result.HTTPPort != original.HTTPPort {
		t.Errorf("HTTPPort = %d, want %d", result.HTTPPort, original.HTTPPort)
	}
	if len(result.Menus) != len(original.Menus) {
		t.Errorf("Menus = %d, want %d", len(result.Menus), len(original.Menus))
	}
	if len(result.Clients) != len(original.Clients) {
		t.Errorf("Clients = %d, want %d", len(result.Clients), len(original.Clients))
	}
}

func TestStateToConfigTOMLRoundTrip(t *testing.T) {
	// State → Config → TOML → LoadDir → Config — full pipeline.
	state := &WizardState{
		Interface:     "eth0",
		DataDir:       "./data",
		DHCPEnabled:   true,
		DHCPPort:      67,
		DHCPProxyPort: 4011,
		TFTPEnabled:   true,
		TFTPPort:      69,
		HTTPEnabled:   true,
		HTTPPort:      8080,
		BootloaderDir: "bootloader",
		IPXEArchs:     []string{"uefi_x64", "bios"},
		Menus: []MenuState{
			{Name: "rescue", Label: "Rescue System", Type: "live", Kernel: "vmlinuz", Initrd: "initrd"},
			{Name: "local-disk", Label: "Boot from local disk", Type: "exit"},
		},
		Clients: []ClientState{
			{MAC: "aa:bb:cc:dd:ee:01", Name: "server-1", Entries: []string{"rescue", "local-disk"}, Default: "rescue", Timeout: 10},
			{MAC: "*", Name: "default", Entries: []string{"local-disk"}},
		},
	}

	cfg := StateToConfig(state)
	dir := t.TempDir()

	if err := toml.WriteConfig(cfg, dir); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	loaded, err := toml.LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}

	if err := loaded.Validate(); err != nil {
		t.Errorf("validation: %v", err)
	}

	if loaded.Server.Interface != "eth0" {
		t.Errorf("Interface = %q", loaded.Server.Interface)
	}
	if loaded.HTTP.Port != 8080 {
		t.Errorf("HTTP port = %d", loaded.HTTP.Port)
	}
	if len(loaded.Menus) != 2 {
		t.Fatalf("Menus = %d", len(loaded.Menus))
	}
	if loaded.Menus[0].Boot.Kernel != "vmlinuz" {
		t.Errorf("kernel = %q", loaded.Menus[0].Boot.Kernel)
	}
	if len(loaded.Clients) != 2 {
		t.Fatalf("Clients = %d", len(loaded.Clients))
	}
	if loaded.Clients[0].Menu.Timeout != 10 {
		t.Errorf("timeout = %d", loaded.Clients[0].Menu.Timeout)
	}
}

func TestMenuNames(t *testing.T) {
	s := &WizardState{
		Menus: []MenuState{
			{Name: "ubuntu"},
			{Name: "rescue"},
			{Name: "local-disk"},
		},
	}

	names := s.MenuNames()
	if len(names) != 3 {
		t.Fatalf("len = %d", len(names))
	}
	if names[0] != "ubuntu" || names[1] != "rescue" || names[2] != "local-disk" {
		t.Errorf("names = %v", names)
	}
}

func TestIPXEDownloadArchs(t *testing.T) {
	s := &WizardState{
		IPXEVariant: IPXEVariantFull,
		IPXEBaseURL: "https://example.com",
		IPXEArchs:   []string{"uefi_x64", "arm64"},
	}

	archs := s.IPXEDownloadArchs()
	if len(archs) != 2 {
		t.Fatalf("len = %d", len(archs))
	}
	if archs[0].Filename != "ipxe.efi" {
		t.Errorf("arch[0] = %q", archs[0].Filename)
	}
	if archs[1].Filename != "ipxe-arm64.efi" {
		t.Errorf("arch[1] = %q", archs[1].Filename)
	}
	if archs[0].URL != "https://example.com/x86_64-efi/ipxe.efi" {
		t.Errorf("full variant URL = %q", archs[0].URL)
	}
}

func TestIPXEArchitectures(t *testing.T) {
	archs := IPXEArchitectures("https://boot.ipxe.org", IPXEVariantFull)
	if len(archs) != 4 {
		t.Fatalf("expected 4 archs, got %d", len(archs))
	}
	labels := []string{"UEFI x64", "UEFI x86", "BIOS", "ARM64"}
	for i, want := range labels {
		if archs[i].Label != want {
			t.Errorf("arch[%d].Label = %q, want %q", i, archs[i].Label, want)
		}
	}
}

func TestIPXEVariantURLs(t *testing.T) {
	base := "https://boot.ipxe.org"

	full := IPXEArchitectures(base, IPXEVariantFull)
	snp := IPXEArchitectures(base, IPXEVariantSNPOnly)

	// EFI architectures differ by variant.
	if full[0].URL != base+"/x86_64-efi/ipxe.efi" {
		t.Errorf("full UEFI x64 URL = %q", full[0].URL)
	}
	if snp[0].URL != base+"/x86_64-efi/snponly.efi" {
		t.Errorf("snp UEFI x64 URL = %q", snp[0].URL)
	}

	// Local filenames are the same regardless of variant.
	for i := range full {
		if full[i].Filename != snp[i].Filename {
			t.Errorf("arch[%d]: full filename %q != snp filename %q", i, full[i].Filename, snp[i].Filename)
		}
	}

	// BIOS URL is identical for both variants.
	if full[2].URL != snp[2].URL {
		t.Errorf("BIOS URLs differ: full=%q, snp=%q", full[2].URL, snp[2].URL)
	}
}

func TestDetectInterfaces(t *testing.T) {
	// Just verify it doesn't panic/error. The result depends on the host.
	ifaces, err := DetectInterfaces()
	if err != nil {
		t.Fatalf("DetectInterfaces: %v", err)
	}
	// In CI/containers there might be no non-loopback interfaces, that's ok.
	_ = ifaces
}

func TestNetInterfaceString(t *testing.T) {
	ni := NetInterface{Name: "eth0", IP: "192.168.1.10"}
	if s := ni.String(); s != "eth0 (192.168.1.10)" {
		t.Errorf("got %q", s)
	}

	ni2 := NetInterface{Name: "eth1"}
	if s := ni2.String(); s != "eth1" {
		t.Errorf("got %q", s)
	}
}

func TestStateToConfigDefaults(t *testing.T) {
	// Converting defaults should produce a valid config
	// (except missing interface which requires user input).
	state := DefaultState()
	state.Interface = "lo"

	cfg := StateToConfig(state)

	if cfg.Server.Logging.Level != "info" {
		t.Errorf("log level = %q", cfg.Server.Logging.Level)
	}
	if cfg.TFTP.BlockSize != 512 {
		t.Errorf("block size = %d", cfg.TFTP.BlockSize)
	}
	if cfg.TFTP.Timeout != 5*time.Second {
		t.Errorf("timeout = %v", cfg.TFTP.Timeout)
	}
	if cfg.Health.Interval != 30*time.Second {
		t.Errorf("health interval = %v", cfg.Health.Interval)
	}
}
