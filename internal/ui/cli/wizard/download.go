package wizard

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// IPXEArch describes a downloadable iPXE bootloader architecture.
type IPXEArch struct {
	Label    string // e.g. "UEFI x64"
	Filename string // e.g. "ipxe.efi"
	URL      string // full download URL (built from base)
}

// DefaultIPXEBaseURL is the default source for iPXE binaries.
const DefaultIPXEBaseURL = "https://boot.ipxe.org"

// IPXEVariant selects which iPXE binary to download for EFI architectures.
type IPXEVariant string

const (
	// IPXEVariantFull downloads ipxe.efi — full build with framebuffer/GOP support.
	// Recommended for maximum hardware compatibility.
	IPXEVariantFull IPXEVariant = "full"

	// IPXEVariantSNPOnly downloads snponly.efi — minimal SNP-only build.
	// Smaller but may lack display output on some UEFI systems.
	IPXEVariantSNPOnly IPXEVariant = "snponly"
)

// IPXEArchitectures returns the available iPXE architectures with download
// URLs derived from baseURL and variant. Uses architecture-specific subdirectories
// matching the boot.ipxe.org layout.
func IPXEArchitectures(baseURL string, variant IPXEVariant) []IPXEArch {
	efiBinary := "ipxe.efi"
	if variant == IPXEVariantSNPOnly {
		efiBinary = "snponly.efi"
	}

	return []IPXEArch{
		{Label: "UEFI x64", Filename: "ipxe.efi", URL: baseURL + "/x86_64-efi/" + efiBinary},
		{Label: "UEFI x86", Filename: "ipxe-i386.efi", URL: baseURL + "/i386-efi/" + efiBinary},
		{Label: "BIOS", Filename: "undionly.kpxe", URL: baseURL + "/x86_64-pcbios/undionly.kpxe"},
		{Label: "ARM64", Filename: "ipxe-arm64.efi", URL: baseURL + "/arm64-efi/" + efiBinary},
	}
}

// DownloadResult reports the outcome for a single file.
type DownloadResult struct {
	Arch IPXEArch
	Size int64
	Err  error
}

// DownloadIPXEFiles downloads the selected architectures into destDir.
// The progress callback is called for each file as it completes.
func DownloadIPXEFiles(archs []IPXEArch, destDir string, progress func(DownloadResult)) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("creating bootloader dir: %w", err)
	}

	var failed int
	for _, arch := range archs {
		res := downloadOne(arch, destDir)
		if res.Err != nil {
			failed++
		}
		if progress != nil {
			progress(res)
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d of %d downloads failed", failed, len(archs))
	}
	return nil
}

func downloadOne(arch IPXEArch, destDir string) DownloadResult {
	res := DownloadResult{Arch: arch}

	resp, err := http.Get(arch.URL) //nolint:gosec // URL is user-configured
	if err != nil {
		res.Err = fmt.Errorf("downloading %s: %w", arch.Filename, err)
		return res
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		res.Err = fmt.Errorf("downloading %s: HTTP %d", arch.Filename, resp.StatusCode)
		return res
	}

	path := filepath.Join(destDir, arch.Filename)
	f, err := os.Create(path)
	if err != nil {
		res.Err = fmt.Errorf("creating %s: %w", path, err)
		return res
	}
	defer f.Close()

	n, err := io.Copy(f, resp.Body)
	if err != nil {
		res.Err = fmt.Errorf("writing %s: %w", path, err)
		return res
	}

	if n == 0 {
		res.Err = fmt.Errorf("%s: downloaded file is empty", arch.Filename)
		return res
	}

	res.Size = n
	return res
}
