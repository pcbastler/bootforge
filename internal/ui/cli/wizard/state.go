package wizard

import (
	"net"
	"time"

	"bootforge/internal/domain"
)

// WizardState holds all values collected during the interactive wizard.
// It is an intermediate representation between the TUI and domain.FullConfig.
type WizardState struct {
	ConfigDir string

	// Server
	Interface string
	ServerIP  string
	DataDir   string

	// Services
	DHCPEnabled   bool
	DHCPPort      int
	DHCPProxyPort int
	TFTPEnabled   bool
	TFTPPort      int
	HTTPEnabled   bool
	HTTPPort      int

	// Bootloader
	DownloadIPXE  bool
	IPXEVariant   IPXEVariant // "full" or "snponly"
	IPXEBaseURL   string
	IPXEArchs     []string // "uefi_x64", "uefi_x86", "bios", "arm64"
	BootloaderDir string

	// Menus
	Menus []MenuState

	// Clients
	Clients []ClientState
}

// MenuState holds wizard data for a single menu entry.
type MenuState struct {
	Name        string
	Label       string
	Type        string // "install", "live", "tool", "exit"
	Kernel      string
	Initrd      string
	Cmdline     string
	HTTPPath    string
	HTTPFiles   string
}

// ClientState holds wizard data for a single client.
type ClientState struct {
	MAC     string
	Name    string
	Entries []string
	Default string
	Timeout int
}

// DefaultState returns a WizardState pre-filled with sensible defaults.
func DefaultState() *WizardState {
	return &WizardState{
		DataDir:       "./data",
		DHCPEnabled:   true,
		DHCPPort:      67,
		DHCPProxyPort: 4011,
		TFTPEnabled:   true,
		TFTPPort:      69,
		HTTPEnabled:   true,
		HTTPPort:      8080,
		IPXEVariant:   IPXEVariantFull,
		IPXEBaseURL:   DefaultIPXEBaseURL,
		BootloaderDir: "bootloader",
		Menus: []MenuState{
			{Name: "local-disk", Label: "Boot from local disk", Type: "exit"},
		},
		Clients: []ClientState{
			{MAC: "*", Name: "default", Entries: []string{"local-disk"}},
		},
	}
}

// StateToConfig converts a WizardState into a domain.FullConfig.
func StateToConfig(s *WizardState) *domain.FullConfig {
	cfg := &domain.FullConfig{
		Server: domain.ServerConfig{
			Interface: s.Interface,
			DataDir:   s.DataDir,
			Logging:   domain.LogConfig{Level: "info", Format: "pretty"},
		},
		DHCPProxy: domain.DHCPProxyConfig{
			Enabled:   s.DHCPEnabled,
			Port:      s.DHCPPort,
			ProxyPort: s.DHCPProxyPort,
		},
		TFTP: domain.TFTPConfig{
			Enabled:   s.TFTPEnabled,
			Port:      s.TFTPPort,
			BlockSize: 512,
			Timeout:   5 * time.Second,
		},
		HTTP: domain.HTTPConfig{
			Enabled:     s.HTTPEnabled,
			Port:        s.HTTPPort,
			ReadTimeout: 30 * time.Second,
		},
		Health: domain.HealthConfig{
			Enabled:      true,
			Interval:     30 * time.Second,
			StartupCheck: true,
		},
		Bootloader: domain.BootloaderConfig{
			Dir:      s.BootloaderDir,
			ChainURL: "http://${server_ip}:${http_port}/boot/${mac}/menu.ipxe",
		},
	}

	// Map downloaded architectures to bootloader filenames.
	for _, arch := range s.IPXEArchs {
		switch arch {
		case "uefi_x64":
			cfg.Bootloader.UEFX64 = "ipxe.efi"
		case "uefi_x86":
			cfg.Bootloader.UEFX86 = "ipxe-i386.efi"
		case "bios":
			cfg.Bootloader.BIOS = "undionly.kpxe"
		case "arm64":
			cfg.Bootloader.ARM64 = "ipxe-arm64.efi"
		}
	}

	// Menus.
	for _, m := range s.Menus {
		mt, _ := domain.ParseMenuType(m.Type)
		entry := &domain.MenuEntry{
			Name:  m.Name,
			Label: m.Label,
			Type:  mt,
			Boot: domain.BootParams{
				Kernel:  m.Kernel,
				Initrd:  m.Initrd,
				Cmdline: m.Cmdline,
			},
			HTTP: domain.MenuHTTP{
				Path:  m.HTTPPath,
				Files: m.HTTPFiles,
			},
		}
		cfg.Menus = append(cfg.Menus, entry)
	}

	// Clients.
	for _, c := range s.Clients {
		mac, _ := domain.ParseMAC(c.MAC)
		client := &domain.Client{
			MAC:     mac,
			Name:    c.Name,
			Enabled: true,
			Menu: domain.MenuConfig{
				Entries: c.Entries,
				Default: c.Default,
				Timeout: c.Timeout,
			},
		}
		cfg.Clients = append(cfg.Clients, client)
	}

	return cfg
}

