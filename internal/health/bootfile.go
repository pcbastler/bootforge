package health

import (
	"encoding/binary"
	"fmt"
	"os"
)

// PE/COFF machine types.
const (
	peI386  = 0x014c
	peAMD64 = 0x8664
	peARM64 = 0xAA64
)

// BootFileInfo holds validation results for a boot file.
type BootFileInfo struct {
	Path     string
	Size     int64
	Format   string // "PE/EFI", "PXE/BIOS", "unknown"
	Arch     string // "x86_64", "i386", "arm64", ""
	HasIPXE  bool   // contains iPXE signature
	Valid    bool
	Problems []string
}

// ValidateBootFile inspects a boot file and reports its format, architecture,
// and whether it looks like a valid bootloader.
func ValidateBootFile(path string, expectedArch string) BootFileInfo {
	info := BootFileInfo{Path: path}

	fi, err := os.Stat(path)
	if err != nil {
		info.Problems = append(info.Problems, fmt.Sprintf("cannot read: %v", err))
		return info
	}
	info.Size = fi.Size()

	if info.Size == 0 {
		info.Problems = append(info.Problems, "file is empty")
		return info
	}

	// Read the first 4KB for header inspection.
	readSize := int64(4096)
	if info.Size < readSize {
		readSize = info.Size
	}
	header := make([]byte, readSize)
	f, err := os.Open(path)
	if err != nil {
		info.Problems = append(info.Problems, fmt.Sprintf("cannot open: %v", err))
		return info
	}
	defer f.Close()

	n, err := f.Read(header)
	if err != nil {
		info.Problems = append(info.Problems, fmt.Sprintf("cannot read: %v", err))
		return info
	}
	header = header[:n]

	// Check for PE/COFF (EFI) format: starts with "MZ".
	if n >= 2 && header[0] == 'M' && header[1] == 'Z' {
		info.Format = "PE/EFI"
		validatePE(&info, header, expectedArch)
	} else {
		// Not a PE binary — only valid for BIOS boot images (kpxe, pxe).
		info.Format = "PXE/BIOS"
		if expectedArch == "bios" {
			info.Valid = true
		} else {
			info.Problems = append(info.Problems,
				fmt.Sprintf("expected EFI binary for %s, but file has no PE/MZ header", expectedArch))
		}
	}

	// Scan for iPXE signature in the full file (or as much as we can read).
	info.HasIPXE = scanForIPXE(f, header)

	return info
}

// validatePE parses the PE header to extract architecture and validate structure.
func validatePE(info *BootFileInfo, header []byte, expectedArch string) {
	// PE offset is at 0x3C (4 bytes, little-endian).
	if len(header) < 0x40 {
		info.Problems = append(info.Problems, "PE header too short")
		return
	}

	peOffset := int(binary.LittleEndian.Uint32(header[0x3C:0x40]))
	if peOffset+6 > len(header) {
		info.Problems = append(info.Problems, "PE offset beyond file data")
		return
	}

	// Verify PE signature: "PE\0\0".
	if header[peOffset] != 'P' || header[peOffset+1] != 'E' ||
		header[peOffset+2] != 0 || header[peOffset+3] != 0 {
		info.Problems = append(info.Problems, "invalid PE signature")
		return
	}

	// Machine type is 2 bytes after PE signature.
	machine := binary.LittleEndian.Uint16(header[peOffset+4 : peOffset+6])
	switch machine {
	case peAMD64:
		info.Arch = "x86_64"
	case peI386:
		info.Arch = "i386"
	case peARM64:
		info.Arch = "arm64"
	default:
		info.Arch = fmt.Sprintf("unknown(0x%04x)", machine)
		info.Problems = append(info.Problems,
			fmt.Sprintf("unknown PE machine type: 0x%04X", machine))
		return
	}

	info.Valid = true

	// Check architecture match.
	if expectedArch != "" && !archMatches(info.Arch, expectedArch) {
		info.Valid = false
		info.Problems = append(info.Problems,
			fmt.Sprintf("architecture mismatch: file is %s, expected %s", info.Arch, expectedArch))
	}
}

// ipxeSignatures are byte patterns that identify iPXE binaries.
// The BIOS undionly.kpxe variant embeds "UNDI" rather than "iPXE".
var ipxeSignatures = [][]byte{
	[]byte("iPXE"),
	[]byte("UNDI code segment"),
	[]byte("gPXE"),
}

// scanForIPXE looks for iPXE-related signatures in the file.
func scanForIPXE(f *os.File, header []byte) bool {
	if matchesAnySignature(header) {
		return true
	}
	// Also check further into the file — version strings are often
	// embedded past the first 4KB.
	buf := make([]byte, 8192)
	for offset := int64(4096); ; offset += int64(len(buf)) {
		n, err := f.ReadAt(buf, offset)
		if n > 0 {
			if matchesAnySignature(buf[:n]) {
				return true
			}
		}
		if err != nil {
			break
		}
	}
	return false
}

func matchesAnySignature(data []byte) bool {
	for _, sig := range ipxeSignatures {
		if containsBytes(data, sig) {
			return true
		}
	}
	return false
}

func containsBytes(data, pattern []byte) bool {
	for i := 0; i <= len(data)-len(pattern); i++ {
		match := true
		for j := range pattern {
			if data[i+j] != pattern[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// archMatches checks if detected arch matches the expected arch label.
func archMatches(detected, expected string) bool {
	switch expected {
	case "x86_64", "uefi_x64", "UEFI x64":
		return detected == "x86_64"
	case "i386", "uefi_x86", "UEFI x86":
		return detected == "i386"
	case "arm64", "ARM64":
		return detected == "arm64"
	case "bios", "BIOS":
		return true // BIOS files are not PE, handled separately
	default:
		return detected == expected
	}
}
