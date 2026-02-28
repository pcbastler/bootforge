package health

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

	// 3. Check bootloader files exist and are valid boot images.
	blDir := filepath.Join(cfg.Server.DataDir, cfg.Bootloader.Dir)
	type blEntry struct {
		label, filename, arch string
	}
	blFiles := []blEntry{
		{"UEFI x64", cfg.Bootloader.UEFX64, "x86_64"},
		{"UEFI x86", cfg.Bootloader.UEFX86, "i386"},
		{"BIOS", cfg.Bootloader.BIOS, "bios"},
		{"ARM64", cfg.Bootloader.ARM64, "arm64"},
	}
	for _, bl := range blFiles {
		if bl.filename == "" {
			continue
		}
		results = append(results, checkBootFile(bl.label, filepath.Join(blDir, bl.filename), bl.arch))
	}

	// 4. Check data directory is readable.
	results = append(results, checkDataDir(cfg.Server.DataDir))

	// 5. Check for competing PXE servers on the network.
	if cfg.DHCPProxy.Enabled {
		results = append(results, checkPXEConflict(cfg.Server.Interface))
	}

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
	name := fmt.Sprintf("port %d/udp", port)

	// Scan /proc/net/udp to detect ALL listeners on this port,
	// including processes bound to specific IPs that our wildcard
	// bind (0.0.0.0) wouldn't conflict with but could intercept packets.
	if listeners := findUDPListeners(port); len(listeners) > 0 {
		return PreflightResult{
			Name:    name,
			Status:  domain.StatusFail,
			Message: fmt.Sprintf("port %d/udp in use by: %s", port, strings.Join(listeners, ", ")),
		}
	}

	// Try to actually bind the port.
	conn, err := net.ListenPacket("udp4", fmt.Sprintf(":%d", port))
	if err != nil {
		msg := fmt.Sprintf("port %d/udp: %v", port, err)
		if os.IsPermission(err) || strings.Contains(err.Error(), "permission denied") {
			msg = fmt.Sprintf("port %d/udp: permission denied (root or CAP_NET_BIND_SERVICE required)", port)
		}
		return PreflightResult{
			Name:    name,
			Status:  domain.StatusFail,
			Message: msg,
		}
	}
	conn.Close()
	time.Sleep(10 * time.Millisecond)

	return PreflightResult{
		Name:    name,
		Status:  domain.StatusOK,
		Message: fmt.Sprintf("port %d/udp available", port),
	}
}

func checkTCPPort(port int) PreflightResult {
	name := fmt.Sprintf("port %d/tcp", port)

	if listeners := findTCPListeners(port); len(listeners) > 0 {
		return PreflightResult{
			Name:    name,
			Status:  domain.StatusFail,
			Message: fmt.Sprintf("port %d/tcp in use by: %s", port, strings.Join(listeners, ", ")),
		}
	}

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		msg := fmt.Sprintf("port %d/tcp: %v", port, err)
		if os.IsPermission(err) || strings.Contains(err.Error(), "permission denied") {
			msg = fmt.Sprintf("port %d/tcp: permission denied (root or CAP_NET_BIND_SERVICE required)", port)
		}
		return PreflightResult{
			Name:    name,
			Status:  domain.StatusFail,
			Message: msg,
		}
	}
	ln.Close()

	return PreflightResult{
		Name:    name,
		Status:  domain.StatusOK,
		Message: fmt.Sprintf("port %d/tcp available", port),
	}
}

// findUDPListeners scans /proc/net/udp for processes listening on the given port.
// Returns human-readable descriptions like "dnsmasq (10.0.3.1)" or "unknown (0.0.0.0)".
func findUDPListeners(port int) []string {
	return findProcNetListeners("/proc/net/udp", port)
}

// findTCPListeners scans /proc/net/tcp for processes listening on the given port.
func findTCPListeners(port int) []string {
	return findProcNetListeners("/proc/net/tcp", port)
}

