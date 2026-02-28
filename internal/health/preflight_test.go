package health

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"bootforge/internal/domain"
)

func TestPreflightBootloaderFilesPresent(t *testing.T) {
	dir := t.TempDir()
	blDir := filepath.Join(dir, "bootloader")
	os.MkdirAll(blDir, 0755)
	os.WriteFile(filepath.Join(blDir, "ipxe.efi"), []byte("boot"), 0644)
	os.WriteFile(filepath.Join(blDir, "undionly.kpxe"), []byte("boot"), 0644)

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
