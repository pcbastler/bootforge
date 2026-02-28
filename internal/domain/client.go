package domain

import (
	"fmt"
	"net"
	"strings"
)

// WildcardMAC is the sentinel value for the default/fallback client.
// It matches any MAC address not explicitly configured.
var WildcardMAC = net.HardwareAddr{0, 0, 0, 0, 0, 0}

// MenuConfig defines which menu entries a client can boot and the default behavior.
type MenuConfig struct {
	Entries []string // references to MenuEntry.Name
	Default string   // which entry boots on timeout (must be in Entries)
	Timeout int      // seconds before auto-boot, 0 = wait forever
}

// Validate checks that the menu config is well-formed.
func (mc *MenuConfig) Validate() error {
	if len(mc.Entries) == 0 {
		return fmt.Errorf("menu config: at least one entry is required")
	}
	if mc.Timeout < 0 {
		return fmt.Errorf("menu config: timeout must not be negative, got %d", mc.Timeout)
	}
	if mc.Default != "" {
		found := false
		for _, e := range mc.Entries {
			if e == mc.Default {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("menu config: default %q is not in entries %v", mc.Default, mc.Entries)
		}
	}
	return nil
}

// Client represents a PXE client identified by its MAC address.
type Client struct {
	MAC        net.HardwareAddr // unique identifier (or WildcardMAC for default)
	Name       string           // human-readable name
	Menu       MenuConfig       // which menu entries this client gets
	Vars       map[string]string // template variables for preseed/kickstart
	Enabled    bool             // whether this client is active
	SourceFile string           // which .toml file defined this client
}

// IsWildcard returns true if this client is the default/fallback client.
func (c *Client) IsWildcard() bool {
	return c.MAC != nil && c.MAC.String() == WildcardMAC.String()
}

// Validate checks that the client is well-formed.
func (c *Client) Validate() error {
	if c.MAC == nil {
		return fmt.Errorf("client: MAC address is required")
	}
	if c.Name == "" {
		return fmt.Errorf("client %s: name is required", c.MAC)
	}
	if err := c.Menu.Validate(); err != nil {
		return fmt.Errorf("client %s (%s): %w", c.MAC, c.Name, err)
	}
	return nil
}

// ParseMAC parses a MAC address string or the wildcard "*".
// Supports formats: "aa:bb:cc:dd:ee:ff", "AA:BB:CC:DD:EE:FF",
// "aa-bb-cc-dd-ee-ff", and "*" for wildcard.
func ParseMAC(s string) (net.HardwareAddr, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("MAC address is empty")
	}
	if s == "*" {
		return WildcardMAC, nil
	}
	// Normalize dashes to colons for net.ParseMAC compatibility.
	s = strings.ReplaceAll(s, "-", ":")
	mac, err := net.ParseMAC(s)
	if err != nil {
		return nil, fmt.Errorf("invalid MAC address %q: %w", s, err)
	}
	return mac, nil
}
