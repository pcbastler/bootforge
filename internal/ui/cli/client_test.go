package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testClientConfig = `
[server]
interface = "eth0"
data_dir = "/tmp"

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
name = "rescue"
label = "Rescue System"
type = "live"

[menu.boot]
kernel = "vmlinuz"

[[menu]]
name = "local-disk"
label = "Boot from local disk"
type = "exit"

[[client]]
mac = "aa:bb:cc:dd:ee:01"
name = "server-1"
enabled = true

[client.menu]
entries = ["rescue", "local-disk"]
default = "rescue"
timeout = 30

[[client]]
mac = "*"
name = "default"

[client.menu]
entries = ["local-disk"]
`

func setupTestConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bootforge.toml"), []byte(testClientConfig), 0644)
	return dir
}

func captureOutput(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String(), err
}

func TestClientList(t *testing.T) {
	cfgDir = setupTestConfig(t)

	out, err := captureOutput(t, func() error {
		return runClientList(nil, nil)
	})
	if err != nil {
		t.Fatalf("runClientList() error = %v", err)
	}

	if !strings.Contains(out, "MAC") {
		t.Error("output should contain header")
	}
	if !strings.Contains(out, "aa:bb:cc:dd:ee:01") {
		t.Error("output should contain client MAC")
	}
	if !strings.Contains(out, "server-1") {
		t.Error("output should contain client name")
	}
	if !strings.Contains(out, "*") {
		t.Error("output should contain wildcard client")
	}
}

func TestClientShowKnown(t *testing.T) {
	cfgDir = setupTestConfig(t)

	out, err := captureOutput(t, func() error {
		return runClientShow(nil, []string{"aa:bb:cc:dd:ee:01"})
	})
	if err != nil {
		t.Fatalf("runClientShow() error = %v", err)
	}

	if !strings.Contains(out, "server-1") {
		t.Error("output should contain client name")
	}
	if !strings.Contains(out, "rescue") {
		t.Error("output should contain menu entries")
	}
}

func TestClientShowWildcard(t *testing.T) {
	cfgDir = setupTestConfig(t)

	out, err := captureOutput(t, func() error {
		return runClientShow(nil, []string{"*"})
	})
	if err != nil {
		t.Fatalf("runClientShow() error = %v", err)
	}

	if !strings.Contains(out, "default") {
		t.Error("output should contain wildcard client name")
	}
}

func TestClientShowUnknown(t *testing.T) {
	cfgDir = setupTestConfig(t)

	_, err := captureOutput(t, func() error {
		return runClientShow(nil, []string{"ff:ff:ff:ff:ff:ff"})
	})
	if err == nil {
		t.Error("runClientShow() should fail for unknown MAC")
	}
}
