// Package toml scans the config directory and reads all .toml files.
// It detects content type by structure ([server], [[menu]], [[client]])
// and merges everything into the domain config types.
package toml

import (
	"fmt"
	"os"
	"time"

	"bootforge/internal/domain"

	toml "github.com/pelletier/go-toml/v2"
)

// rawFile represents the raw TOML structure of a single config file.
// Fields are optional — a file may contain any combination of sections.
type rawFile struct {
	Server     *rawServer     `toml:"server"`
	DHCPProxy  *rawDHCPProxy  `toml:"dhcp_proxy"`
	TFTP       *rawTFTP       `toml:"tftp"`
	HTTP       *rawHTTP       `toml:"http"`
	Health     *rawHealth     `toml:"health"`
	Bootloader *rawBootloader `toml:"bootloader"`
	Menu       []rawMenu      `toml:"menu"`
	Client     []rawClient    `toml:"client"`
}

type rawServer struct {
	Interface string     `toml:"interface"`
	DataDir   string     `toml:"data_dir"`
	Logging   rawLogging `toml:"logging"`
}

type rawLogging struct {
	Level  string `toml:"level"`
	Format string `toml:"format"`
}

type rawDHCPProxy struct {
	Enabled     *bool  `toml:"enabled"`
	Port        int    `toml:"port"`
	ProxyPort   int    `toml:"proxy_port"`
	VendorClass string `toml:"vendor_class"`
}

type rawTFTP struct {
	Enabled   *bool  `toml:"enabled"`
	Port      int    `toml:"port"`
	BlockSize int    `toml:"block_size"`
	Timeout   string `toml:"timeout"`
	Retries   int    `toml:"retries"`
}

type rawHTTP struct {
	Enabled     *bool  `toml:"enabled"`
	Port        int    `toml:"port"`
	ReadTimeout string `toml:"read_timeout"`
	TLS         *rawTLS `toml:"tls"`
}

type rawTLS struct {
	Enabled bool   `toml:"enabled"`
	Cert    string `toml:"cert"`
	Key     string `toml:"key"`
}

type rawHealth struct {
	Enabled      *bool  `toml:"enabled"`
	Interval     string `toml:"interval"`
	StartupCheck *bool  `toml:"startup_check"`
}

type rawBootloader struct {
	Dir      string `toml:"dir"`
	UEFX64   string `toml:"uefi_x64"`
	UEFX86   string `toml:"uefi_x86"`
	BIOS     string `toml:"bios"`
	ARM64    string `toml:"arm64"`
	ChainURL string `toml:"chain_url"`
}

type rawMenu struct {
	Name        string    `toml:"name"`
	Label       string    `toml:"label"`
	Description string    `toml:"description"`
	Type        string    `toml:"type"`
	HTTP        *rawHTTPSection `toml:"http"`
	Boot        *rawBoot  `toml:"boot"`
}

type rawHTTPSection struct {
	Files    string `toml:"files"`
	Path     string `toml:"path"`
	Upstream string `toml:"upstream"`
}

type rawBoot struct {
	Kernel  string   `toml:"kernel"`
	Initrd  string   `toml:"initrd"`
	Cmdline string   `toml:"cmdline"`
	Loader  string   `toml:"loader"`
	Files   []string `toml:"files"`
	Binary  string   `toml:"binary"`
	Image   string   `toml:"image"`
}

type rawClient struct {
	MAC        string                  `toml:"mac"`
	Name       string                  `toml:"name"`
	Enabled    *bool                   `toml:"enabled"`
	Menu       rawMenuConfig           `toml:"menu"`
	Bootloader *rawBootloaderOverride  `toml:"bootloader"`
	Vars       map[string]string       `toml:"vars"`
}

type rawBootloaderOverride struct {
	UEFX64 string `toml:"uefi_x64"`
	UEFX86 string `toml:"uefi_x86"`
	BIOS   string `toml:"bios"`
	ARM64  string `toml:"arm64"`
}

type rawMenuConfig struct {
	Entries []string `toml:"entries"`
	Default string   `toml:"default"`
	Timeout int      `toml:"timeout"`
}

// fileResult holds the parsed content from a single file, tagged with origin.
type fileResult struct {
	filename   string
	server     *domain.ServerConfig
	dhcpProxy  *domain.DHCPProxyConfig
	tftp       *domain.TFTPConfig
	http       *domain.HTTPConfig
	health     *domain.HealthConfig
	bootloader *domain.BootloaderConfig
	menus      []*domain.MenuEntry
	clients    []*domain.Client
}

// parseFile reads and parses a single TOML file into domain types.
func parseFile(path string) (*fileResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var raw rawFile
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	result := &fileResult{filename: path}

	if raw.Server != nil {
		cfg := convertServer(raw.Server)
		result.server = &cfg
	}

	if raw.DHCPProxy != nil {
		cfg := convertDHCPProxy(raw.DHCPProxy)
		result.dhcpProxy = &cfg
	}

	if raw.TFTP != nil {
		cfg, err := convertTFTP(raw.TFTP)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		result.tftp = &cfg
	}

	if raw.HTTP != nil {
		cfg, err := convertHTTP(raw.HTTP)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		result.http = &cfg
	}

	if raw.Health != nil {
		cfg, err := convertHealth(raw.Health)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		result.health = &cfg
	}

	if raw.Bootloader != nil {
		cfg := convertBootloader(raw.Bootloader)
		result.bootloader = &cfg
	}

	for i, rm := range raw.Menu {
		entry, err := convertMenu(&rm, path)
		if err != nil {
			return nil, fmt.Errorf("%s: menu[%d]: %w", path, i, err)
		}
		result.menus = append(result.menus, entry)
	}

	for i, rc := range raw.Client {
		client, err := convertClient(&rc, path)
		if err != nil {
			return nil, fmt.Errorf("%s: client[%d]: %w", path, i, err)
		}
		result.clients = append(result.clients, client)
	}

	return result, nil
}

