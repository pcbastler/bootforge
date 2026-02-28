package health

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateBootFile_MissingFile(t *testing.T) {
	info := ValidateBootFile("/nonexistent/file.efi", "x86_64")
	if info.Valid {
		t.Error("should not be valid")
	}
	if len(info.Problems) == 0 {
		t.Error("should have problems")
	}
}

func TestValidateBootFile_EmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.efi")
	os.WriteFile(path, []byte{}, 0644)

	info := ValidateBootFile(path, "x86_64")
	if info.Valid {
		t.Error("empty file should not be valid")
	}
}

func TestValidateBootFile_ValidEFI(t *testing.T) {
	// Build a minimal PE/COFF binary with MZ header and PE signature.
	pe := buildMinimalPE(peAMD64, true)
	path := filepath.Join(t.TempDir(), "test.efi")
	os.WriteFile(path, pe, 0644)

	info := ValidateBootFile(path, "x86_64")
	if !info.Valid {
		t.Errorf("should be valid, problems: %v", info.Problems)
	}
	if info.Format != "PE/EFI" {
		t.Errorf("format = %q, want PE/EFI", info.Format)
	}
	if info.Arch != "x86_64" {
		t.Errorf("arch = %q, want x86_64", info.Arch)
	}
	if !info.HasIPXE {
		t.Error("should detect iPXE signature")
	}
}

func TestValidateBootFile_ArchMismatch(t *testing.T) {
	// ARM64 binary but expect x86_64.
	pe := buildMinimalPE(peARM64, false)
	path := filepath.Join(t.TempDir(), "arm.efi")
	os.WriteFile(path, pe, 0644)

	info := ValidateBootFile(path, "x86_64")
	if info.Valid {
		t.Error("arch mismatch should not be valid")
	}
	if info.Arch != "arm64" {
		t.Errorf("arch = %q, want arm64", info.Arch)
	}
}

func TestValidateBootFile_I386(t *testing.T) {
	pe := buildMinimalPE(peI386, false)
	path := filepath.Join(t.TempDir(), "x86.efi")
	os.WriteFile(path, pe, 0644)

	info := ValidateBootFile(path, "i386")
	if !info.Valid {
		t.Errorf("should be valid, problems: %v", info.Problems)
	}
	if info.Arch != "i386" {
		t.Errorf("arch = %q, want i386", info.Arch)
	}
}

func TestValidateBootFile_BIOSFile(t *testing.T) {
	// BIOS kpxe files are not PE — just raw binary data.
	data := make([]byte, 1024)
	data[0] = 0xEB // JMP short (common BIOS boot sector start)
	copy(data[100:], []byte("iPXE initialising"))

	path := filepath.Join(t.TempDir(), "undionly.kpxe")
	os.WriteFile(path, data, 0644)

	info := ValidateBootFile(path, "bios")
	if !info.Valid {
		t.Errorf("BIOS file should be valid, problems: %v", info.Problems)
	}
	if info.Format != "PXE/BIOS" {
		t.Errorf("format = %q, want PXE/BIOS", info.Format)
	}
	if !info.HasIPXE {
		t.Error("should detect iPXE signature")
	}
}

func TestValidateBootFile_NonEFIWithEFIExpected(t *testing.T) {
	// Raw binary (no MZ header) but expected to be EFI.
	data := make([]byte, 256)
	path := filepath.Join(t.TempDir(), "bad.efi")
	os.WriteFile(path, data, 0644)

	info := ValidateBootFile(path, "x86_64")
	if info.Valid {
		t.Error("non-PE file expected as EFI should not be valid")
	}
}

func TestValidateBootFile_NoIPXESignature(t *testing.T) {
	pe := buildMinimalPE(peAMD64, false) // no iPXE string
	path := filepath.Join(t.TempDir(), "other.efi")
	os.WriteFile(path, pe, 0644)

	info := ValidateBootFile(path, "x86_64")
	if !info.Valid {
		t.Errorf("should be valid PE, problems: %v", info.Problems)
	}
	if info.HasIPXE {
		t.Error("should not detect iPXE")
	}
}

func TestArchMatches(t *testing.T) {
	tests := []struct {
		detected, expected string
		want               bool
	}{
		{"x86_64", "x86_64", true},
		{"x86_64", "uefi_x64", true},
		{"x86_64", "UEFI x64", true},
		{"i386", "i386", true},
		{"i386", "uefi_x86", true},
		{"arm64", "arm64", true},
		{"arm64", "ARM64", true},
		{"x86_64", "arm64", false},
		{"i386", "x86_64", false},
		{"arm64", "i386", false},
	}

	for _, tt := range tests {
		if got := archMatches(tt.detected, tt.expected); got != tt.want {
			t.Errorf("archMatches(%q, %q) = %v, want %v", tt.detected, tt.expected, got, tt.want)
		}
	}
}

// buildMinimalPE creates a minimal PE/COFF binary for testing.
func buildMinimalPE(machine uint16, includeIPXE bool) []byte {
	// Minimal PE: MZ header + PE signature + COFF header.
	data := make([]byte, 512)

	// MZ magic.
	data[0] = 'M'
	data[1] = 'Z'

	// PE offset at 0x3C → point to offset 0x80.
	binary.LittleEndian.PutUint32(data[0x3C:], 0x80)

	// PE signature at 0x80.
	data[0x80] = 'P'
	data[0x81] = 'E'
	data[0x82] = 0
	data[0x83] = 0

	// Machine type at 0x84.
	binary.LittleEndian.PutUint16(data[0x84:], machine)

	// Optionally embed iPXE signature.
	if includeIPXE {
		copy(data[0x100:], []byte("iPXE -- Open Source Network Boot Firmware"))
	}

	return data
}
