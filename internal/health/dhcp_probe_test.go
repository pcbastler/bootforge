package health

import (
	"net"
	"testing"
	"time"

	"bootforge/internal/domain"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

func TestBuildPXEDiscover(t *testing.T) {
	mac, _ := net.ParseMAC("02:00:00:00:00:01")
	discover, err := BuildPXEDiscover(mac)
	if err != nil {
		t.Fatalf("BuildPXEDiscover: %v", err)
	}

	// Check message type.
	if discover.MessageType() != dhcpv4.MessageTypeDiscover {
		t.Errorf("message type = %v, want Discover", discover.MessageType())
	}

	// Check broadcast flag.
	if !discover.IsBroadcast() {
		t.Error("broadcast flag should be set")
	}

	// Check PXEClient vendor class (Option 60).
	classID := discover.Options.Get(dhcpv4.OptionClassIdentifier)
	if classID == nil {
		t.Fatal("Option 60 (Class Identifier) missing")
	}
	if string(classID) != "PXEClient" {
		t.Errorf("class identifier = %q, want PXEClient", string(classID))
	}

	// Check client architecture (Option 93).
	archOpt := discover.Options.Get(dhcpv4.OptionClientSystemArchitectureType)
	if archOpt == nil {
		t.Fatal("Option 93 (Client System Architecture) missing")
	}
	if len(archOpt) != 2 || archOpt[0] != 0x00 || archOpt[1] != 0x07 {
		t.Errorf("architecture option = %x, want 0007", archOpt)
	}

	// Check MAC address.
	if discover.ClientHWAddr.String() != mac.String() {
		t.Errorf("client MAC = %s, want %s", discover.ClientHWAddr, mac)
	}
}

func TestParsePXEOfferResponse(t *testing.T) {
	// Build a discover to get a transaction ID.
	mac, _ := net.ParseMAC("02:00:00:00:00:01")
	discover, err := BuildPXEDiscover(mac)
	if err != nil {
		t.Fatalf("BuildPXEDiscover: %v", err)
	}

	// Build a mock PXE offer response.
	reply, err := dhcpv4.NewReplyFromRequest(discover,
		dhcpv4.WithMessageType(dhcpv4.MessageTypeOffer),
		dhcpv4.WithServerIP(net.IPv4(192, 168, 1, 100)),
	)
	if err != nil {
		t.Fatalf("NewReplyFromRequest: %v", err)
	}

	reply.ServerIPAddr = net.IPv4(192, 168, 1, 100)
	reply.BootFileName = "ipxe.efi"
	reply.Options.Update(dhcpv4.OptGeneric(
		dhcpv4.OptionClassIdentifier, []byte("PXEClient")))

	// Verify the reply can be serialized and parsed back.
	data := reply.ToBytes()
	parsed, err := dhcpv4.FromBytes(data)
	if err != nil {
		t.Fatalf("FromBytes: %v", err)
	}

	// Transaction ID must match.
	if parsed.TransactionID != discover.TransactionID {
		t.Errorf("transaction ID mismatch: %v != %v", parsed.TransactionID, discover.TransactionID)
	}

	// Message type must be Offer.
	if parsed.MessageType() != dhcpv4.MessageTypeOffer {
		t.Errorf("message type = %v, want Offer", parsed.MessageType())
	}

	// Server IP.
	if !parsed.ServerIPAddr.Equal(net.IPv4(192, 168, 1, 100)) {
		t.Errorf("server IP = %s, want 192.168.1.100", parsed.ServerIPAddr)
	}

	// Boot filename.
	if parsed.BootFileName != "ipxe.efi" {
		t.Errorf("boot filename = %q, want ipxe.efi", parsed.BootFileName)
	}
}

func TestProbePXEServers_Loopback(t *testing.T) {
	// Probing on loopback should return nil immediately (skipped).
	results, err := ProbePXEServers("lo", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("ProbePXEServers on loopback: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results on loopback, got %v", results)
	}
}

func TestProbePXEServers_NonexistentInterface(t *testing.T) {
	_, err := ProbePXEServers("nonexistent0", 100*time.Millisecond)
	if err == nil {
		t.Error("expected error for nonexistent interface")
	}
}

func TestDHCPProbe_Disabled(t *testing.T) {
	probe := NewDHCPProbe(false, 67)
	result := probe.Check()

	if result.Status != domain.StatusOK {
		t.Errorf("status = %v, want OK", result.Status)
	}
	if result.Name != "dhcp" {
		t.Errorf("name = %q, want dhcp", result.Name)
	}
}

func TestDHCPProbe_Enabled(t *testing.T) {
	probe := NewDHCPProbe(true, 67)
	result := probe.Check()

	if result.Status != domain.StatusOK {
		t.Errorf("status = %v, want OK", result.Status)
	}
}

func TestCheckPXEConflict_Loopback(t *testing.T) {
	result := checkPXEConflict("lo")

	if result.Name != "pxe_conflict" {
		t.Errorf("name = %q, want pxe_conflict", result.Name)
	}
	// On loopback, the probe is skipped — should report OK (no servers).
	if result.Status != domain.StatusOK {
		t.Errorf("status = %v, want OK", result.Status)
	}
}

func TestPreflightIncludesPXEConflict(t *testing.T) {
	cfg := &domain.FullConfig{
		Server: domain.ServerConfig{
			Interface: "lo",
			DataDir:   t.TempDir(),
		},
		DHCPProxy: domain.DHCPProxyConfig{
			Enabled:   true,
			Port:      16767,
			ProxyPort: 14011,
		},
		Bootloader: domain.BootloaderConfig{
			Dir: "bootloader",
		},
	}

	results, _ := RunPreflight(cfg)

	found := false
	for _, r := range results {
		if r.Name == "pxe_conflict" {
			found = true
			if r.Status != domain.StatusOK {
				t.Errorf("pxe_conflict status = %v, want OK (loopback)", r.Status)
			}
		}
	}
	if !found {
		t.Error("pxe_conflict check should be present when DHCP proxy is enabled")
	}
}

func TestPreflightSkipsPXEConflictWhenDisabled(t *testing.T) {
	cfg := &domain.FullConfig{
		Server: domain.ServerConfig{
			Interface: "lo",
			DataDir:   t.TempDir(),
		},
		DHCPProxy: domain.DHCPProxyConfig{
			Enabled: false,
		},
		Bootloader: domain.BootloaderConfig{
			Dir: "bootloader",
		},
	}

	results, _ := RunPreflight(cfg)

	for _, r := range results {
		if r.Name == "pxe_conflict" {
			t.Error("pxe_conflict check should not be present when DHCP proxy is disabled")
		}
	}
}