// findProcNetListeners parses a /proc/net/{udp,tcp} file to find listeners on a port.
func findProcNetListeners(procFile string, port int) []string {
	data, err := os.ReadFile(procFile)
	if err != nil {
		return nil // not Linux or no permissions
	}

	hexPort := fmt.Sprintf("%04X", port)
	var listeners []string

	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}

		// fields[1] = local_address as HEX_IP:HEX_PORT
		localAddr := fields[1]
		parts := strings.Split(localAddr, ":")
		if len(parts) != 2 || parts[1] != hexPort {
			continue
		}

		ip := parseHexIPv4(parts[0])
		inode := fields[9]

		procName := findProcessBySocketInode(inode)
		if procName != "" {
			listeners = append(listeners, fmt.Sprintf("%s (%s)", procName, ip))
		} else {
			listeners = append(listeners, ip)
		}
	}

	return listeners
}

// parseHexIPv4 converts a /proc/net hex IP (host byte order) to dotted notation.
// Example: "0100007F" → "127.0.0.1"
func parseHexIPv4(hexIP string) string {
	val, err := strconv.ParseUint(hexIP, 16, 32)
	if err != nil {
		return hexIP
	}
	return fmt.Sprintf("%d.%d.%d.%d",
		val&0xFF, (val>>8)&0xFF, (val>>16)&0xFF, (val>>24)&0xFF)
}

// findProcessBySocketInode resolves a socket inode to a process name
// by scanning /proc/[pid]/fd/ for matching socket references.
func findProcessBySocketInode(inode string) string {
	if inode == "0" {
		return ""
	}

	target := "socket:[" + inode + "]"

	procs, err := os.ReadDir("/proc")
	if err != nil {
		return ""
	}

	for _, entry := range procs {
		if !entry.IsDir() {
			continue
		}
		pid := entry.Name()
		if len(pid) == 0 || pid[0] < '0' || pid[0] > '9' {
			continue
		}

		fdDir := filepath.Join("/proc", pid, "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue // no permission to read this process
		}

		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err == nil && link == target {
				comm, err := os.ReadFile(filepath.Join("/proc", pid, "comm"))
				if err == nil {
					return strings.TrimSpace(string(comm))
				}
				return "pid " + pid
			}
		}
	}

	return ""
}

func checkBootFile(label, path, expectedArch string) PreflightResult {
	name := fmt.Sprintf("bootloader/%s", label)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return PreflightResult{
			Name:    name,
			Status:  domain.StatusFail,
			Message: fmt.Sprintf("%s: file not found at %s", label, path),
		}
	}

	info := ValidateBootFile(path, expectedArch)

	if len(info.Problems) > 0 {
		return PreflightResult{
			Name:    name,
			Status:  domain.StatusFail,
			Message: fmt.Sprintf("%s: %s", label, info.Problems[0]),
		}
	}

	msg := fmt.Sprintf("%s: %s %s (%d bytes)", label, info.Format, info.Arch, info.Size)
	if info.HasIPXE {
		msg += ", iPXE"
	}

	status := domain.StatusOK
	if !info.HasIPXE {
		status = domain.StatusWarn
		msg += " (no iPXE signature found)"
	}

	return PreflightResult{
		Name:    name,
		Status:  status,
		Message: msg,
	}
}

func checkPXEConflict(ifaceName string) PreflightResult {
	results, err := ProbePXEServers(ifaceName, 3*time.Second)
	if err != nil {
		return PreflightResult{
			Name:    "pxe_conflict",
			Status:  domain.StatusWarn,
			Message: fmt.Sprintf("PXE conflict check skipped: %v", err),
		}
	}

	if len(results) == 0 {
		return PreflightResult{
			Name:    "pxe_conflict",
			Status:  domain.StatusOK,
			Message: "no competing PXE servers detected",
		}
	}

	msg := fmt.Sprintf("%d competing PXE server(s) detected:", len(results))
	for _, r := range results {
		msg += fmt.Sprintf(" %s", r.ServerIP)
		if r.Bootfile != "" {
			msg += fmt.Sprintf(" (file=%s)", r.Bootfile)
		}
	}

	return PreflightResult{
		Name:    "pxe_conflict",
		Status:  domain.StatusWarn,
		Message: msg,
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
