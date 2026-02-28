package health

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"bootforge/internal/domain"
)

// PreflightResult is the outcome of a single pre-flight check.
type PreflightResult struct {
	Name    string
	Status  domain.CheckStatus
	Message string
}

// RunPreflight runs all pre-flight checks against the configuration.
// Returns the results and whether all checks passed.
func RunPreflight(cfg *domain.FullConfig) ([]PreflightResult, bool) {
	var results []PreflightResult
	allOK := true

	// 1. Check network interface exists and is UP.
	results = append(results, checkInterface(cfg.Server.Interface))

	// 2. Check port availability.
	if cfg.DHCPProxy.Enabled {
		results = append(results, checkUDPPort(cfg.DHCPProxy.Port))
		results = append(results, checkUDPPort(cfg.DHCPProxy.ProxyPort))
	}
	if cfg.TFTP.Enabled {
		results = append(results, checkUDPPort(cfg.TFTP.Port))
	}
	if cfg.HTTP.Enabled {
		results = append(results, checkTCPPort(cfg.HTTP.Port))
	}

	// 3. Check bootloader files exist.
	blDir := filepath.Join(cfg.Server.DataDir, cfg.Bootloader.Dir)
	files := map[string]string{
		"UEFI x64": cfg.Bootloader.UEFX64,
		"UEFI x86": cfg.Bootloader.UEFX86,
		"BIOS":     cfg.Bootloader.BIOS,
		"ARM64":    cfg.Bootloader.ARM64,
	}
	for arch, filename := range files {
		if filename == "" {
			continue
		}
		results = append(results, checkFile(arch, filepath.Join(blDir, filename)))
	}

	// 4. Check data directory is readable.
	results = append(results, checkDataDir(cfg.Server.DataDir))

	for _, r := range results {
		if r.Status == domain.StatusFail {
			allOK = false
		}
	}

	return results, allOK
}

func checkInterface(name string) PreflightResult {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return PreflightResult{
			Name:    "interface",
			Status:  domain.StatusFail,
			Message: fmt.Sprintf("interface %s: %v", name, err),
		}
	}

	if iface.Flags&net.FlagUp == 0 {
		return PreflightResult{
			Name:    "interface",
			Status:  domain.StatusFail,
			Message: fmt.Sprintf("interface %s exists but is DOWN", name),
		}
	}

	addrs, _ := iface.Addrs()
	addrStr := ""
	for _, a := range addrs {
		if ipNet, ok := a.(*net.IPNet); ok && ipNet.IP.To4() != nil {
			addrStr = ipNet.IP.String()
			break
		}
	}

	return PreflightResult{
		Name:    "interface",
		Status:  domain.StatusOK,
		Message: fmt.Sprintf("interface %s (UP, %s)", name, addrStr),
	}
}

func checkUDPPort(port int) PreflightResult {
	addr := fmt.Sprintf(":%d", port)
	conn, err := net.ListenPacket("udp4", addr)
	if err != nil {
		return PreflightResult{
			Name:    fmt.Sprintf("port %d/udp", port),
			Status:  domain.StatusFail,
			Message: fmt.Sprintf("port %d/udp already in use", port),
		}
	}
	conn.Close()
	// Brief delay to ensure socket is fully released.
	time.Sleep(10 * time.Millisecond)
	return PreflightResult{
		Name:    fmt.Sprintf("port %d/udp", port),
		Status:  domain.StatusOK,
		Message: fmt.Sprintf("port %d/udp available", port),
	}
}

func checkTCPPort(port int) PreflightResult {
	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return PreflightResult{
			Name:    fmt.Sprintf("port %d/tcp", port),
			Status:  domain.StatusFail,
			Message: fmt.Sprintf("port %d/tcp already in use", port),
		}
	}
	ln.Close()
	return PreflightResult{
		Name:    fmt.Sprintf("port %d/tcp", port),
		Status:  domain.StatusOK,
		Message: fmt.Sprintf("port %d/tcp available", port),
	}
}

func checkFile(label, path string) PreflightResult {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return PreflightResult{
			Name:    fmt.Sprintf("bootloader/%s", label),
			Status:  domain.StatusFail,
			Message: fmt.Sprintf("%s: file not found at %s", label, path),
		}
	}
	if err != nil {
		return PreflightResult{
			Name:    fmt.Sprintf("bootloader/%s", label),
			Status:  domain.StatusFail,
			Message: fmt.Sprintf("%s: %v", label, err),
		}
	}
	return PreflightResult{
		Name:    fmt.Sprintf("bootloader/%s", label),
		Status:  domain.StatusOK,
		Message: fmt.Sprintf("%s: %s (%d bytes)", label, path, info.Size()),
	}
}

func checkDataDir(path string) PreflightResult {
	info, err := os.Stat(path)
	if err != nil {
		return PreflightResult{
			Name:    "data_dir",
			Status:  domain.StatusFail,
			Message: fmt.Sprintf("data directory %s: %v", path, err),
		}
	}
	if !info.IsDir() {
		return PreflightResult{
			Name:    "data_dir",
			Status:  domain.StatusFail,
			Message: fmt.Sprintf("data directory %s: not a directory", path),
		}
	}
	return PreflightResult{
		Name:    "data_dir",
		Status:  domain.StatusOK,
		Message: fmt.Sprintf("data directory %s readable", path),
	}
}
