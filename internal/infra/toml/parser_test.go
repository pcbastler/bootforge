package toml

import (
	"os"
	"path/filepath"
	"testing"

	"bootforge/internal/domain"
)

func TestParseFileServerOnly(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "test.toml", `
[server]
interface = "eth0"
data_dir = "/data"

[server.logging]
level = "debug"
format = "json"
`)

	r, err := parseFile(filepath.Join(dir, "test.toml"))
	if err != nil {
		t.Fatalf("parseFile() error = %v", err)
	}

	if r.server == nil {
		t.Fatal("server should not be nil")
	}
	if r.server.Interface != "eth0" {
		t.Errorf("Interface = %q, want %q", r.server.Interface, "eth0")
	}
	if r.server.DataDir != "/data" {
		t.Errorf("DataDir = %q, want %q", r.server.DataDir, "/data")
	}
	if r.server.Logging.Level != "debug" {
		t.Errorf("Logging.Level = %q, want %q", r.server.Logging.Level, "debug")
	}
	if r.server.Logging.Format != "json" {
		t.Errorf("Logging.Format = %q, want %q", r.server.Logging.Format, "json")
	}
	if len(r.menus) != 0 {
		t.Errorf("menus should be empty, got %d", len(r.menus))
	}
	if len(r.clients) != 0 {
		t.Errorf("clients should be empty, got %d", len(r.clients))
	}
}

func TestParseFileMenusOnly(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "test.toml", `
[[menu]]
name = "rescue"
label = "Rescue System"
type = "live"

[menu.boot]
kernel = "vmlinuz"
initrd = "initrd"
cmdline = "boot=live"

[menu.http]
files = "tools/rescue/"
path = "/tools/rescue/"

[[menu]]
name = "local-disk"
label = "Boot from local disk"
type = "exit"
`)

	r, err := parseFile(filepath.Join(dir, "test.toml"))
	if err != nil {
		t.Fatalf("parseFile() error = %v", err)
	}

	if r.server != nil {
		t.Error("server should be nil")
	}
	if len(r.menus) != 2 {
		t.Fatalf("menus count = %d, want 2", len(r.menus))
	}

	m := r.menus[0]
	if m.Name != "rescue" {
		t.Errorf("menu[0].Name = %q, want %q", m.Name, "rescue")
	}
	if m.Type != domain.MenuLive {
		t.Errorf("menu[0].Type = %v, want MenuLive", m.Type)
	}
	if m.Boot.Kernel != "vmlinuz" {
		t.Errorf("menu[0].Boot.Kernel = %q, want %q", m.Boot.Kernel, "vmlinuz")
	}
	if m.HTTP.Files != "tools/rescue/" {
		t.Errorf("menu[0].HTTP.Files = %q, want %q", m.HTTP.Files, "tools/rescue/")
	}

	m2 := r.menus[1]
	if m2.Type != domain.MenuExit {
		t.Errorf("menu[1].Type = %v, want MenuExit", m2.Type)
	}
}

func TestParseFileClientsOnly(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "test.toml", `
[[client]]
mac = "aa:bb:cc:dd:ee:01"
name = "workstation-01"
enabled = true

[client.menu]
entries = ["rescue", "local-disk"]
default = "rescue"
timeout = 30

[client.vars]
hostname = "ws01"

[[client]]
mac = "*"
name = "default"

[client.menu]
entries = ["rescue"]
`)

	r, err := parseFile(filepath.Join(dir, "test.toml"))
	if err != nil {
		t.Fatalf("parseFile() error = %v", err)
	}

	if len(r.clients) != 2 {
		t.Fatalf("clients count = %d, want 2", len(r.clients))
	}

	c := r.clients[0]
	if c.MAC.String() != "aa:bb:cc:dd:ee:01" {
		t.Errorf("client[0].MAC = %s, want aa:bb:cc:dd:ee:01", c.MAC)
	}
	if c.Name != "workstation-01" {
		t.Errorf("client[0].Name = %q, want %q", c.Name, "workstation-01")
	}
	if !c.Enabled {
		t.Error("client[0].Enabled should be true")
	}
	if len(c.Menu.Entries) != 2 {
		t.Errorf("client[0].Menu.Entries count = %d, want 2", len(c.Menu.Entries))
	}
	if c.Menu.Default != "rescue" {
		t.Errorf("client[0].Menu.Default = %q, want %q", c.Menu.Default, "rescue")
	}
	if c.Vars["hostname"] != "ws01" {
		t.Errorf("client[0].Vars[hostname] = %q, want %q", c.Vars["hostname"], "ws01")
	}

	c2 := r.clients[1]
	if !c2.IsWildcard() {
		t.Error("client[1] should be wildcard")
	}
	if !c2.Enabled {
		t.Error("client[1].Enabled should default to true")
	}
}