// ConfigToState converts a domain.FullConfig into a WizardState for editing.
func ConfigToState(cfg *domain.FullConfig) *WizardState {
	s := &WizardState{
		Interface:     cfg.Server.Interface,
		DataDir:       cfg.Server.DataDir,
		DHCPEnabled:   cfg.DHCPProxy.Enabled,
		DHCPPort:      cfg.DHCPProxy.Port,
		DHCPProxyPort: cfg.DHCPProxy.ProxyPort,
		TFTPEnabled:   cfg.TFTP.Enabled,
		TFTPPort:      cfg.TFTP.Port,
		HTTPEnabled:   cfg.HTTP.Enabled,
		HTTPPort:      cfg.HTTP.Port,
		BootloaderDir: cfg.Bootloader.Dir,
		IPXEVariant:   IPXEVariantFull,
		IPXEBaseURL:   DefaultIPXEBaseURL,
	}

	// Reconstruct architecture list from bootloader filenames.
	if cfg.Bootloader.UEFX64 != "" {
		s.IPXEArchs = append(s.IPXEArchs, "uefi_x64")
	}
	if cfg.Bootloader.UEFX86 != "" {
		s.IPXEArchs = append(s.IPXEArchs, "uefi_x86")
	}
	if cfg.Bootloader.BIOS != "" {
		s.IPXEArchs = append(s.IPXEArchs, "bios")
	}
	if cfg.Bootloader.ARM64 != "" {
		s.IPXEArchs = append(s.IPXEArchs, "arm64")
	}

	// Menus.
	for _, m := range cfg.Menus {
		s.Menus = append(s.Menus, MenuState{
			Name:      m.Name,
			Label:     m.Label,
			Type:      m.Type.String(),
			Kernel:    m.Boot.Kernel,
			Initrd:    m.Boot.Initrd,
			Cmdline:   m.Boot.Cmdline,
			HTTPPath:  m.HTTP.Path,
			HTTPFiles: m.HTTP.Files,
		})
	}

	// Clients.
	for _, c := range cfg.Clients {
		mac := c.MAC.String()
		if c.IsWildcard() {
			mac = "*"
		}
		s.Clients = append(s.Clients, ClientState{
			MAC:     mac,
			Name:    c.Name,
			Entries: c.Menu.Entries,
			Default: c.Menu.Default,
			Timeout: c.Menu.Timeout,
		})
	}

	return s
}

// MenuNames returns the names of all menus in the state.
func (s *WizardState) MenuNames() []string {
	names := make([]string, len(s.Menus))
	for i, m := range s.Menus {
		names[i] = m.Name
	}
	return names
}

// IPXEDownloadArchs returns the IPXEArch list for the selected architectures.
func (s *WizardState) IPXEDownloadArchs() []IPXEArch {
	all := IPXEArchitectures(s.IPXEBaseURL, s.IPXEVariant)
	archMap := map[string]int{
		"uefi_x64": 0,
		"uefi_x86": 1,
		"bios":     2,
		"arm64":    3,
	}
	var result []IPXEArch
	for _, sel := range s.IPXEArchs {
		if idx, ok := archMap[sel]; ok && idx < len(all) {
			result = append(result, all[idx])
		}
	}
	return result
}

// ServerIPForConfig tries to determine the server IP from the selected interface.
func (s *WizardState) ServerIPForConfig() string {
	if s.ServerIP != "" {
		return s.ServerIP
	}
	// Try to detect from interface.
	iface, err := net.InterfaceByName(s.Interface)
	if err != nil {
		return "0.0.0.0"
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return "0.0.0.0"
	}
	ip := firstIPv4(addrs)
	if ip == "" {
		return "0.0.0.0"
	}
	return ip
}
