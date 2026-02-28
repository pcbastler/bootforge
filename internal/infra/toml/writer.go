package toml

import (
	"fmt"
	"os"
	"path/filepath"

	"bootforge/internal/domain"

	toml "github.com/pelletier/go-toml/v2"
)

// WriteConfig marshals a FullConfig to TOML and writes it to dir/bootforge.toml.
// Absolute data_dir paths are made relative to dir if they're inside it.
func WriteConfig(cfg *domain.FullConfig, dir string) error {
	// Work on a copy so we don't mutate the caller's config.
	writeCfg := *cfg
	writeCfg.Server.DataDir = makeRelative(cfg.Server.DataDir, dir)

	raw := configToRaw(&writeCfg)

	data, err := toml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	path := filepath.Join(dir, "bootforge.toml")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}

// makeRelative converts an absolute path back to a relative one if it's
// inside baseDir. If it's already relative or outside baseDir, it's returned as-is.
func makeRelative(path, baseDir string) string {
	if path == "" || !filepath.IsAbs(path) {
		return path
	}
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(absBase, path)
	if err != nil {
		return path
	}
	return rel
}

// configToRaw converts domain types back to the raw TOML structs.
func configToRaw(cfg *domain.FullConfig) rawFile {
	raw := rawFile{
		Server: &rawServer{
			Interface: cfg.Server.Interface,
			DataDir:   cfg.Server.DataDir,
			Logging: rawLogging{
				Level:  cfg.Server.Logging.Level,
				Format: cfg.Server.Logging.Format,
			},
		},
		Bootloader: &rawBootloader{
			Dir:      cfg.Bootloader.Dir,
			UEFX64:   cfg.Bootloader.UEFX64,
			UEFX86:   cfg.Bootloader.UEFX86,
			BIOS:     cfg.Bootloader.BIOS,
			ARM64:    cfg.Bootloader.ARM64,
			ChainURL: cfg.Bootloader.ChainURL,
		},
	}

	// DHCP proxy — only include if non-zero config.
	if cfg.DHCPProxy.Port > 0 || cfg.DHCPProxy.Enabled {
		enabled := cfg.DHCPProxy.Enabled
		raw.DHCPProxy = &rawDHCPProxy{
			Enabled:     &enabled,
			Port:        cfg.DHCPProxy.Port,
			ProxyPort:   cfg.DHCPProxy.ProxyPort,
			VendorClass: cfg.DHCPProxy.VendorClass,
		}
	}

	// TFTP.
	if cfg.TFTP.Port > 0 || cfg.TFTP.Enabled {
		enabled := cfg.TFTP.Enabled
		raw.TFTP = &rawTFTP{
			Enabled:   &enabled,
			Port:      cfg.TFTP.Port,
			BlockSize: cfg.TFTP.BlockSize,
			Retries:   cfg.TFTP.Retries,
		}
		if cfg.TFTP.Timeout > 0 {
			raw.TFTP.Timeout = cfg.TFTP.Timeout.String()
		}
	}

	// HTTP.
	if cfg.HTTP.Port > 0 || cfg.HTTP.Enabled {
		enabled := cfg.HTTP.Enabled
		raw.HTTP = &rawHTTP{
			Enabled: &enabled,
			Port:    cfg.HTTP.Port,
		}
		if cfg.HTTP.ReadTimeout > 0 {
			raw.HTTP.ReadTimeout = cfg.HTTP.ReadTimeout.String()
		}
		if cfg.HTTP.TLS.Enabled {
			raw.HTTP.TLS = &rawTLS{
				Enabled: cfg.HTTP.TLS.Enabled,
				Cert:    cfg.HTTP.TLS.Cert,
				Key:     cfg.HTTP.TLS.Key,
			}
		}
	}

	// Health.
	if cfg.Health.Enabled || cfg.Health.Interval > 0 {
		enabled := cfg.Health.Enabled
		startupCheck := cfg.Health.StartupCheck
		raw.Health = &rawHealth{
			Enabled:      &enabled,
			StartupCheck: &startupCheck,
		}
		if cfg.Health.Interval > 0 {
			raw.Health.Interval = cfg.Health.Interval.String()
		}
	}

	// Menus.
	for _, m := range cfg.Menus {
		rm := rawMenu{
			Name:        m.Name,
			Label:       m.Label,
			Description: m.Description,
			Type:        m.Type.String(),
		}

		if m.HTTP.Path != "" || m.HTTP.Files != "" || m.HTTP.Upstream != "" {
			rm.HTTP = &rawHTTPSection{
				Files:    m.HTTP.Files,
				Path:     m.HTTP.Path,
				Upstream: m.HTTP.Upstream,
			}
		}

		if m.Type != domain.MenuExit {
			rm.Boot = &rawBoot{
				Kernel:  m.Boot.Kernel,
				Initrd:  m.Boot.Initrd,
				Cmdline: m.Boot.Cmdline,
				Loader:  m.Boot.Loader,
				Files:   m.Boot.Files,
				Binary:  m.Boot.Binary,
				Image:   m.Boot.Image,
			}
		}

		raw.Menu = append(raw.Menu, rm)
	}

	// Clients.
	for _, c := range cfg.Clients {
		mac := c.MAC.String()
		if c.IsWildcard() {
			mac = "*"
		}
		enabled := c.Enabled
		rc := rawClient{
			MAC:     mac,
			Name:    c.Name,
			Enabled: &enabled,
			Vars:    c.Vars,
			Menu: rawMenuConfig{
				Entries: c.Menu.Entries,
				Default: c.Menu.Default,
				Timeout: c.Menu.Timeout,
			},
		}
		raw.Client = append(raw.Client, rc)
	}

	return raw
}
