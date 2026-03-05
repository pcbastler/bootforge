package tftp

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"bootforge/internal/domain"

	"github.com/pin/tftp/v3"
)

func startTestServer(t *testing.T) (addr string, dataDir string, cleanup func()) {
	t.Helper()

	// Create temp directory with bootloader files.
	dataDir = t.TempDir()
	blDir := filepath.Join(dataDir, "bootloader")
	os.MkdirAll(blDir, 0755)

	files := map[string]string{
		"ipxe.efi":        "UEFI-X64-BOOTLOADER",
		"ipxe-x86.efi":    "UEFI-X86-BOOTLOADER",
		"ipxe-arm64.efi":  "ARM64-BOOTLOADER",
		"undionly.kpxe":    "BIOS-BOOTLOADER",
	}
	for name, content := range files {
		os.WriteFile(filepath.Join(blDir, name), []byte(content), 0644)
	}

	cfg := domain.TFTPConfig{
		Enabled:   true,
		Port:      0, // ephemeral port
		Timeout:   5 * time.Second,
		BlockSize: 512,
	}

	srv := NewTFTPServer(cfg, dataDir, "bootloader", slog.Default())

	// Use a manual listen to get the ephemeral port.
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening: %v", err)
	}

	actualAddr := conn.LocalAddr().String()

	srv.server = tftp.NewServer(srv.readHandler, nil)
	srv.server.SetTimeout(cfg.Timeout)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		srv.server.Serve(conn)
	}()

	return actualAddr, dataDir, func() {
		cancel()
		srv.server.Shutdown()
		_ = ctx
	}
}

func TestTFTPReadKnownFile(t *testing.T) {
	addr, _, cleanup := startTestServer(t)
	defer cleanup()

	data, err := tftpRead(addr, "ipxe.efi")
	if err != nil {
		t.Fatalf("TFTP read error: %v", err)
	}
	if string(data) != "UEFI-X64-BOOTLOADER" {
		t.Errorf("got %q, want %q", string(data), "UEFI-X64-BOOTLOADER")
	}
}

func TestTFTPReadAllBootloaders(t *testing.T) {
	addr, _, cleanup := startTestServer(t)
	defer cleanup()

	tests := []struct {
		filename string
		want     string
	}{
		{"ipxe.efi", "UEFI-X64-BOOTLOADER"},
		{"ipxe-x86.efi", "UEFI-X86-BOOTLOADER"},
		{"ipxe-arm64.efi", "ARM64-BOOTLOADER"},
		{"undionly.kpxe", "BIOS-BOOTLOADER"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			data, err := tftpRead(addr, tt.filename)
			if err != nil {
				t.Fatalf("TFTP read error: %v", err)
			}
			if string(data) != tt.want {
				t.Errorf("got %q, want %q", string(data), tt.want)
			}
		})
	}
}

func TestTFTPReadUnknownFile(t *testing.T) {
	addr, _, cleanup := startTestServer(t)
	defer cleanup()

	_, err := tftpRead(addr, "nonexistent.file")
	if err == nil {
		t.Error("TFTP read should fail for unknown file")
	}
}

func TestTFTPReadPathTraversal(t *testing.T) {
	addr, dataDir, cleanup := startTestServer(t)
	defer cleanup()

	// Create a secret file outside the bootloader dir.
	os.WriteFile(filepath.Join(dataDir, "secret.txt"), []byte("SECRET"), 0644)

	traversals := []string{
		"../secret.txt",
		"../../etc/passwd",
		"../../../etc/shadow",
		"bootloader/../secret.txt",
	}

	for _, path := range traversals {
		t.Run(path, func(t *testing.T) {
			_, err := tftpRead(addr, path)
			if err == nil {
				t.Errorf("TFTP read should fail for path traversal: %s", path)
			}
		})
	}
}

