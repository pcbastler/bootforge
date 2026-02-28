package toml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDirMinimalConfig(t *testing.T) {
	cfg, err := LoadDir("../../../testdata/config/valid/minimal")
	if err != nil {
		t.Fatalf("LoadDir() error = %v", err)
	}

	if cfg.Server.Interface != "eth0" {
		t.Errorf("Server.Interface = %q, want %q", cfg.Server.Interface, "eth0")
	}
	if cfg.Server.DataDir != "/etc/bootforge/data" {
		t.Errorf("Server.DataDir = %q, want %q", cfg.Server.DataDir, "/etc/bootforge/data")
	}
	if !cfg.DHCPProxy.Enabled {
		t.Error("DHCPProxy should be enabled")
	}
	if cfg.DHCPProxy.Port != 67 {
		t.Errorf("DHCPProxy.Port = %d, want 67", cfg.DHCPProxy.Port)
	}
	if !cfg.TFTP.Enabled {
		t.Error("TFTP should be enabled")
	}
	if !cfg.HTTP.Enabled {
		t.Error("HTTP should be enabled")
	}
	if len(cfg.Menus) != 2 {
		t.Errorf("Menus count = %d, want 2", len(cfg.Menus))
	}
	if len(cfg.Clients) != 2 {
		t.Errorf("Clients count = %d, want 2", len(cfg.Clients))
	}
}

func TestLoadDirSplitConfig(t *testing.T) {
	cfg, err := LoadDir("../../../testdata/config/valid/split")
	if err != nil {
		t.Fatalf("LoadDir() error = %v", err)
	}

	if cfg.Server.Interface != "eth0" {
		t.Errorf("Server.Interface = %q, want %q", cfg.Server.Interface, "eth0")
	}
	if len(cfg.Menus) != 3 {
		t.Errorf("Menus count = %d, want 3", len(cfg.Menus))
	}
	if len(cfg.Clients) != 3 {
		t.Errorf("Clients count = %d, want 3", len(cfg.Clients))
	}

	// Verify menu names.
	menuNames := make(map[string]bool)
	for _, m := range cfg.Menus {
		menuNames[m.Name] = true
	}
	for _, name := range []string{"rescue", "ubuntu-install", "local-disk"} {
		if !menuNames[name] {
			t.Errorf("menu %q not found", name)
		}
	}

	// Verify wildcard client exists.
	hasWildcard := false
	for _, c := range cfg.Clients {
		if c.IsWildcard() {
			hasWildcard = true
			break
		}
	}
	if !hasWildcard {
		t.Error("wildcard client not found")
	}
}

func TestLoadDirDuplicateMAC(t *testing.T) {
	_, err := LoadDir("../../../testdata/config/invalid/duplicate-mac")
	if err == nil {
		t.Fatal("LoadDir() should fail with duplicate MAC")
	}
	if !strings.Contains(err.Error(), "duplicate client MAC") {
		t.Errorf("error should mention duplicate MAC, got: %v", err)
	}
	// Should mention both files.
	if !strings.Contains(err.Error(), "a.toml") || !strings.Contains(err.Error(), "b.toml") {
		t.Errorf("error should mention both files, got: %v", err)
	}
}

func TestLoadDirDuplicateMenuName(t *testing.T) {
	_, err := LoadDir("../../../testdata/config/invalid/duplicate-menu")
	if err == nil {
		t.Fatal("LoadDir() should fail with duplicate menu name")
	}
	if !strings.Contains(err.Error(), "duplicate menu name") {
		t.Errorf("error should mention duplicate menu, got: %v", err)
	}
}

func TestLoadDirBadReference(t *testing.T) {
	_, err := LoadDir("../../../testdata/config/invalid/bad-reference")
	if err == nil {
		t.Fatal("LoadDir() should fail with bad reference")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention the missing entry name, got: %v", err)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should say 'not found', got: %v", err)
	}
}

func TestLoadDirBadReferenceSuggestion(t *testing.T) {
	// Create a config where client references "rescu" (typo for "rescue").
	dir := t.TempDir()
	write(t, dir, "config.toml", `
[server]
interface = "eth0"
data_dir = "/data"

[bootloader]
dir = "bootloader/"
chain_url = "http://x/menu.ipxe"

[[menu]]
name = "rescue"
label = "Rescue"
type = "live"

[menu.boot]
kernel = "vmlinuz"

[[client]]
mac = "*"
name = "default"

[client.menu]
entries = ["rescu"]
`)

	_, err := LoadDir(dir)
	if err == nil {
		t.Fatal("LoadDir() should fail with bad reference")
	}
	if !strings.Contains(err.Error(), "did you mean") {
		t.Errorf("error should suggest similar name, got: %v", err)
	}
}

func TestLoadDirNoServer(t *testing.T) {
	_, err := LoadDir("../../../testdata/config/invalid/no-server")
	if err == nil {
		t.Fatal("LoadDir() should fail without [server] section")
	}
	if !strings.Contains(err.Error(), "[server]") {
		t.Errorf("error should mention [server], got: %v", err)
	}
}

func TestLoadDirSyntaxError(t *testing.T) {
	_, err := LoadDir("../../../testdata/config/invalid/syntax-error")
	if err == nil {
		t.Fatal("LoadDir() should fail with syntax error")
	}
}

func TestLoadDirNonExistentDir(t *testing.T) {
	_, err := LoadDir("/nonexistent/directory")
	if err == nil {
		t.Fatal("LoadDir() should fail for nonexistent directory")
	}
}

