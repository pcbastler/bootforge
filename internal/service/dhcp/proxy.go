// Package dhcp implements the DHCP Proxy server (ports 67/4011).
package dhcp

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"syscall"

	"bootforge/internal/domain"
	"bootforge/internal/server"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

// ProxyServer listens for DHCP Discover/Request messages and responds
// with PXE boot information without assigning IP addresses.
type ProxyServer struct {
	cfg       domain.DHCPProxyConfig
	blCfg     domain.BootloaderConfig
	serverIP  net.IP
	ifaceName string
	httpPort  int
	srv       *server.Server
	logger    *slog.Logger
	conn67    *net.UDPConn
	conn4011  *net.UDPConn
	cancel    context.CancelFunc
}

// NewProxyServer creates a new DHCP proxy server.
func NewProxyServer(cfg domain.DHCPProxyConfig, blCfg domain.BootloaderConfig, serverIP net.IP, ifaceName string, httpPort int, srv *server.Server, logger *slog.Logger) *ProxyServer {
	return &ProxyServer{
		cfg:       cfg,
		blCfg:     blCfg,
		serverIP:  serverIP,
		ifaceName: ifaceName,
		httpPort:  httpPort,
		srv:       srv,
		logger:    logger,
	}
}

// Name returns the service name.
func (p *ProxyServer) Name() string { return "dhcp-proxy" }

// Start begins listening on the DHCP port and proxy port.
func (p *ProxyServer) Start(ctx context.Context) error {
	ctx, p.cancel = context.WithCancel(ctx)

	// Listen on port 67 for DHCP Discover.
	conn67, err := p.listenUDP(ctx, p.cfg.Port)
	if err != nil {
		return fmt.Errorf("listening on port %d: %w", p.cfg.Port, err)
	}
	p.conn67 = conn67

	// Listen on port 4011 for PXE proxy requests.
	conn4011, err := p.listenUDP(ctx, p.cfg.ProxyPort)
	if err != nil {
		conn67.Close()
		return fmt.Errorf("listening on port %d: %w", p.cfg.ProxyPort, err)
	}
	p.conn4011 = conn4011

	go p.serve(ctx, conn67, "port-67")
	go p.serve(ctx, conn4011, "port-4011")

	p.logger.Info("DHCP proxy started",
		"interface", p.ifaceName,
		"port", p.cfg.Port,
		"proxy_port", p.cfg.ProxyPort,
	)
	return nil
}

// listenUDP creates a UDP socket bound to the configured interface via
// SO_BINDTODEVICE with SO_BROADCAST enabled. SO_BINDTODEVICE ensures DHCP
// broadcast packets arriving on the interface are delivered to our socket.
// SO_BROADCAST allows sending replies to the broadcast address (255.255.255.255).
func (p *ProxyServer) listenUDP(ctx context.Context, port int) (*net.UDPConn, error) {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var opErr error
			if err := c.Control(func(fd uintptr) {
				opErr = syscall.SetsockoptString(
					int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, p.ifaceName)
				if opErr != nil {
					return
				}
				opErr = syscall.SetsockoptInt(
					int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
			}); err != nil {
				return err
			}
			return opErr
		},
	}

	pc, err := lc.ListenPacket(ctx, "udp4", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}

	conn, ok := pc.(*net.UDPConn)
	if !ok {
		pc.Close()
		return nil, fmt.Errorf("unexpected connection type for udp4")
	}

	return conn, nil
}

// Stop shuts down the DHCP proxy server.
func (p *ProxyServer) Stop(_ context.Context) error {
	if p.cancel != nil {
		p.cancel()
	}
	if p.conn67 != nil {
		p.conn67.Close()
	}
	if p.conn4011 != nil {
		p.conn4011.Close()
	}
	return nil
}

func (p *ProxyServer) serve(ctx context.Context, conn *net.UDPConn, name string) {
	buf := make([]byte, 1500)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, peer, err := conn.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				p.logger.Error("DHCP read error", "listener", name, "error", err)
				continue
			}
		}

		msg, err := dhcpv4.FromBytes(buf[:n])
		if err != nil {
			p.logger.Debug("invalid DHCP packet", "listener", name, "error", err)
			continue
		}

		go p.handleMessage(ctx, conn, peer, msg)
	}
}