func convertServer(raw *rawServer) domain.ServerConfig {
	return domain.ServerConfig{
		Interface: raw.Interface,
		DataDir:   raw.DataDir,
		Logging: domain.LogConfig{
			Level:  raw.Logging.Level,
			Format: raw.Logging.Format,
		},
	}
}

func convertDHCPProxy(raw *rawDHCPProxy) domain.DHCPProxyConfig {
	return domain.DHCPProxyConfig{
		Enabled:     boolDefault(raw.Enabled, true),
		Port:        raw.Port,
		ProxyPort:   raw.ProxyPort,
		VendorClass: raw.VendorClass,
	}
}

func convertTFTP(raw *rawTFTP) (domain.TFTPConfig, error) {
	cfg := domain.TFTPConfig{
		Enabled:   boolDefault(raw.Enabled, true),
		Port:      raw.Port,
		BlockSize: raw.BlockSize,
		Retries:   raw.Retries,
	}
	if raw.Timeout != "" {
		d, err := time.ParseDuration(raw.Timeout)
		if err != nil {
			return cfg, fmt.Errorf("tftp.timeout: %w", err)
		}
		cfg.Timeout = d
	}
	return cfg, nil
}

func convertHTTP(raw *rawHTTP) (domain.HTTPConfig, error) {
	cfg := domain.HTTPConfig{
		Enabled: boolDefault(raw.Enabled, true),
		Port:    raw.Port,
	}
	if raw.ReadTimeout != "" {
		d, err := time.ParseDuration(raw.ReadTimeout)
		if err != nil {
			return cfg, fmt.Errorf("http.read_timeout: %w", err)
		}
		cfg.ReadTimeout = d
	}
	if raw.TLS != nil {
		cfg.TLS = domain.TLSConfig{
			Enabled: raw.TLS.Enabled,
			Cert:    raw.TLS.Cert,
			Key:     raw.TLS.Key,
		}
	}
	return cfg, nil
}

func convertHealth(raw *rawHealth) (domain.HealthConfig, error) {
	cfg := domain.HealthConfig{
		Enabled:      boolDefault(raw.Enabled, true),
		StartupCheck: boolDefault(raw.StartupCheck, true),
	}
	if raw.Interval != "" {
		d, err := time.ParseDuration(raw.Interval)
		if err != nil {
			return cfg, fmt.Errorf("health.interval: %w", err)
		}
		cfg.Interval = d
	}
	return cfg, nil
}

func convertBootloader(raw *rawBootloader) domain.BootloaderConfig {
	return domain.BootloaderConfig{
		Dir:      raw.Dir,
		UEFX64:   raw.UEFX64,
		UEFX86:   raw.UEFX86,
		BIOS:     raw.BIOS,
		ARM64:    raw.ARM64,
		ChainURL: raw.ChainURL,
	}
}

func convertMenu(raw *rawMenu, sourceFile string) (*domain.MenuEntry, error) {
	entry := &domain.MenuEntry{
		Name:        raw.Name,
		Label:       raw.Label,
		Description: raw.Description,
		SourceFile:  sourceFile,
	}

	if raw.Type != "" {
		mt, err := domain.ParseMenuType(raw.Type)
		if err != nil {
			return nil, fmt.Errorf("menu %q: %w", raw.Name, err)
		}
		entry.Type = mt
	}

	if raw.HTTP != nil {
		entry.HTTP = domain.MenuHTTP{
			Files:    raw.HTTP.Files,
			Path:     raw.HTTP.Path,
			Upstream: raw.HTTP.Upstream,
		}
	}

	if raw.Boot != nil {
		entry.Boot = domain.BootParams{
			Kernel:  raw.Boot.Kernel,
			Initrd:  raw.Boot.Initrd,
			Cmdline: raw.Boot.Cmdline,
			Loader:  raw.Boot.Loader,
			Files:   raw.Boot.Files,
			Binary:  raw.Boot.Binary,
			Image:   raw.Boot.Image,
		}
	}

	return entry, nil
}

func convertClient(raw *rawClient, sourceFile string) (*domain.Client, error) {
	mac, err := domain.ParseMAC(raw.MAC)
	if err != nil {
		return nil, fmt.Errorf("client %q: %w", raw.Name, err)
	}

	client := &domain.Client{
		MAC:        mac,
		Name:       raw.Name,
		Enabled:    boolDefault(raw.Enabled, true),
		SourceFile: sourceFile,
		Vars:       raw.Vars,
		Menu: domain.MenuConfig{
			Entries: raw.Menu.Entries,
			Default: raw.Menu.Default,
			Timeout: raw.Menu.Timeout,
		},
	}

	if raw.Bootloader != nil {
		client.Bootloader = &domain.BootloaderOverride{
			UEFX64: raw.Bootloader.UEFX64,
			UEFX86: raw.Bootloader.UEFX86,
			BIOS:   raw.Bootloader.BIOS,
			ARM64:  raw.Bootloader.ARM64,
		}
	}

	return client, nil
}

// boolDefault returns the value pointed to by b, or defaultVal if b is nil.
func boolDefault(b *bool, defaultVal bool) bool {
	if b != nil {
		return *b
	}
	return defaultVal
}
