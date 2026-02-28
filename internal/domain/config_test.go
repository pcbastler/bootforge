package domain

import (
	"testing"
)

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name    string
		port    int
		wantErr bool
	}{
		{"port 1", 1, false},
		{"port 80", 80, false},
		{"port 8080", 8080, false},
		{"port 65535", 65535, false},
		{"port 0", 0, true},
		{"port -1", -1, true},
		{"port 65536", 65536, true},
		{"port 100000", 100000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePort(tt.port, "test.port")
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePort(%d) error = %v, wantErr %v", tt.port, err, tt.wantErr)
			}
		})
	}
}

func TestServerConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ServerConfig
		wantErr bool
	}{
		{
			name:    "valid",
			cfg:     ServerConfig{Interface: "eth0", DataDir: "/etc/bootforge/data"},
			wantErr: false,
		},
		{
			name:    "empty interface",
			cfg:     ServerConfig{DataDir: "/data"},
			wantErr: true,
		},
		{
			name:    "empty data dir",
			cfg:     ServerConfig{Interface: "eth0"},
			wantErr: true,
		},
		{
			name:    "both empty",
			cfg:     ServerConfig{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ServerConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDHCPProxyConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     DHCPProxyConfig
		wantErr bool
	}{
		{"disabled", DHCPProxyConfig{Enabled: false}, false},
		{"valid", DHCPProxyConfig{Enabled: true, Port: 67, ProxyPort: 4011}, false},
		{"invalid port", DHCPProxyConfig{Enabled: true, Port: 0, ProxyPort: 4011}, true},
		{"invalid proxy port", DHCPProxyConfig{Enabled: true, Port: 67, ProxyPort: 0}, true},
		{"port too high", DHCPProxyConfig{Enabled: true, Port: 70000, ProxyPort: 4011}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("DHCPProxyConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTFTPConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     TFTPConfig
		wantErr bool
	}{
		{"disabled", TFTPConfig{Enabled: false}, false},
		{"valid", TFTPConfig{Enabled: true, Port: 69}, false},
		{"invalid port", TFTPConfig{Enabled: true, Port: 0}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("TFTPConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHTTPConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     HTTPConfig
		wantErr bool
	}{
		{"disabled", HTTPConfig{Enabled: false}, false},
		{"valid", HTTPConfig{Enabled: true, Port: 8080}, false},
		{"invalid port", HTTPConfig{Enabled: true, Port: -1}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("HTTPConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBootloaderConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     BootloaderConfig
		wantErr bool
	}{
		{
			name: "valid",
			cfg: BootloaderConfig{
				Dir:      "data/bootloader/",
				ChainURL: "http://${server_ip}:${http_port}/boot/${mac}/menu.ipxe",
			},
			wantErr: false,
		},
		{
			name:    "empty dir",
			cfg:     BootloaderConfig{ChainURL: "http://x/menu.ipxe"},
			wantErr: true,
		},
		{
			name:    "empty chain url",
			cfg:     BootloaderConfig{Dir: "data/bootloader/"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("BootloaderConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBootloaderConfigFileForArch(t *testing.T) {
	cfg := BootloaderConfig{
		UEFX64: "ipxe.efi",
		UEFX86: "ipxe-x86.efi",
		BIOS:   "undionly.kpxe",
		ARM64:  "ipxe-arm64.efi",
	}

	tests := []struct {
		arch ClientArch
		want string
	}{
		{ArchUEFIx64, "ipxe.efi"},
		{ArchUEFIx86, "ipxe-x86.efi"},
		{ArchBIOS, "undionly.kpxe"},
		{ArchARM64, "ipxe-arm64.efi"},
		{ClientArch(99), ""},
	}

	for _, tt := range tests {
		t.Run(tt.arch.String(), func(t *testing.T) {
			if got := cfg.FileForArch(tt.arch); got != tt.want {
				t.Errorf("FileForArch(%s) = %q, want %q", tt.arch, got, tt.want)
			}
		})
	}
}

func TestFullConfigValidate(t *testing.T) {
	validConfig := func() *FullConfig {
		return &FullConfig{
			Server:    ServerConfig{Interface: "eth0", DataDir: "/data"},
			DHCPProxy: DHCPProxyConfig{Enabled: true, Port: 67, ProxyPort: 4011},
			TFTP:      TFTPConfig{Enabled: true, Port: 69},
			HTTP:      HTTPConfig{Enabled: true, Port: 8080},
			Bootloader: BootloaderConfig{
				Dir:      "data/bootloader/",
				ChainURL: "http://x/boot/${mac}/menu.ipxe",
			},
			Menus: []*MenuEntry{
				{Name: "rescue", Label: "Rescue", Type: MenuLive, Boot: BootParams{Kernel: "vmlinuz"}},
			},
			Clients: []*Client{
				{
					MAC:  WildcardMAC,
					Name: "default",
					Menu: MenuConfig{Entries: []string{"rescue"}},
				},
			},
		}
	}

	t.Run("valid full config", func(t *testing.T) {
		cfg := validConfig()
		if err := cfg.Validate(); err != nil {
			t.Errorf("FullConfig.Validate() unexpected error: %v", err)
		}
	})

	t.Run("invalid server config", func(t *testing.T) {
		cfg := validConfig()
		cfg.Server.Interface = ""
		if err := cfg.Validate(); err == nil {
			t.Error("FullConfig.Validate() should fail with empty interface")
		}
	})

	t.Run("invalid menu entry", func(t *testing.T) {
		cfg := validConfig()
		cfg.Menus = append(cfg.Menus, &MenuEntry{Name: "", Label: "Bad"})
		if err := cfg.Validate(); err == nil {
			t.Error("FullConfig.Validate() should fail with invalid menu entry")
		}
	})

	t.Run("invalid client", func(t *testing.T) {
		cfg := validConfig()
		cfg.Clients = append(cfg.Clients, &Client{MAC: nil, Name: "bad"})
		if err := cfg.Validate(); err == nil {
			t.Error("FullConfig.Validate() should fail with nil MAC client")
		}
	})
}
