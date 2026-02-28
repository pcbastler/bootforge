package server

import (
	"net"
	"sync"
	"testing"

	"bootforge/internal/domain"
)

func testClients() []*domain.Client {
	return []*domain.Client{
		{
			MAC:  net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01},
			Name: "ws-01",
			Menu: domain.MenuConfig{Entries: []string{"rescue"}},
		},
		{
			MAC:  net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x02},
			Name: "ws-02",
			Menu: domain.MenuConfig{Entries: []string{"ubuntu-install"}},
		},
		{
			MAC:  domain.WildcardMAC,
			Name: "default",
			Menu: domain.MenuConfig{Entries: []string{"rescue"}},
		},
	}
}

func TestRegistryFindClientExact(t *testing.T) {
	r := NewRegistry(testClients())

	mac := net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01}
	c, err := r.FindClient(mac)
	if err != nil {
		t.Fatalf("FindClient() error = %v", err)
	}
	if c.Name != "ws-01" {
		t.Errorf("FindClient() = %q, want %q", c.Name, "ws-01")
	}
}

func TestRegistryFindClientWildcardFallback(t *testing.T) {
	r := NewRegistry(testClients())

	unknownMAC := net.HardwareAddr{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	c, err := r.FindClient(unknownMAC)
	if err != nil {
		t.Fatalf("FindClient() error = %v", err)
	}
	if c.Name != "default" {
		t.Errorf("FindClient() = %q, want %q (wildcard fallback)", c.Name, "default")
	}
}

func TestRegistryFindClientNoWildcardError(t *testing.T) {
	// No wildcard client.
	clients := []*domain.Client{
		{
			MAC:  net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01},
			Name: "ws-01",
			Menu: domain.MenuConfig{Entries: []string{"rescue"}},
		},
	}
	r := NewRegistry(clients)

	unknownMAC := net.HardwareAddr{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	_, err := r.FindClient(unknownMAC)
	if err == nil {
		t.Error("FindClient() should fail without wildcard fallback")
	}
}

func TestRegistryBootOverrideSetAndConsume(t *testing.T) {
	r := NewRegistry(testClients())
	mac := net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01}

	r.SetBootOverride(mac, "ubuntu-install")

	entry := r.ConsumeBootOverride(mac)
	if entry != "ubuntu-install" {
		t.Errorf("ConsumeBootOverride() = %q, want %q", entry, "ubuntu-install")
	}

	// Second consume should be empty.
	entry = r.ConsumeBootOverride(mac)
	if entry != "" {
		t.Errorf("ConsumeBootOverride() = %q, want empty (already consumed)", entry)
	}
}

func TestRegistryBootOverrideNoOverride(t *testing.T) {
	r := NewRegistry(testClients())
	mac := net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01}

	entry := r.ConsumeBootOverride(mac)
	if entry != "" {
		t.Errorf("ConsumeBootOverride() = %q, want empty (no override set)", entry)
	}
}

func TestRegistryBootOverrideUnknownMAC(t *testing.T) {
	r := NewRegistry(testClients())
	unknownMAC := net.HardwareAddr{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}

	// Setting override for unknown MAC is allowed (for WoL).
	r.SetBootOverride(unknownMAC, "rescue")
	entry := r.ConsumeBootOverride(unknownMAC)
	if entry != "rescue" {
		t.Errorf("ConsumeBootOverride() = %q, want %q", entry, "rescue")
	}
}

func TestRegistryReloadPreservesOverrides(t *testing.T) {
	r := NewRegistry(testClients())
	mac := net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01}
	r.SetBootOverride(mac, "ubuntu-install")

	// Reload with different clients.
	newClients := []*domain.Client{
		{
			MAC:  net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01},
			Name: "ws-01-updated",
			Menu: domain.MenuConfig{Entries: []string{"rescue"}},
		},
	}
	r.Reload(newClients)

	// Override should still be there.
	entry := r.ConsumeBootOverride(mac)
	if entry != "ubuntu-install" {
		t.Errorf("ConsumeBootOverride() after reload = %q, want %q", entry, "ubuntu-install")
	}

	// Client should be updated.
	c, _ := r.FindClient(mac)
	if c.Name != "ws-01-updated" {
		t.Errorf("FindClient() after reload = %q, want %q", c.Name, "ws-01-updated")
	}
}

func TestRegistryListClients(t *testing.T) {
	r := NewRegistry(testClients())
	clients := r.ListClients()
	if len(clients) != 3 {
		t.Errorf("ListClients() count = %d, want 3", len(clients))
	}
}

func TestRegistryConcurrentAccess(t *testing.T) {
	r := NewRegistry(testClients())
	mac := net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			r.FindClient(mac)
		}()
		go func() {
			defer wg.Done()
			r.SetBootOverride(mac, "rescue")
		}()
		go func() {
			defer wg.Done()
			r.ConsumeBootOverride(mac)
		}()
	}
	wg.Wait()
}
