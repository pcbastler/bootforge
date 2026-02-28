// Package wol implements Wake-on-LAN magic packet sending.
package wol

import (
	"fmt"
	"net"
)

// MagicPacket builds a Wake-on-LAN magic packet for the given MAC address.
// The packet is 102 bytes: 6 bytes of 0xFF followed by the MAC repeated 16 times.
func MagicPacket(mac net.HardwareAddr) ([]byte, error) {
	if len(mac) != 6 {
		return nil, fmt.Errorf("invalid MAC address length: %d (expected 6)", len(mac))
	}

	packet := make([]byte, 102)

	// 6 bytes of 0xFF.
	for i := 0; i < 6; i++ {
		packet[i] = 0xFF
	}

	// MAC repeated 16 times.
	for i := 0; i < 16; i++ {
		copy(packet[6+i*6:], mac)
	}

	return packet, nil
}

// Send sends a Wake-on-LAN magic packet to the broadcast address on port 9.
func Send(mac net.HardwareAddr, broadcastAddr string) error {
	packet, err := MagicPacket(mac)
	if err != nil {
		return err
	}

	if broadcastAddr == "" {
		broadcastAddr = "255.255.255.255:9"
	}

	addr, err := net.ResolveUDPAddr("udp4", broadcastAddr)
	if err != nil {
		return fmt.Errorf("resolving broadcast address %q: %w", broadcastAddr, err)
	}

	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return fmt.Errorf("dialing UDP: %w", err)
	}
	defer conn.Close()

	_, err = conn.Write(packet)
	if err != nil {
		return fmt.Errorf("sending magic packet: %w", err)
	}

	return nil
}
