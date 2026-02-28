// Package dhcp implements the DHCP Proxy server (ports 67/4011).
package dhcp

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"bootforge/internal/domain"
	"bootforge/internal/server"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

// ProxyServer listens for DHCP Discover/Request messages and responds
// with PXE boot information without assigning IP addresses.
type ProxyServer struct {
	cfg      domain.DHCPProxyConfig
	blCfg    domain.BootloaderConfig
	serverIP net.IP
	srv      *server.Server
	logger   *slog.Logger
	conn67   *net.UDPConn
	conn4011 *net.UDPConn
	cancel   context.CancelFunc
}

// NewProxyServer creates a new DHCP proxy server.
func NewProxyServer(cfg domain.DHCPProxyConfig, blCfg domain.BootloaderConfig, serverIP net.IP, srv *server.Server, logger *slog.Logger) *ProxyServer {
	return &ProxyServer{
		cfg:      cfg,
		blCfg:    blCfg,
		serverIP: serverIP,
		srv:      srv,
		logger:   logger,
	}
}

// Name returns the service name.
func (p *ProxyServer) Name() string { return "dhcp-proxy" }

// Start begins listening on the DHCP port and proxy port.
func (p *ProxyServer) Start(ctx context.Context) error {
	ctx, p.cancel = context.WithCancel(ctx)

	// Listen on port 67 for DHCP Discover.
	addr67 := &net.UDPAddr{Port: p.cfg.Port}
	conn67, err := net.ListenUDP("udp4", addr67)
	if err != nil {
		return fmt.Errorf("listening on port %d: %w", p.cfg.Port, err)
	}
	p.conn67 = conn67

	// Listen on port 4011 for PXE proxy requests.
	addr4011 := &net.UDPAddr{Port: p.cfg.ProxyPort}
	conn4011, err := net.ListenUDP("udp4", addr4011)
	if err != nil {
		conn67.Close()
		return fmt.Errorf("listening on port %d: %w", p.cfg.ProxyPort, err)
	}
	p.conn4011 = conn4011

	go p.serve(ctx, conn67, "port-67")
	go p.serve(ctx, conn4011, "port-4011")

	p.logger.Info("DHCP proxy started",
		"port", p.cfg.Port,
		"proxy_port", p.cfg.ProxyPort,
	)
	return nil
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

func (p *ProxyServer) handleMessage(ctx context.Context, conn *net.UDPConn, peer *net.UDPAddr, msg *dhcpv4.DHCPv4) {
	msgType := msg.MessageType()

	p.logger.Debug("DHCP packet received",
		"type", msgType,
		"mac", msg.ClientHWAddr,
		"peer", peer,
	)

	// Only handle Discover and Request.
	if msgType != dhcpv4.MessageTypeDiscover && msgType != dhcpv4.MessageTypeRequest {
		p.logger.Debug("ignoring non-discover/request", "type", msgType, "mac", msg.ClientHWAddr)
		return
	}

	// Check for PXE client (Option 60 = "PXEClient").
	classID := msg.Options.Get(dhcpv4.OptionClassIdentifier)
	if classID == nil || string(classID) != "PXEClient" {
		p.logger.Debug("ignoring non-PXE DHCP packet",
			"mac", msg.ClientHWAddr,
			"class_id", string(classID),
		)
		return
	}

	// Parse architecture from Option 93.
	arch, err := ArchFromOption93(msg)
	if err != nil {
		p.logger.Debug("PXE client without architecture option",
			"mac", msg.ClientHWAddr,
			"error", err,
		)
		return
	}

	bootfile := BootfileForArch(arch, p.blCfg)
	if bootfile == "" {
		p.logger.Warn("no bootloader configured for architecture",
			"mac", msg.ClientHWAddr, "arch", arch)
		return
	}

	p.logger.Info("PXE client discovered",
		"mac", msg.ClientHWAddr,
		"arch", arch,
		"bootfile", bootfile,
		"type", msgType,
	)

	// Build proxy offer/ack.
	reply, err := BuildProxyOffer(msg, p.serverIP, bootfile, p.blCfg.ChainURL)
	if err != nil {
		p.logger.Error("building proxy offer", "mac", msg.ClientHWAddr, "error", err)
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
		p.logger.Error("sending DHCP reply", "mac", msg.ClientHWAddr, "error", err)
	}
}
