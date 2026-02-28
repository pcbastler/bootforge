package store

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"bootforge/internal/domain"
)

func newSession(id string, mac net.HardwareAddr, state domain.BootState) *domain.Session {
	return &domain.Session{
		ID:        id,
		MAC:       mac,
		State:     state,
		StartedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

var mac1 = net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01}
var mac2 = net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x02}

func TestMemorySessionStoreCreateAndFindByMAC(t *testing.T) {
	store := NewMemorySessionStore()

	s := newSession("s1", mac1, domain.StateDiscover)
	if err := store.Create(s); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	found, err := store.FindByMAC(mac1)
	if err != nil {
		t.Fatalf("FindByMAC() error = %v", err)
	}
	if found.ID != "s1" {
		t.Errorf("FindByMAC().ID = %q, want %q", found.ID, "s1")
	}
}

func TestMemorySessionStoreFindByMACReturnsNewest(t *testing.T) {
	store := NewMemorySessionStore()

	store.Create(newSession("s1", mac1, domain.StateDone))
	store.Create(newSession("s2", mac1, domain.StateDiscover))

	found, err := store.FindByMAC(mac1)
	if err != nil {
		t.Fatalf("FindByMAC() error = %v", err)
	}
	if found.ID != "s2" {
		t.Errorf("FindByMAC() should return newest, got ID=%q", found.ID)
	}
}

func TestMemorySessionStoreFindByMACNotFound(t *testing.T) {
	store := NewMemorySessionStore()

	_, err := store.FindByMAC(mac1)
	if err == nil {
		t.Error("FindByMAC() should fail for unknown MAC")
	}
}

func TestMemorySessionStoreUpdate(t *testing.T) {
	store := NewMemorySessionStore()

	s := newSession("s1", mac1, domain.StateDiscover)
	store.Create(s)

	s.State = domain.StateOffer
	if err := store.Update(s); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	found, _ := store.FindByMAC(mac1)
	if found.State != domain.StateOffer {
		t.Errorf("State after update = %s, want offer", found.State)
	}
}

func TestMemorySessionStoreUpdateNotFound(t *testing.T) {
	store := NewMemorySessionStore()

	s := newSession("nonexistent", mac1, domain.StateDiscover)
	err := store.Update(s)
	if err == nil {
		t.Error("Update() should fail for nonexistent session")
	}
}

func TestMemorySessionStoreFindActive(t *testing.T) {
	store := NewMemorySessionStore()

	store.Create(newSession("s1", mac1, domain.StateDiscover))
	store.Create(newSession("s2", mac1, domain.StateDone))
	store.Create(newSession("s3", mac2, domain.StateOffer))
	store.Create(newSession("s4", mac2, domain.StateFailed))

	active, err := store.FindActive()
	if err != nil {
		t.Fatalf("FindActive() error = %v", err)
	}
	if len(active) != 2 {
		t.Errorf("FindActive() count = %d, want 2", len(active))
	}
}

func TestMemorySessionStoreFindActiveEmpty(t *testing.T) {
	store := NewMemorySessionStore()

	active, err := store.FindActive()
	if err != nil {
		t.Fatalf("FindActive() error = %v", err)
	}
	if len(active) != 0 {
		t.Errorf("FindActive() count = %d, want 0", len(active))
	}
}

func TestMemorySessionStoreHistory(t *testing.T) {
	store := NewMemorySessionStore()

	for i := 0; i < 5; i++ {
		store.Create(newSession(fmt.Sprintf("s%d", i), mac1, domain.StateDone))
	}
	store.Create(newSession("other", mac2, domain.StateDone))

	history, err := store.History(mac1, 3)
	if err != nil {
		t.Fatalf("History() error = %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("History() count = %d, want 3", len(history))
	}
	// Should be newest first.
	if history[0].ID != "s4" {
		t.Errorf("History()[0].ID = %q, want %q", history[0].ID, "s4")
	}
}

func TestMemorySessionStoreHistoryLimitExceedsCount(t *testing.T) {
	store := NewMemorySessionStore()

	store.Create(newSession("s1", mac1, domain.StateDone))
	store.Create(newSession("s2", mac1, domain.StateDone))

	history, err := store.History(mac1, 100)
	if err != nil {
		t.Fatalf("History() error = %v", err)
	}
	if len(history) != 2 {
		t.Errorf("History() count = %d, want 2", len(history))
	}
}

func TestMemorySessionStoreConcurrent(t *testing.T) {
	store := NewMemorySessionStore()

	// Pre-populate sessions so goroutines don't modify shared session objects.
	for i := 0; i < 50; i++ {
		mac := net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, byte(i)}
		store.Create(newSession(fmt.Sprintf("s%d", i), mac, domain.StateOffer))
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			mac := net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, byte(i)}
			store.FindByMAC(mac)
			store.FindActive()
			store.History(mac, 5)
		}(i)
	}
	wg.Wait()
}
