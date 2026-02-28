package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bootforge/internal/domain"
)

var testVars = IPXEVars{
	ServerIP: "192.168.1.10",
	HTTPPort: 8080,
	MAC:      "aa:bb:cc:dd:ee:01",
}

func TestIPXESingleEntry(t *testing.T) {
	gen := NewIPXEGenerator()
	entries := []*domain.MenuEntry{
		{
			Name: "rescue", Label: "Rescue System", Type: domain.MenuLive,
			Boot: domain.BootParams{Kernel: "vmlinuz", Initrd: "initrd", Cmdline: "boot=live"},
			HTTP: domain.MenuHTTP{Path: "/tools/rescue/"},
		},
	}
	mc := domain.MenuConfig{Entries: []string{"rescue"}}

	got := gen.Generate(entries, mc, testVars)
	golden(t, "single-entry.ipxe", got)
}

func TestIPXEMultiMenu(t *testing.T) {
	gen := NewIPXEGenerator()
	entries := []*domain.MenuEntry{
		{
			Name: "rescue", Label: "Rescue System", Type: domain.MenuLive,
			Boot: domain.BootParams{Kernel: "vmlinuz", Initrd: "initrd", Cmdline: "boot=live"},
			HTTP: domain.MenuHTTP{Path: "/tools/rescue/"},
		},
		{
			Name: "ubuntu-install", Label: "Ubuntu 24.04", Type: domain.MenuInstall,
			Boot: domain.BootParams{Kernel: "vmlinuz", Initrd: "initrd", Cmdline: "autoinstall ds=nocloud"},
			HTTP: domain.MenuHTTP{Path: "/installers/ubuntu/"},
		},
		{
			Name: "local-disk", Label: "Boot from local disk", Type: domain.MenuExit,
		},
	}
	mc := domain.MenuConfig{
		Entries: []string{"rescue", "ubuntu-install", "local-disk"},
		Default: "rescue",
		Timeout: 30,
	}

	got := gen.Generate(entries, mc, testVars)
	golden(t, "multi-menu.ipxe", got)
}

func TestIPXEExitOnly(t *testing.T) {
	gen := NewIPXEGenerator()
	entries := []*domain.MenuEntry{
		{Name: "local-disk", Label: "Boot from local disk", Type: domain.MenuExit},
	}
	mc := domain.MenuConfig{Entries: []string{"local-disk"}}

	got := gen.Generate(entries, mc, testVars)
	golden(t, "exit-entry.ipxe", got)
}

func TestIPXEOverride(t *testing.T) {
	gen := NewIPXEGenerator()
	entry := &domain.MenuEntry{
		Name: "ubuntu-install", Label: "Ubuntu 24.04", Type: domain.MenuInstall,
		Boot: domain.BootParams{Kernel: "vmlinuz", Initrd: "initrd", Cmdline: "autoinstall ds=nocloud"},
		HTTP: domain.MenuHTTP{Path: "/installers/ubuntu/"},
	}

	got := gen.GenerateOverride(entry, testVars)
	golden(t, "override.ipxe", got)
}

func TestIPXEWimboot(t *testing.T) {
	gen := NewIPXEGenerator()
	entries := []*domain.MenuEntry{
		{
			Name: "win11", Label: "Windows 11", Type: domain.MenuInstall,
			Boot: domain.BootParams{
				Kernel: "wimboot",
				Loader: "wimboot",
				Files:  []string{"BCD", "boot.sdi", "boot.wim"},
			},
			HTTP: domain.MenuHTTP{Path: "/installers/win11/"},
		},
	}
	mc := domain.MenuConfig{Entries: []string{"win11"}}

	got := gen.Generate(entries, mc, testVars)
	golden(t, "wimboot.ipxe", got)
}

func TestIPXEVariableSubstitution(t *testing.T) {
	gen := NewIPXEGenerator()
	entries := []*domain.MenuEntry{
		{
			Name: "test", Label: "Test", Type: domain.MenuLive,
			Boot: domain.BootParams{
				Kernel:  "vmlinuz",
				Cmdline: "ip=${server_ip} hostname=${hostname}",
			},
			HTTP: domain.MenuHTTP{Path: "/test/"},
		},
	}
	mc := domain.MenuConfig{Entries: []string{"test"}}
	vars := IPXEVars{
		ServerIP: "10.0.0.1",
		HTTPPort: 9090,
		MAC:      "aa:bb:cc:dd:ee:ff",
		Custom:   map[string]string{"hostname": "ws01"},
	}

	got := gen.Generate(entries, mc, vars)

	if !strings.Contains(got, "ip=10.0.0.1") {
		t.Errorf("script should contain substituted server_ip, got:\n%s", got)
	}
	if !strings.Contains(got, "hostname=ws01") {
		t.Errorf("script should contain substituted hostname, got:\n%s", got)
	}
	if !strings.Contains(got, "http://10.0.0.1:9090") {
		t.Errorf("script should contain substituted URL, got:\n%s", got)
	}
}

func TestIPXEBinaryBoot(t *testing.T) {
	gen := NewIPXEGenerator()
	entries := []*domain.MenuEntry{
		{
			Name: "memtest", Label: "Memtest86+", Type: domain.MenuTool,
			Boot: domain.BootParams{Binary: "memtest.bin"},
			HTTP: domain.MenuHTTP{Path: "/tools/memtest/"},
		},
	}
	mc := domain.MenuConfig{Entries: []string{"memtest"}}

	got := gen.Generate(entries, mc, testVars)

	if !strings.Contains(got, "chain http://192.168.1.10:8080/tools/memtest/memtest.bin") {
		t.Errorf("script should contain chain command for binary, got:\n%s", got)
	}
}

func TestIPXEFallbackPath(t *testing.T) {
	gen := NewIPXEGenerator()
	entries := []*domain.MenuEntry{
		{
			Name: "custom", Label: "Custom", Type: domain.MenuLive,
			Boot: domain.BootParams{Kernel: "vmlinuz"},
			// No HTTP.Path set — should fall back to entry name.
		},
	}
	mc := domain.MenuConfig{Entries: []string{"custom"}}

	got := gen.Generate(entries, mc, testVars)

	if !strings.Contains(got, "http://192.168.1.10:8080/custom/vmlinuz") {
		t.Errorf("script should use entry name as path fallback, got:\n%s", got)
	}
}

// golden compares the generated output with a golden file.
// Set UPDATE_GOLDEN=1 to update golden files.
func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "ipxe", name)

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, []byte(got), 0644); err != nil {
			t.Fatalf("updating golden file %s: %v", path, err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden file %s: %v", path, err)
	}

	if got != string(want) {
		t.Errorf("output does not match golden file %s.\n\nGot:\n%s\n\nWant:\n%s", name, got, string(want))
	}
}
