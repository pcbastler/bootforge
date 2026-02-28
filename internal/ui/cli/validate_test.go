package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateValidConfig(t *testing.T) {
	// Create a minimal valid config in a temp dir.
	dir := t.TempDir()
	cfg := `
[server]
interface = "eth0"
data_dir = "/tmp/bootforge"

[bootloader]
dir = "bootloader"
chain_url = "http://192.168.1.10:8080/boot/${mac}/menu.ipxe"

[dhcp_proxy]
enabled = true
port = 67
proxy_port = 4011

[tftp]
enabled = true
port = 69

[http]
enabled = true
port = 8080

[[menu]]
name = "local-disk"
label = "Boot from local disk"
type = "exit"

[[client]]
mac = "*"
name = "default"

[client.menu]
entries = ["local-disk"]
`
	os.WriteFile(filepath.Join(dir, "bootforge.toml"), []byte(cfg), 0644)

	cfgDir = dir
	err := runValidate(nil, nil)
	if err != nil {
		t.Errorf("runValidate() error = %v, want nil for valid config", err)
	}
}

func TestValidateInvalidConfig(t *testing.T) {
	// Create config with syntax error.
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "broken.toml"), []byte("this is not valid toml [[["), 0644)

	cfgDir = dir
	err := runValidate(nil, nil)
	if err == nil {
		t.Error("runValidate() should fail for invalid config")
	}
}

func TestValidateEmptyDir(t *testing.T) {
	dir := t.TempDir()

	cfgDir = dir
	err := runValidate(nil, nil)
	if err == nil {
		t.Error("runValidate() should fail for empty config dir")
	}
}
