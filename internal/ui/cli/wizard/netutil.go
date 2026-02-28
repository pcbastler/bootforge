package wizard

import (
	"fmt"
	"net"
)

// NetInterface represents a detected network interface with its IP.
type NetInterface struct {
	Name string
	IP   string
}

// String returns a human-readable label like "eth0 (192.168.1.10)".
func (n NetInterface) String() string {
	if n.IP != "" {
		return fmt.Sprintf("%s (%s)", n.Name, n.IP)
	}
	return n.Name
}

// DetectInterfaces returns network interfaces that are UP, not loopback,
// and have at least one IPv4 address.
func DetectInterfaces() ([]NetInterface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("listing interfaces: %w", err)
	}

	var result []NetInterface
	for _, iface := range ifaces {
		// Skip down, loopback, and point-to-point interfaces.
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		ip := firstIPv4(addrs)
		if ip == "" {
			continue
		}

		result = append(result, NetInterface{Name: iface.Name, IP: ip})
	}

	return result, nil
}

// firstIPv4 extracts the first IPv4 address from a list of net.Addr.
func firstIPv4(addrs []net.Addr) string {
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		if v4 := ipNet.IP.To4(); v4 != nil {
			return v4.String()
		}
	}
	return ""
}
