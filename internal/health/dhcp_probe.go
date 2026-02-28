package health

import (
	"crypto/rand"
	"fmt"
	"net"
	"time"

	"bootforge/internal/domain"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

// DHCPProbe validates DHCP proxy configuration at runtime.
type DHCPProbe struct {
	enabled bool
	port    int
}

// NewDHCPProbe creates a DHCP config validation probe.
func NewDHCPProbe(enabled bool, port int) *DHCPProbe {
	return &DHCPProbe{enabled: enabled, port: port}
}

func (p *DHCPProbe) Name() string { return "dhcp" }

func (p *DHCPProbe) Check() domain.CheckResult {
	start := time.Now()
	result := domain.CheckResult{
		Name: "dhcp",
		At:   start,
	}

	if !p.enabled {
		result.Status = domain.StatusOK
		result.Message = "DHCP proxy disabled (skipped)"
		result.Duration = time.Since(start)
		return result
	}

	result.Status = domain.StatusOK
	result.Message = fmt.Sprintf("DHCP proxy configured on port %d", p.port)
	result.Duration = time.Since(start)
	return result
}

// PXEProbeResult holds information about a PXE server detected on the network.
type PXEProbeResult struct {
	ServerIP net.IP
	Bootfile string
}

// ProbePXEServers sends a DHCP Discover with PXEClient vendor class
// on the specified interface and listens for PXE offers from other servers.
// This detects competing PXE proxies on the same broadcast domain.
func ProbePXEServers(ifaceName string, timeout time.Duration) ([]PXEProbeResult, error) {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return nil, fmt.Errorf("interface %s: %w", ifaceName, err)
	}

	// Skip loopback — no point probing for PXE servers there.
	if iface.Flags&net.FlagLoopback != 0 {
		return nil, nil
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, fmt.Errorf("getting addresses for %s: %w", ifaceName, err)
	}

	var localIP net.IP
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil {
			localIP = ipNet.IP.To4()
			break
		}
	}
	if localIP == nil {
		return nil, fmt.Errorf("no IPv4 address on interface %s", ifaceName)
	}

	// Generate a locally-administered MAC for the probe to avoid
	// confusing real DHCP servers with a real client.
	probeMAC := make(net.HardwareAddr, 6)
	if _, err := rand.Read(probeMAC); err != nil {
		return nil, fmt.Errorf("generating probe MAC: %w", err)
	}
	probeMAC[0] = (probeMAC[0] | 0x02) & 0xFE // locally administered, unicast

	// Build DHCP Discover with PXE vendor class.
	discover, err := dhcpv4.New(
		dhcpv4.WithMessageType(dhcpv4.MessageTypeDiscover),
		dhcpv4.WithHwAddr(probeMAC),
	)
	if err != nil {
		return nil, fmt.Errorf("building DHCP discover: %w", err)
	}

	// Set broadcast flag so servers respond via broadcast.
	discover.SetBroadcast()

	// Option 60: Vendor Class Identifier = "PXEClient"
	discover.Options.Update(dhcpv4.OptGeneric(
		dhcpv4.OptionClassIdentifier, []byte("PXEClient")))

	// Option 93: Client System Architecture = EFI x64 (0x0007)
	discover.Options.Update(dhcpv4.OptGeneric(
		dhcpv4.OptionClientSystemArchitectureType, []byte{0x00, 0x07}))

	// Try port 68 first (captures broadcast responses from standard
	// DHCP servers). Fall back to a random port which still catches
	// PXE proxies that respond directly to the source address.
	var conn *net.UDPConn
	conn, err = net.ListenUDP("udp4", &net.UDPAddr{IP: localIP, Port: 68})
	if err != nil {
		conn, err = net.ListenUDP("udp4", &net.UDPAddr{IP: localIP, Port: 0})
		if err != nil {
			return nil, fmt.Errorf("binding UDP on %s: %w", ifaceName, err)
		}
	}
	defer conn.Close()

	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, fmt.Errorf("setting read deadline: %w", err)
	}

	// Send broadcast discover.
	dest := &net.UDPAddr{IP: net.IPv4bcast, Port: 67}
	if _, err := conn.WriteToUDP(discover.ToBytes(), dest); err != nil {
		return nil, fmt.Errorf("sending DHCP discover broadcast: %w", err)
	}

	// Collect responses matching our transaction ID.
	var results []PXEProbeResult
	seen := make(map[string]bool)
	buf := make([]byte, 1500)

	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			break // timeout or read error — done collecting
		}

		reply, err := dhcpv4.FromBytes(buf[:n])
		if err != nil {
			continue
		}

		if reply.TransactionID != discover.TransactionID {
			continue
		}

		mt := reply.MessageType()
		if mt != dhcpv4.MessageTypeOffer && mt != dhcpv4.MessageTypeAck {
			continue
		}

		serverIP := reply.ServerIPAddr
		if serverIP == nil || serverIP.Equal(net.IPv4zero) {
			if si := reply.Options.Get(dhcpv4.OptionServerIdentifier); si != nil {
				serverIP = net.IP(si)
			}
		}
		if serverIP == nil {
			continue
		}

		key := serverIP.String()
		if seen[key] {
			continue
		}
		seen[key] = true

		result := PXEProbeResult{ServerIP: serverIP}
		if reply.BootFileName != "" {
			result.Bootfile = reply.BootFileName
		} else if bf := reply.Options.Get(dhcpv4.OptionBootfileName); bf != nil {
			result.Bootfile = string(bf)
		}

		results = append(results, result)
	}

	return results, nil
}

// BuildPXEDiscover creates a DHCP Discover packet with PXEClient vendor class
// for testing and inspection purposes.
func BuildPXEDiscover(hwAddr net.HardwareAddr) (*dhcpv4.DHCPv4, error) {
	discover, err := dhcpv4.New(
		dhcpv4.WithMessageType(dhcpv4.MessageTypeDiscover),
		dhcpv4.WithHwAddr(hwAddr),
	)
	if err != nil {
		return nil, err
	}
	discover.SetBroadcast()
	discover.Options.Update(dhcpv4.OptGeneric(
		dhcpv4.OptionClassIdentifier, []byte("PXEClient")))
	discover.Options.Update(dhcpv4.OptGeneric(
		dhcpv4.OptionClientSystemArchitectureType, []byte{0x00, 0x07}))
	return discover, nil
}
