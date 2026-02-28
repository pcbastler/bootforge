package health

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bootforge/internal/domain"
)

// fakeEFI builds a minimal PE/COFF binary with an iPXE signature.
func fakeEFI(machine uint16) []byte {
	data := buildMinimalPE(machine, true)
	return data
}

// fakeBIOS builds a minimal BIOS boot image with an iPXE signature.
func fakeBIOS() []byte {
	data := make([]byte, 512)
	data[0] = 0xEB
	copy(data[100:], []byte("iPXE initialising"))
	return data
}

func TestPreflightBootloaderFilesPresent(t *testing.T) {
	dir := t.TempDir()
	blDir := filepath.Join(dir, "bootloader")
	os.MkdirAll(blDir, 0755)
	os.WriteFile(filepath.Join(blDir, "ipxe.efi"), fakeEFI(peAMD64), 0644)
	os.WriteFile(filepath.Join(blDir, "undionly.kpxe"), fakeBIOS(), 0644)

	cfg := &domain.FullConfig{
		Server: domain.ServerConfig{
			Interface: "lo",
			DataDir:   dir,
		},
		Bootloader: domain.BootloaderConfig{
			Dir:    "bootloader",
			UEFX64: "ipxe.efi",
			BIOS:   "undionly.kpxe",
		},
	}

	results, _ := RunPreflight(cfg)

	// Check bootloader file results.
	for _, r := range results {
		if r.Name == "bootloader/UEFI x64" || r.Name == "bootloader/BIOS" {
			if r.Status != domain.StatusOK {
				t.Errorf("preflight %s: status = %v, want OK", r.Name, r.Status)
			}
		}
	}
}

func TestPreflightBootloaderFilesMissing(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "bootloader"), 0755)

	cfg := &domain.FullConfig{
		Server: domain.ServerConfig{
			Interface: "lo",
			DataDir:   dir,
		},
		Bootloader: domain.BootloaderConfig{
			Dir:    "bootloader",
			UEFX64: "ipxe.efi", // not created
		},
	}

	results, allOK := RunPreflight(cfg)
	if allOK {
		t.Error("preflight should fail when bootloader files are missing")
	}

	found := false
	for _, r := range results {
		if r.Name == "bootloader/UEFI x64" {
			found = true
			if r.Status != domain.StatusFail {
				t.Errorf("bootloader check status = %v, want Fail", r.Status)
			}
		}
	}
	if !found {
		t.Error("should have a bootloader check result")
	}
}

func TestPreflightInterfaceLoopback(t *testing.T) {
	cfg := &domain.FullConfig{
		Server: domain.ServerConfig{
			Interface: "lo",
			DataDir:   t.TempDir(),
		},
		Bootloader: domain.BootloaderConfig{
			Dir: "bootloader",
		},
	}

	results, _ := RunPreflight(cfg)
	for _, r := range results {
		if r.Name == "interface" {
			if r.Status != domain.StatusOK {
				t.Errorf("loopback interface check: status = %v, want OK", r.Status)
			}
			return
		}
	}
	t.Error("should have an interface check result")
}

func TestPreflightInterfaceNotFound(t *testing.T) {
	cfg := &domain.FullConfig{
		Server: domain.ServerConfig{
			Interface: "nonexistent0",
			DataDir:   t.TempDir(),
		},
		Bootloader: domain.BootloaderConfig{
			Dir: "bootloader",
		},
	}

	results, allOK := RunPreflight(cfg)
	if allOK {
		t.Error("preflight should fail for nonexistent interface")
	}

	for _, r := range results {
		if r.Name == "interface" {
			if r.Status != domain.StatusFail {
				t.Errorf("interface check: status = %v, want Fail", r.Status)
			}
			return
		}
	}
	t.Error("should have an interface check result")
}

func TestPreflightPortConflict(t *testing.T) {
	// Bind a TCP port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	cfg := &domain.FullConfig{
		Server: domain.ServerConfig{
			Interface: "lo",
			DataDir:   t.TempDir(),
		},
		HTTP: domain.HTTPConfig{
			Enabled: true,
			Port:    port,
		},
		Bootloader: domain.BootloaderConfig{
			Dir: "bootloader",
		},
	}

	results, allOK := RunPreflight(cfg)
	if allOK {
		t.Error("preflight should fail when port is in use")
	}

	for _, r := range results {
		if r.Name == "port "+string(rune(port))+"/tcp" || r.Status == domain.StatusFail {
			// Found the port conflict.
			return
		}
	}
	// Port conflict detection should be in the results.
	hasFail := false
	for _, r := range results {
		if r.Status == domain.StatusFail {
			hasFail = true
		}
	}
	if !hasFail {
		t.Error("should have at least one failing check for port conflict")
	}
}

func TestParseHexIPv4(t *testing.T) {
	tests := []struct {
		hex  string
		want string
	}{
		{"0100007F", "127.0.0.1"},
		{"00000000", "0.0.0.0"},
		{"0103000A", "10.0.3.1"},
		{"0101A8C0", "192.168.1.1"},
	}

	for _, tt := range tests {
		got := parseHexIPv4(tt.hex)
		if got != tt.want {
			t.Errorf("parseHexIPv4(%q) = %q, want %q", tt.hex, got, tt.want)
		}
	}
}

func TestFindUDPListeners_DetectsOwnSocket(t *testing.T) {
	// Bind a UDP port and verify findUDPListeners detects it.
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	port := conn.LocalAddr().(*net.UDPAddr).Port
	listeners := findUDPListeners(port)

	if len(listeners) == 0 {
		t.Fatalf("findUDPListeners(%d) found no listeners, expected at least 1", port)
	}

	// The listener should mention 127.0.0.1.
	found := false
	for _, l := range listeners {
		if strings.Contains(l, "127.0.0.1") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected listener on 127.0.0.1, got %v", listeners)
	}
}

func TestCheckUDPPort_Conflict(t *testing.T) {
	// Bind a UDP port.
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	port := conn.LocalAddr().(*net.UDPAddr).Port
	result := checkUDPPort(port)

	if result.Status != domain.StatusFail {
		t.Errorf("status = %v, want Fail", result.Status)
	}
	if !strings.Contains(result.Message, fmt.Sprintf("port %d/udp", port)) {
		t.Errorf("message should mention port: %s", result.Message)
	}
}

func TestCheckUDPPort_Available(t *testing.T) {
	// Find a free port by binding and immediately closing.
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := conn.LocalAddr().(*net.UDPAddr).Port
	conn.Close()

	result := checkUDPPort(port)
	if result.Status != domain.StatusOK {
		t.Errorf("status = %v, want OK: %s", result.Status, result.Message)
	}
}

func TestPreflightDataDirMissing(t *testing.T) {
	cfg := &domain.FullConfig{
		Server: domain.ServerConfig{
			Interface: "lo",
			DataDir:   "/nonexistent/dir/12345",
		},
		Bootloader: domain.BootloaderConfig{
			Dir: "bootloader",
		},
	}

	results, allOK := RunPreflight(cfg)
	if allOK {
		t.Error("preflight should fail when data dir is missing")
	}

	for _, r := range results {
		if r.Name == "data_dir" {
			if r.Status != domain.StatusFail {
				t.Errorf("data_dir check: status = %v, want Fail", r.Status)
			}
			return
		}
	}
	t.Error("should have a data_dir check result")
}