func TestLoadDirEmptyDir(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadDir(dir)
	if err == nil {
		t.Fatal("LoadDir() should fail for empty directory")
	}
	if !strings.Contains(err.Error(), "no .toml files") {
		t.Errorf("error should mention no .toml files, got: %v", err)
	}
}

func TestLoadDirNoTomlFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not toml"), 0644)
	os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0644)

	_, err := LoadDir(dir)
	if err == nil {
		t.Fatal("LoadDir() should fail when no .toml files exist")
	}
}

func TestLoadDirIgnoresSubdirectories(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "config.toml", `
[server]
interface = "eth0"
data_dir = "/data"

[bootloader]
dir = "bootloader/"
chain_url = "http://x/menu.ipxe"

[[menu]]
name = "rescue"
label = "Rescue"
type = "live"

[menu.boot]
kernel = "vmlinuz"

[[client]]
mac = "*"
name = "default"

[client.menu]
entries = ["rescue"]
`)

	// Create subdirectory with a toml file (should be ignored).
	subdir := filepath.Join(dir, "subdir")
	os.MkdirAll(subdir, 0755)
	os.WriteFile(filepath.Join(subdir, "nested.toml"), []byte("[server]\ninterface = \"lo\""), 0644)

	cfg, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir() error = %v", err)
	}

	// Should only see the top-level config, not nested.
	if cfg.Server.Interface != "eth0" {
		t.Errorf("Server.Interface = %q, want %q (nested file should be ignored)", cfg.Server.Interface, "eth0")
	}
}

func TestLoadDirMultipleWildcards(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.toml", `
[server]
interface = "eth0"
data_dir = "/data"

[bootloader]
dir = "bootloader/"
chain_url = "http://x/menu.ipxe"

[[menu]]
name = "rescue"
label = "Rescue"
type = "live"

[menu.boot]
kernel = "vmlinuz"

[[client]]
mac = "*"
name = "default-1"

[client.menu]
entries = ["rescue"]
`)
	write(t, dir, "b.toml", `
[[client]]
mac = "*"
name = "default-2"

[client.menu]
entries = ["rescue"]
`)

	_, err := LoadDir(dir)
	if err == nil {
		t.Fatal("LoadDir() should fail with multiple wildcard clients")
	}
	if !strings.Contains(err.Error(), "multiple wildcard") {
		t.Errorf("error should mention multiple wildcard, got: %v", err)
	}
}

func TestLoadDirDuplicateServerSection(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.toml", `
[server]
interface = "eth0"
data_dir = "/data"
`)
	write(t, dir, "b.toml", `
[server]
interface = "lo"
data_dir = "/other"
`)

	_, err := LoadDir(dir)
	if err == nil {
		t.Fatal("LoadDir() should fail with duplicate [server]")
	}
	if !strings.Contains(err.Error(), "[server] defined in both") {
		t.Errorf("error should mention both files, got: %v", err)
	}
}

func TestLoadDirClientVarsPreserved(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "config.toml", `
[server]
interface = "eth0"
data_dir = "/data"

[bootloader]
dir = "bootloader/"
chain_url = "http://x/menu.ipxe"

[[menu]]
name = "rescue"
label = "Rescue"
type = "live"

[menu.boot]
kernel = "vmlinuz"

[[client]]
mac = "aa:bb:cc:dd:ee:01"
name = "ws01"

[client.menu]
entries = ["rescue"]

[client.vars]
hostname = "workstation-01"
ip = "192.168.1.100"
`)

	cfg, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir() error = %v", err)
	}

	c := cfg.Clients[0]
	if c.Vars["hostname"] != "workstation-01" {
		t.Errorf("Vars[hostname] = %q, want %q", c.Vars["hostname"], "workstation-01")
	}
	if c.Vars["ip"] != "192.168.1.100" {
		t.Errorf("Vars[ip] = %q, want %q", c.Vars["ip"], "192.168.1.100")
	}
}

func TestLoadDirBootloaderConfig(t *testing.T) {
	cfg, err := LoadDir("../../../testdata/config/valid/minimal")
	if err != nil {
		t.Fatalf("LoadDir() error = %v", err)
	}

	bl := cfg.Bootloader
	if bl.Dir != "bootloader/" {
		t.Errorf("Bootloader.Dir = %q, want %q", bl.Dir, "bootloader/")
	}
	if bl.UEFX64 != "ipxe.efi" {
		t.Errorf("Bootloader.UEFX64 = %q, want %q", bl.UEFX64, "ipxe.efi")
	}
	if bl.BIOS != "undionly.kpxe" {
		t.Errorf("Bootloader.BIOS = %q, want %q", bl.BIOS, "undionly.kpxe")
	}
	if !strings.Contains(bl.ChainURL, "${server_ip}") {
		t.Errorf("Bootloader.ChainURL should contain ${server_ip}, got %q", bl.ChainURL)
	}
}

func TestLoadDirDisabledClient(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "config.toml", `
[server]
interface = "eth0"
data_dir = "/data"

[bootloader]
dir = "bootloader/"
chain_url = "http://x/menu.ipxe"

[[menu]]
name = "rescue"
label = "Rescue"
type = "live"

[menu.boot]
kernel = "vmlinuz"

[[client]]
mac = "aa:bb:cc:dd:ee:01"
name = "disabled-client"
enabled = false

[client.menu]
entries = ["rescue"]
`)

	cfg, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir() error = %v", err)
	}

	if cfg.Clients[0].Enabled {
		t.Error("Client should be disabled")
	}
}
