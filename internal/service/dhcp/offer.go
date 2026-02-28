package dhcp

import (
	"fmt"
	"net"
	"strings"

	"bootforge/internal/domain"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

// ArchFromOption93 extracts the client architecture from DHCP Option 93.
// Returns an error if the option is missing or contains an unknown value.
func ArchFromOption93(msg *dhcpv4.DHCPv4) (domain.ClientArch, error) {
	opt := msg.Options.Get(dhcpv4.OptionClientSystemArchitectureType)
	if opt == nil {
		return 0, fmt.Errorf("DHCP option 93 (Client System Architecture) not present")
	}
	if len(opt) < 2 {
		return 0, fmt.Errorf("DHCP option 93 too short: %d bytes", len(opt))
	}

	value := uint16(opt[0])<<8 | uint16(opt[1])
	return domain.ParseArch(value)
}

// BootfileForArch returns the bootloader filename for the given architecture.
func BootfileForArch(arch domain.ClientArch, bl domain.BootloaderConfig) string {
	return bl.FileForArch(arch)
}

// BuildProxyOffer creates a DHCP proxy offer response from a discover message.
// It sets the boot server (option 66), boot filename (option 67), and
// vendor-specific options (option 43) for PXE.
//
// When ipxeClient is true, the bootfile is an HTTP URL (chain URL) and PXE
// vendor options (option 43) are omitted since iPXE doesn't need them.
func BuildProxyOffer(discover *dhcpv4.DHCPv4, serverIP net.IP, bootfile string, ipxeClient bool) (*dhcpv4.DHCPv4, error) {
	reply, err := dhcpv4.NewReplyFromRequest(discover,
		dhcpv4.WithMessageType(dhcpv4.MessageTypeOffer),
		dhcpv4.WithServerIP(serverIP),
	)
	if err != nil {
		return nil, fmt.Errorf("building proxy offer: %w", err)
	}

	// Option 66: TFTP Server Name (next-server).
	reply.ServerIPAddr = serverIP
	reply.Options.Update(dhcpv4.OptGeneric(dhcpv4.OptionTFTPServerName, []byte(serverIP.String())))

	// Option 67: Boot Filename.
	reply.BootFileName = bootfile
	reply.Options.Update(dhcpv4.OptGeneric(dhcpv4.OptionBootfileName, []byte(bootfile)))

	if !ipxeClient {
		// Option 43: Vendor-Specific Information (PXE discovery control).
		// Only needed for raw PXE firmware, not for iPXE.
		vendorOpts := buildPXEVendorOptions(serverIP)
		reply.Options.Update(dhcpv4.OptGeneric(dhcpv4.OptionVendorSpecificInformation, vendorOpts))
	}

	// Option 54: Server Identifier (required by RFC 2131 in OFFER/ACK).
	reply.Options.Update(dhcpv4.OptGeneric(dhcpv4.OptionServerIdentifier, serverIP.To4()))

	// Option 60: Vendor Class Identifier = "PXEClient"
	reply.Options.Update(dhcpv4.OptGeneric(dhcpv4.OptionClassIdentifier, []byte("PXEClient")))

	return reply, nil
}

// isIPXE checks whether the DHCP client is already running iPXE by looking
// for "iPXE" in Option 77 (User Class). This distinguishes second-stage iPXE
// requests from first-stage UEFI/BIOS PXE firmware requests.
func isIPXE(msg *dhcpv4.DHCPv4) bool {
	userClass := msg.Options.Get(dhcpv4.OptionUserClassInformation)
	return userClass != nil && strings.Contains(string(userClass), "iPXE")
}

// buildPXEVendorOptions constructs PXE vendor-specific sub-options (Option 43).
func buildPXEVendorOptions(serverIP net.IP) []byte {
	// Sub-option 6: PXE_DISCOVERY_CONTROL
	// Value 8: skip discovery, use boot server IP directly
	discCtrl := []byte{6, 1, 8}

	// Sub-option 8: PXE_BOOT_SERVERS
	// Type(2) + Count(1) + IP(4)
	bootServers := []byte{8, 7, 0, 0, 1}
	bootServers = append(bootServers, serverIP.To4()...)

	// Sub-option 9: PXE_BOOT_MENU
	// Type(2) + DescLen(1) + Description
	desc := []byte("Bootforge PXE")
	bootMenu := []byte{9, byte(3 + len(desc)), 0, 0, byte(len(desc))}
	bootMenu = append(bootMenu, desc...)

	// Sub-option 10: PXE_BOOT_PROMPT
	// Timeout(1) + Prompt
	prompt := []byte("Bootforge")
	bootPrompt := []byte{10, byte(1 + len(prompt)), 0}
	bootPrompt = append(bootPrompt, prompt...)

	// End marker
	end := []byte{255}

	var result []byte
	result = append(result, discCtrl...)
	result = append(result, bootServers...)
	result = append(result, bootMenu...)
	result = append(result, bootPrompt...)
	result = append(result, end...)
	return result
}