// resolveChainURL replaces template variables in the chain URL with actual values.
func (p *ProxyServer) resolveChainURL(mac string) string {
	url := p.blCfg.ChainURL
	url = strings.ReplaceAll(url, "${server_ip}", p.serverIP.String())
	url = strings.ReplaceAll(url, "${http_port}", fmt.Sprintf("%d", p.httpPort))
	url = strings.ReplaceAll(url, "${mac}", mac)
	return url
}

func (p *ProxyServer) handleMessage(ctx context.Context, conn *net.UDPConn, peer *net.UDPAddr, msg *dhcpv4.DHCPv4) {
	msgType := msg.MessageType()

	mac := msg.ClientHWAddr.String()

	p.logger.Debug("DHCP packet received",
		"type", msgType,
		"mac", mac,
		"peer", peer,
	)

	// Only handle Discover and Request.
	if msgType != dhcpv4.MessageTypeDiscover && msgType != dhcpv4.MessageTypeRequest {
		p.logger.Debug("ignoring non-discover/request", "type", msgType, "mac", mac)
		return
	}

	// Check for PXE client (Option 60 starts with "PXEClient").
	// PXE clients send "PXEClient:Arch:00007:UNDI:003016" etc.
	classID := msg.Options.Get(dhcpv4.OptionClassIdentifier)
	if classID == nil || !strings.Contains(string(classID), "PXEClient") {
		p.logger.Debug("ignoring non-PXE DHCP packet",
			"mac", mac,
			"class_id", string(classID),
		)
		return
	}

	// Parse architecture from Option 93.
	arch, err := ArchFromOption93(msg)
	if err != nil {
		p.logger.Debug("PXE client without architecture option",
			"mac", mac,
			"error", err,
		)
		return
	}

	// Detect whether the client is already running iPXE (second stage)
	// or raw UEFI/BIOS PXE firmware (first stage).
	ipxeClient := isIPXE(msg)

	var bootfile string
	if ipxeClient {
		// iPXE is loaded — respond with chain URL to the HTTP boot menu.
		bootfile = p.resolveChainURL(mac)
		if bootfile == "" {
			p.logger.Warn("no chain URL configured for iPXE client", "mac", mac)
			return
		}
		p.logger.Info("iPXE client discovered, chaining to menu",
			"mac", mac,
			"chain_url", bootfile,
			"type", msgType,
		)
	} else {
		// Raw PXE firmware — respond with TFTP bootloader file.
		bootfile = BootfileForArch(arch, p.blCfg)
		if bootfile == "" {
			p.logger.Warn("no bootloader configured for architecture",
				"mac", mac, "arch", arch)
			return
		}
		p.logger.Info("PXE client discovered",
			"mac", mac,
			"arch", arch,
			"bootfile", bootfile,
			"type", msgType,
		)
	}

	// Build proxy offer/ack.
	reply, err := BuildProxyOffer(msg, p.serverIP, bootfile, ipxeClient)
	if err != nil {
		p.logger.Error("building proxy offer", "mac", mac, "error", err)
		return
	}

	// For requests, change message type to ACK.
	if msgType == dhcpv4.MessageTypeRequest {
		reply.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeAck))
	}

	// Publish event.
	p.srv.Events.Publish(ctx, domain.Event{
		Type:    domain.EventDHCP,
		MAC:     msg.ClientHWAddr,
		Message: fmt.Sprintf("PXE %s → %s (arch=%s, file=%s)", msgType, reply.MessageType(), arch, bootfile),
	})

	// Send reply.
	dest := &net.UDPAddr{IP: net.IPv4bcast, Port: 68}
	if peer != nil && !peer.IP.Equal(net.IPv4zero) {
		dest = peer
	}

	if _, err := conn.WriteToUDP(reply.ToBytes(), dest); err != nil {
		p.logger.Error("sending DHCP reply", "mac", mac, "dest", dest, "error", err)
		return
	}

	p.logger.Debug("DHCP reply sent",
		"mac", mac,
		"type", reply.MessageType(),
		"dest", dest,
		"siaddr", reply.ServerIPAddr,
		"bootfile", reply.BootFileName,
	)
}
