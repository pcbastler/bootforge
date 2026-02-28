package dhcp

import (
	"net"
	"testing"

	"bootforge/internal/domain"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

func TestArchFromOption93(t *testing.T) {
	tests := []struct {
		name    string
		value   []byte
		want    domain.ClientArch
		wantErr bool
	}{
		{"BIOS", []byte{0x00, 0x00}, domain.ArchBIOS, false},
		{"UEFI x86", []byte{0x00, 0x06}, domain.ArchUEFIx86, false},
		{"UEFI x64 type 7", []byte{0x00, 0x07}, domain.ArchUEFIx64, false},
		{"UEFI x64 type 9", []byte{0x00, 0x09}, domain.ArchUEFIx64, false},
		{"ARM64", []byte{0x00, 0x0B}, domain.ArchARM64, false},
		{"unknown", []byte{0x00, 0x05}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := buildDiscover(t, tt.value)
			got, err := ArchFromOption93(msg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ArchFromOption93() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ArchFromOption93() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestArchFromOption93Missing(t *testing.T) {
	msg, err := dhcpv4.New(dhcpv4.WithMessageType(dhcpv4.MessageTypeDiscover))
	if err != nil {
		t.Fatal(err)
	}
	// No option 93 set.
	_, err = ArchFromOption93(msg)
	if err == nil {
		t.Error("ArchFromOption93() should fail when option 93 is missing")
	}
}

func TestArchFromOption93TooShort(t *testing.T) {
	msg, err := dhcpv4.New(dhcpv4.WithMessageType(dhcpv4.MessageTypeDiscover))
	if err != nil {
		t.Fatal(err)
	}
	msg.Options.Update(dhcpv4.OptGeneric(dhcpv4.OptionClientSystemArchitectureType, []byte{0x00}))

	_, err = ArchFromOption93(msg)
	if err == nil {
		t.Error("ArchFromOption93() should fail when option 93 is too short")
	}
}

func TestBootfileForArch(t *testing.T) {
	bl := domain.BootloaderConfig{
		UEFX64: "ipxe.efi",
		UEFX86: "ipxe-x86.efi",
		BIOS:   "undionly.kpxe",
		ARM64:  "ipxe-arm64.efi",
	}

	tests := []struct {
		arch domain.ClientArch
		want string
	}{
		{domain.ArchUEFIx64, "ipxe.efi"},
		{domain.ArchUEFIx86, "ipxe-x86.efi"},
		{domain.ArchBIOS, "undionly.kpxe"},
		{domain.ArchARM64, "ipxe-arm64.efi"},
		{domain.ClientArch(99), ""},
	}

	for _, tt := range tests {
		t.Run(tt.arch.String(), func(t *testing.T) {
			got := BootfileForArch(tt.arch, bl)
			if got != tt.want {
				t.Errorf("BootfileForArch(%s) = %q, want %q", tt.arch, got, tt.want)
			}
		})
	}
}

func TestBuildProxyOffer(t *testing.T) {
	discover := buildDiscover(t, []byte{0x00, 0x07})
	serverIP := net.ParseIP("192.168.1.10")

	reply, err := BuildProxyOffer(discover, serverIP, "ipxe.efi", "http://192.168.1.10:8080/boot/${mac}/menu.ipxe")
	if err != nil {
		t.Fatalf("BuildProxyOffer() error = %v", err)
	}

	// Check message type.
	if reply.MessageType() != dhcpv4.MessageTypeOffer {
		t.Errorf("MessageType = %v, want Offer", reply.MessageType())
	}

	// Check server IP.
	if !reply.ServerIPAddr.Equal(serverIP) {
		t.Errorf("ServerIPAddr = %v, want %v", reply.ServerIPAddr, serverIP)
	}

	// Check boot filename.
	if reply.BootFileName != "ipxe.efi" {
		t.Errorf("BootFileName = %q, want %q", reply.BootFileName, "ipxe.efi")
	}

	// Check Option 66 (TFTP Server Name).
	opt66 := reply.Options.Get(dhcpv4.OptionTFTPServerName)
	if opt66 == nil {
		t.Error("Option 66 (TFTP Server Name) should be set")
	} else if string(opt66) != "192.168.1.10" {
		t.Errorf("Option 66 = %q, want %q", string(opt66), "192.168.1.10")
	}

	// Check Option 67 (Boot Filename).
	opt67 := reply.Options.Get(dhcpv4.OptionBootfileName)
	if opt67 == nil {
		t.Error("Option 67 (Boot Filename) should be set")
	} else if string(opt67) != "ipxe.efi" {
		t.Errorf("Option 67 = %q, want %q", string(opt67), "ipxe.efi")
	}

	// Check Option 43 (Vendor-Specific).
	opt43 := reply.Options.Get(dhcpv4.OptionVendorSpecificInformation)
	if opt43 == nil {
		t.Error("Option 43 (Vendor-Specific) should be set")
	}

	// Check Option 60 (Class Identifier).
	opt60 := reply.Options.Get(dhcpv4.OptionClassIdentifier)
	if opt60 == nil {
		t.Error("Option 60 (Class Identifier) should be set")
	} else if string(opt60) != "PXEClient" {
		t.Errorf("Option 60 = %q, want %q", string(opt60), "PXEClient")
	}

	// Check Transaction ID matches.
	if reply.TransactionID != discover.TransactionID {
		t.Errorf("TransactionID = %v, want %v", reply.TransactionID, discover.TransactionID)
	}
}

func TestBuildProxyOfferBIOS(t *testing.T) {
	discover := buildDiscover(t, []byte{0x00, 0x00})
	serverIP := net.ParseIP("10.0.0.1")

	reply, err := BuildProxyOffer(discover, serverIP, "undionly.kpxe", "")
	if err != nil {
		t.Fatalf("BuildProxyOffer() error = %v", err)
	}

	if reply.BootFileName != "undionly.kpxe" {
		t.Errorf("BootFileName = %q, want %q", reply.BootFileName, "undionly.kpxe")
	}
}

// buildDiscover creates a DHCP Discover message with Option 93.
func buildDiscover(t *testing.T, option93 []byte) *dhcpv4.DHCPv4 {
	t.Helper()
	msg, err := dhcpv4.New(
		dhcpv4.WithMessageType(dhcpv4.MessageTypeDiscover),
		dhcpv4.WithHwAddr(net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01}),
	)
	if err != nil {
		t.Fatal(err)
	}
	msg.Options.Update(dhcpv4.OptGeneric(dhcpv4.OptionClientSystemArchitectureType, option93))
	msg.Options.Update(dhcpv4.OptGeneric(dhcpv4.OptionClassIdentifier, []byte("PXEClient")))
	return msg
}
