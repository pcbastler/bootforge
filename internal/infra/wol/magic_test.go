package wol

import (
	"net"
	"testing"
)

func TestMagicPacketStructure(t *testing.T) {
	mac := net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}

	packet, err := MagicPacket(mac)
	if err != nil {
		t.Fatalf("MagicPacket() error = %v", err)
	}

	if len(packet) != 102 {
		t.Fatalf("packet length = %d, want 102", len(packet))
	}

	// First 6 bytes must be 0xFF.
	for i := 0; i < 6; i++ {
		if packet[i] != 0xFF {
			t.Errorf("packet[%d] = 0x%02X, want 0xFF", i, packet[i])
		}
	}

	// Next 96 bytes: MAC repeated 16 times.
	for rep := 0; rep < 16; rep++ {
		offset := 6 + rep*6
		for j := 0; j < 6; j++ {
			if packet[offset+j] != mac[j] {
				t.Errorf("packet[%d] (rep %d, byte %d) = 0x%02X, want 0x%02X",
					offset+j, rep, j, packet[offset+j], mac[j])
			}
		}
	}
}

func TestMagicPacketInvalidMAC(t *testing.T) {
	tests := []struct {
		name string
		mac  net.HardwareAddr
	}{
		{"nil", nil},
		{"empty", net.HardwareAddr{}},
		{"too short", net.HardwareAddr{0xAA, 0xBB, 0xCC}},
		{"too long", net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF, 0x11}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := MagicPacket(tt.mac)
			if err == nil {
				t.Error("MagicPacket() should fail for invalid MAC")
			}
		})
	}
}

func TestMagicPacketDifferentMACs(t *testing.T) {
	mac1 := net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x01}
	mac2 := net.HardwareAddr{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFE}

	p1, err := MagicPacket(mac1)
	if err != nil {
		t.Fatalf("MagicPacket(mac1) error = %v", err)
	}
	p2, err := MagicPacket(mac2)
	if err != nil {
		t.Fatalf("MagicPacket(mac2) error = %v", err)
	}

	// Same header (0xFF x6), different payload.
	for i := 0; i < 6; i++ {
		if p1[i] != p2[i] {
			t.Errorf("headers should match at byte %d", i)
		}
	}
	// Payload should differ.
	if p1[6] == p2[6] {
		t.Error("payloads should differ for different MACs")
	}
}

func TestMagicPacketAllZerosMAC(t *testing.T) {
	mac := net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	packet, err := MagicPacket(mac)
	if err != nil {
		t.Fatalf("MagicPacket() error = %v", err)
	}

	// Header should be 0xFF, payload should be 0x00.
	for i := 0; i < 6; i++ {
		if packet[i] != 0xFF {
			t.Errorf("header[%d] = 0x%02X, want 0xFF", i, packet[i])
		}
	}
	for i := 6; i < 102; i++ {
		if packet[i] != 0x00 {
			t.Errorf("payload[%d] = 0x%02X, want 0x00", i, packet[i])
		}
	}
}
