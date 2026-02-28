package server

import (
	"fmt"
	"net"
	"sync"

	"bootforge/internal/domain"
)

// Registry provides MAC-based client lookup with wildcard fallback
// and one-time boot override support.
type Registry struct {
	mu        sync.RWMutex
	clients   map[string]*domain.Client // MAC string → Client
	wildcard  *domain.Client
	overrides map[string]string // MAC string → menu entry name
}

// NewRegistry creates a Registry from a list of clients.
func NewRegistry(clients []*domain.Client) *Registry {
	r := &Registry{
		clients:   make(map[string]*domain.Client),
		overrides: make(map[string]string),
	}
	r.loadClients(clients)
	return r
}

// FindClient looks up a client by MAC address.
// Returns the exact match, falls back to the wildcard client,
// or returns an error if no match is found.
func (r *Registry) FindClient(mac net.HardwareAddr) (*domain.Client, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := mac.String()
	if c, ok := r.clients[key]; ok {
		return c, nil
	}
	if r.wildcard != nil {
		return r.wildcard, nil
	}
	return nil, fmt.Errorf("no client config for MAC %s", mac)
}

// SetBootOverride sets a one-time boot override for the given MAC.
// The next call to ConsumeBootOverride for this MAC will return the
// override entry name and clear it.
func (r *Registry) SetBootOverride(mac net.HardwareAddr, entryName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.overrides[mac.String()] = entryName
}

// ConsumeBootOverride returns and clears a one-time boot override
// for the given MAC. Returns empty string if no override is set.
func (r *Registry) ConsumeBootOverride(mac net.HardwareAddr) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := mac.String()
	entry, ok := r.overrides[key]
	if !ok {
		return ""
	}
	delete(r.overrides, key)
	return entry
}

// Reload replaces the client list while preserving existing overrides.
func (r *Registry) Reload(clients []*domain.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loadClients(clients)
}

// ListClients returns all registered clients.
func (r *Registry) ListClients() []*domain.Client {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*domain.Client, 0, len(r.clients)+1)
	for _, c := range r.clients {
		result = append(result, c)
	}
	if r.wildcard != nil {
		result = append(result, r.wildcard)
	}
	return result
}

func (r *Registry) loadClients(clients []*domain.Client) {
	r.clients = make(map[string]*domain.Client, len(clients))
	r.wildcard = nil

	for _, c := range clients {
		if c.IsWildcard() {
			r.wildcard = c
		} else {
			r.clients[c.MAC.String()] = c
		}
	}
}