func TestTFTPConcurrentReads(t *testing.T) {
	addr, _, cleanup := startTestServer(t)
	defer cleanup()

	var wg sync.WaitGroup
	files := []string{"ipxe.efi", "undionly.kpxe", "ipxe-arm64.efi"}

	for i := 0; i < 5; i++ {
		for _, f := range files {
			wg.Add(1)
			go func(filename string) {
				defer wg.Done()
				data, err := tftpRead(addr, filename)
				if err != nil {
					t.Errorf("concurrent read of %s failed: %v", filename, err)
					return
				}
				if len(data) == 0 {
					t.Errorf("concurrent read of %s returned empty data", filename)
				}
			}(f)
		}
	}
	wg.Wait()
}

func TestTFTPAutoexecDynamic(t *testing.T) {
	addr, _, cleanup := startTestServerWithAutoexec(t)
	defer cleanup()

	data, err := tftpRead(addr, "autoexec.ipxe")
	if err != nil {
		t.Fatalf("TFTP read autoexec.ipxe: %v", err)
	}

	script := string(data)
	if !strings.HasPrefix(script, "#!ipxe\n") {
		t.Errorf("autoexec.ipxe should start with #!ipxe shebang, got: %s", script)
	}
	if !strings.Contains(script, "chain --autofree") {
		t.Errorf("autoexec.ipxe should contain chain command, got: %s", script)
	}
	if !strings.Contains(script, "192.168.1.10:8080") {
		t.Errorf("autoexec.ipxe should contain resolved server IP and port, got: %s", script)
	}
	if !strings.Contains(script, "${net0/mac}") {
		t.Errorf("autoexec.ipxe should use iPXE ${net0/mac} variable, got: %s", script)
	}
}

func TestTFTPAutoexecNotConfigured(t *testing.T) {
	addr, _, cleanup := startTestServer(t)
	defer cleanup()

	// Without autoexec config, autoexec.ipxe should not be found.
	_, err := tftpRead(addr, "autoexec.ipxe")
	if err == nil {
		t.Error("autoexec.ipxe should fail when not configured")
	}
}

func startTestServerWithAutoexec(t *testing.T) (addr string, dataDir string, cleanup func()) {
	t.Helper()

	dataDir = t.TempDir()
	blDir := filepath.Join(dataDir, "bootloader")
	os.MkdirAll(blDir, 0755)

	files := map[string]string{
		"ipxe.efi":     "UEFI-X64-BOOTLOADER",
		"undionly.kpxe": "BIOS-BOOTLOADER",
	}
	for name, content := range files {
		os.WriteFile(filepath.Join(blDir, name), []byte(content), 0644)
	}

	cfg := domain.TFTPConfig{
		Enabled:   true,
		Port:      0,
		Timeout:   5 * time.Second,
		BlockSize: 512,
	}

	srv := NewTFTPServer(cfg, dataDir, "bootloader", slog.Default())
	srv.SetAutoexec(AutoexecConfig{
		ServerIP: "192.168.1.10",
		HTTPPort: 8080,
		ChainURL: "http://${server_ip}:${http_port}/boot/${mac}/menu.ipxe",
	})

	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening: %v", err)
	}

	actualAddr := conn.LocalAddr().String()
	srv.server = tftp.NewServer(srv.readHandler, nil)
	srv.server.SetTimeout(cfg.Timeout)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		srv.server.Serve(conn)
	}()

	return actualAddr, dataDir, func() {
		cancel()
		srv.server.Shutdown()
		_ = ctx
	}
}

// tftpRead performs a TFTP read request and returns the file contents.
func tftpRead(addr, filename string) ([]byte, error) {
	client, err := tftp.NewClient(addr)
	if err != nil {
		return nil, fmt.Errorf("creating TFTP client: %w", err)
	}
	client.SetTimeout(3 * time.Second)

	wt, err := client.Receive(filename, "octet")
	if err != nil {
		return nil, fmt.Errorf("TFTP receive %s: %w", filename, err)
	}

	var buf bytes.Buffer
	_, err = wt.WriteTo(&buf)
	if err != nil {
		return nil, fmt.Errorf("reading TFTP data: %w", err)
	}

	return buf.Bytes(), nil
}
