package domain

import (
	"fmt"
	"time"
)

// LogConfig controls structured logging behavior.
type LogConfig struct {
	Level  string // debug, info, warn, error
	Format string // pretty, json
}

// ServerConfig holds global server settings.
type ServerConfig struct {
	Interface string    // network interface to bind to
	DataDir   string    // root directory for boot files
	Logging   LogConfig // logging configuration
}

// Validate checks that the server config is well-formed.
func (c *ServerConfig) Validate() error {
	if c.Interface == "" {
		return fmt.Errorf("server config: interface is required")
	}
	if c.DataDir == "" {
		return fmt.Errorf("server config: data_dir is required")
	}
	return nil
}

// DHCPProxyConfig controls the DHCP Proxy service.
type DHCPProxyConfig struct {
	Enabled    bool
	Port       int    // default: 67
	ProxyPort  int    // default: 4011
	VendorClass string // default: "PXEClient"
}

// Validate checks that the DHCP proxy config is well-formed.
func (c *DHCPProxyConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if err := validatePort(c.Port, "dhcp_proxy.port"); err != nil {
		return err
	}
	if err := validatePort(c.ProxyPort, "dhcp_proxy.proxy_port"); err != nil {
		return err
	}
	return nil
}

// TFTPConfig controls the TFTP service.
type TFTPConfig struct {
	Enabled   bool
	Port      int           // default: 69
	BlockSize int           // default: 512
	Timeout   time.Duration // default: 5s
	Retries   int           // default: 3
}

// Validate checks that the TFTP config is well-formed.
func (c *TFTPConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if err := validatePort(c.Port, "tftp.port"); err != nil {
		return err
	}
	return nil
}

// TLSConfig holds TLS certificate paths.
type TLSConfig struct {
	Enabled bool
	Cert    string
	Key     string
}

// HTTPConfig controls the HTTP boot service.
type HTTPConfig struct {
	Enabled     bool
	Port        int           // default: 8080
	ReadTimeout time.Duration // default: 30s
	TLS         TLSConfig
}

// Validate checks that the HTTP config is well-formed.
func (c *HTTPConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if err := validatePort(c.Port, "http.port"); err != nil {
		return err
	}
	return nil
}

// HealthConfig controls the health check system.
type HealthConfig struct {
	Enabled      bool
	Interval     time.Duration // default: 30s
	StartupCheck bool          // run checks before starting services
}

// BootloaderConfig defines the bootloader files and chain URL.
type BootloaderConfig struct {
	Dir      string // path to bootloader directory (relative to DataDir)
	UEFX64   string // filename for UEFI x64 (e.g. "ipxe.efi")
	UEFX86   string // filename for UEFI x86 (e.g. "ipxe-x86.efi")
	BIOS     string // filename for BIOS (e.g. "undionly.kpxe")
	ARM64    string // filename for ARM64 (e.g. "ipxe-arm64.efi")
	ChainURL string // iPXE chain URL template
}

// Validate checks that the bootloader config is well-formed.
func (c *BootloaderConfig) Validate() error {
	if c.Dir == "" {
		return fmt.Errorf("bootloader config: dir is required")
	}
	if c.ChainURL == "" {
		return fmt.Errorf("bootloader config: chain_url is required")
	}
	return nil
}

// FileForArch returns the bootloader filename for a given architecture.
func (c *BootloaderConfig) FileForArch(arch ClientArch) string {
	switch arch {
	case ArchUEFIx64:
		return c.UEFX64
	case ArchUEFIx86:
		return c.UEFX86
	case ArchBIOS:
		return c.BIOS
	case ArchARM64:
		return c.ARM64
	default:
		return ""
	}
}

// FullConfig is the top-level aggregation of all configuration.
type FullConfig struct {
	Server     ServerConfig
	DHCPProxy  DHCPProxyConfig
	TFTP       TFTPConfig
	HTTP       HTTPConfig
	Health     HealthConfig
	Bootloader BootloaderConfig
	Menus      []*MenuEntry
	Clients    []*Client
}

// Validate checks all configuration for consistency.
func (c *FullConfig) Validate() error {
	if err := c.Server.Validate(); err != nil {
		return err
	}
	if err := c.DHCPProxy.Validate(); err != nil {
		return err
	}
	if err := c.TFTP.Validate(); err != nil {
		return err
	}
	if err := c.HTTP.Validate(); err != nil {
		return err
	}
	if err := c.Bootloader.Validate(); err != nil {
		return err
	}
	for _, m := range c.Menus {
		if err := m.Validate(); err != nil {
			return err
		}
	}
	for _, cl := range c.Clients {
		if err := cl.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func validatePort(port int, field string) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%s: port must be between 1 and 65535, got %d", field, port)
	}
	return nil
}