func TestParseFileMixedContent(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "test.toml", `
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

	r, err := parseFile(filepath.Join(dir, "test.toml"))
	if err != nil {
		t.Fatalf("parseFile() error = %v", err)
	}

	if r.server == nil {
		t.Error("server should not be nil")
	}
	if r.bootloader == nil {
		t.Error("bootloader should not be nil")
	}
	if len(r.menus) != 1 {
		t.Errorf("menus count = %d, want 1", len(r.menus))
	}
	if len(r.clients) != 1 {
		t.Errorf("clients count = %d, want 1", len(r.clients))
	}
}

func TestParseFileEmptyFile(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "test.toml", "")

	r, err := parseFile(filepath.Join(dir, "test.toml"))
	if err != nil {
		t.Fatalf("parseFile() error = %v", err)
	}

	if r.server != nil {
		t.Error("server should be nil for empty file")
	}
	if len(r.menus) != 0 {
		t.Error("menus should be empty for empty file")
	}
}

func TestParseFileSyntaxError(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "test.toml", `[server
this is broken`)

	_, err := parseFile(filepath.Join(dir, "test.toml"))
	if err == nil {
		t.Error("parseFile() should fail on syntax error")
	}
}

func TestParseFileInvalidMenuType(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "test.toml", `
[[menu]]
name = "bad"
label = "Bad"
type = "unknown-type"
`)

	_, err := parseFile(filepath.Join(dir, "test.toml"))
	if err == nil {
		t.Error("parseFile() should fail on invalid menu type")
	}
}

func TestParseFileInvalidMAC(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "test.toml", `
[[client]]
mac = "not-a-mac"
name = "bad"

[client.menu]
entries = ["rescue"]
`)

	_, err := parseFile(filepath.Join(dir, "test.toml"))
	if err == nil {
		t.Error("parseFile() should fail on invalid MAC")
	}
}

func TestParseFileNonExistent(t *testing.T) {
	_, err := parseFile("/nonexistent/file.toml")
	if err == nil {
		t.Error("parseFile() should fail on nonexistent file")
	}
}

func TestParseFileDHCPProxyConfig(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "test.toml", `
[dhcp_proxy]
enabled = false
port = 67
proxy_port = 4011
vendor_class = "PXEClient"
`)

	r, err := parseFile(filepath.Join(dir, "test.toml"))
	if err != nil {
		t.Fatalf("parseFile() error = %v", err)
	}

	if r.dhcpProxy == nil {
		t.Fatal("dhcpProxy should not be nil")
	}
	if r.dhcpProxy.Enabled {
		t.Error("dhcpProxy.Enabled should be false")
	}
	if r.dhcpProxy.Port != 67 {
		t.Errorf("dhcpProxy.Port = %d, want 67", r.dhcpProxy.Port)
	}
	if r.dhcpProxy.VendorClass != "PXEClient" {
		t.Errorf("dhcpProxy.VendorClass = %q, want %q", r.dhcpProxy.VendorClass, "PXEClient")
	}
}

func TestParseFileTFTPWithDuration(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "test.toml", `
[tftp]
port = 69
timeout = "10s"
retries = 5
`)

	r, err := parseFile(filepath.Join(dir, "test.toml"))
	if err != nil {
		t.Fatalf("parseFile() error = %v", err)
	}

	if r.tftp == nil {
		t.Fatal("tftp should not be nil")
	}
	if r.tftp.Timeout.Seconds() != 10 {
		t.Errorf("tftp.Timeout = %v, want 10s", r.tftp.Timeout)
	}
}

func TestParseFileInvalidDuration(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "test.toml", `
[tftp]
port = 69
timeout = "not-a-duration"
`)

	_, err := parseFile(filepath.Join(dir, "test.toml"))
	if err == nil {
		t.Error("parseFile() should fail on invalid duration")
	}
}

func TestParseFileTLSConfig(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "test.toml", `
[http]
port = 443

[http.tls]
enabled = true
cert = "/etc/ssl/cert.pem"
key = "/etc/ssl/key.pem"
`)

	r, err := parseFile(filepath.Join(dir, "test.toml"))
	if err != nil {
		t.Fatalf("parseFile() error = %v", err)
	}

	if !r.http.TLS.Enabled {
		t.Error("TLS.Enabled should be true")
	}
	if r.http.TLS.Cert != "/etc/ssl/cert.pem" {
		t.Errorf("TLS.Cert = %q, want %q", r.http.TLS.Cert, "/etc/ssl/cert.pem")
	}
}

func TestParseFileEnabledDefaultsToTrue(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "test.toml", `
[dhcp_proxy]
port = 67
proxy_port = 4011

[tftp]
port = 69

[http]
port = 8080
`)

	r, err := parseFile(filepath.Join(dir, "test.toml"))
	if err != nil {
		t.Fatalf("parseFile() error = %v", err)
	}

	if !r.dhcpProxy.Enabled {
		t.Error("dhcpProxy.Enabled should default to true")
	}
	if !r.tftp.Enabled {
		t.Error("tftp.Enabled should default to true")
	}
	if !r.http.Enabled {
		t.Error("http.Enabled should default to true")
	}
}

func TestParseFileSourceFileTracking(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "menus.toml")
	write(t, dir, "menus.toml", `
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

	r, err := parseFile(path)
	if err != nil {
		t.Fatalf("parseFile() error = %v", err)
	}

	if r.menus[0].SourceFile != path {
		t.Errorf("menu.SourceFile = %q, want %q", r.menus[0].SourceFile, path)
	}
	if r.clients[0].SourceFile != path {
		t.Errorf("client.SourceFile = %q, want %q", r.clients[0].SourceFile, path)
	}
}

func TestParseFileBootParamsComplete(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "test.toml", `
[[menu]]
name = "windows"
label = "Windows 11"
type = "install"

[menu.boot]
kernel = "wimboot"
loader = "wimboot"
files = ["BCD", "boot.sdi", "boot.wim"]
cmdline = "quiet"
`)

	r, err := parseFile(filepath.Join(dir, "test.toml"))
	if err != nil {
		t.Fatalf("parseFile() error = %v", err)
	}

	m := r.menus[0]
	if m.Boot.Loader != "wimboot" {
		t.Errorf("Boot.Loader = %q, want %q", m.Boot.Loader, "wimboot")
	}
	if len(m.Boot.Files) != 3 {
		t.Errorf("Boot.Files count = %d, want 3", len(m.Boot.Files))
	}
}

func TestParseFileDashSeparatedMAC(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "test.toml", `
[[client]]
mac = "aa-bb-cc-dd-ee-ff"
name = "dash-client"

[client.menu]
entries = ["rescue"]
`)

	r, err := parseFile(filepath.Join(dir, "test.toml"))
	if err != nil {
		t.Fatalf("parseFile() error = %v", err)
	}

	if r.clients[0].MAC.String() != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("MAC = %s, want aa:bb:cc:dd:ee:ff", r.clients[0].MAC)
	}
}

// write is a test helper that creates a file in the given directory.
func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("writing test file %s: %v", name, err)
	}
}
