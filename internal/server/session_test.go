package server

import (
	"fmt"
	"net"
	"sync"
	"testing"

	"bootforge/internal/domain"
)

// testSessionStore is a simple in-memory implementation for testing.
type testSessionStore struct {
	mu       sync.Mutex
	sessions []*domain.Session
}

func newTestSessionStore() *testSessionStore {
	return &testSessionStore{}
}

func (s *testSessionStore) Create(session *domain.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions = append(s.sessions, session)
	return nil
}

func (s *testSessionStore) Update(session *domain.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, existing := range s.sessions {
		if existing.ID == session.ID {
			s.sessions[i] = session
			return nil
		}
	}
	return fmt.Errorf("session %s not found", session.ID)
}

func (s *testSessionStore) FindByMAC(mac net.HardwareAddr) (*domain.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := mac.String()
	for i := len(s.sessions) - 1; i >= 0; i-- {
		if s.sessions[i].MAC.String() == key {
			return s.sessions[i], nil
		}
	}
	return nil, fmt.Errorf("no session for MAC %s", mac)
}

func (s *testSessionStore) FindActive() ([]*domain.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var active []*domain.Session
	for _, sess := range s.sessions {
		if sess.State != domain.StateDone && sess.State != domain.StateFailed {
			active = append(active, sess)
		}
	}
	return active, nil
}

func (s *testSessionStore) History(mac net.HardwareAddr, limit int) ([]*domain.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := mac.String()
	var result []*domain.Session
	for i := len(s.sessions) - 1; i >= 0 && len(result) < limit; i-- {
		if s.sessions[i].MAC.String() == key {
			result = append(result, s.sessions[i])
		}
	}
	return result, nil
}

func TestSessionManagerStart(t *testing.T) {
	store := newTestSessionStore()
	mgr := NewSessionManager(store)

	mac := net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01}
	session, err := mgr.Start(mac, "ws-01", domain.ArchUEFIx64)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if session.MAC.String() != mac.String() {
		t.Errorf("session.MAC = %s, want %s", session.MAC, mac)
	}
	if session.State != domain.StateDiscover {
		t.Errorf("session.State = %s, want discover", session.State)
	}
	if session.ClientName != "ws-01" {
		t.Errorf("session.ClientName = %q, want %q", session.ClientName, "ws-01")
	}
	if session.Arch != domain.ArchUEFIx64 {
		t.Errorf("session.Arch = %v, want uefi-x64", session.Arch)
	}
	if session.ID == "" {
		t.Error("session.ID should not be empty")
	}
}

func TestSessionManagerTransition(t *testing.T) {
	store := newTestSessionStore()
	mgr := NewSessionManager(store)

	mac := net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01}
	session, _ := mgr.Start(mac, "ws-01", domain.ArchBIOS)

	if err := mgr.Transition(session, domain.StateOffer, "dhcp offer sent"); err != nil {
		t.Fatalf("Transition() error = %v", err)
	}
	if session.State != domain.StateOffer {
		t.Errorf("session.State = %s, want offer", session.State)
	}
}

func TestSessionManagerTransitionInvalid(t *testing.T) {
	store := newTestSessionStore()
	mgr := NewSessionManager(store)

	mac := net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01}
	session, _ := mgr.Start(mac, "ws-01", domain.ArchBIOS)

	// Skip offer → tftp directly should fail.
	err := mgr.Transition(session, domain.StateTFTP, "skip")
	if err == nil {
		t.Error("Transition() should fail for invalid state transition")
	}
}

func TestSessionManagerFindByMAC(t *testing.T) {
	store := newTestSessionStore()
	mgr := NewSessionManager(store)

	mac := net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01}
	mgr.Start(mac, "ws-01", domain.ArchBIOS)

	found, err := mgr.FindByMAC(mac)
	if err != nil {
		t.Fatalf("FindByMAC() error = %v", err)
	}
	if found.ClientName != "ws-01" {
		t.Errorf("FindByMAC().ClientName = %q, want %q", found.ClientName, "ws-01")
	}
}

func TestSessionManagerActive(t *testing.T) {
	store := newTestSessionStore()
	mgr := NewSessionManager(store)

	mac1 := net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01}
	mac2 := net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x02}

	s1, _ := mgr.Start(mac1, "ws-01", domain.ArchBIOS)
	mgr.Start(mac2, "ws-02", domain.ArchUEFIx64)

	// Complete session 1.
	mgr.Transition(s1, domain.StateOffer, "")
	mgr.Transition(s1, domain.StateTFTP, "")
	mgr.Transition(s1, domain.StateIPXE, "")
	mgr.Transition(s1, domain.StateBoot, "")
	mgr.Transition(s1, domain.StateDone, "")

	active, err := mgr.Active()
	if err != nil {
		t.Fatalf("Active() error = %v", err)
	}
	if len(active) != 1 {
		t.Errorf("Active() count = %d, want 1", len(active))
	}
}

func TestSessionManagerHistory(t *testing.T) {
	store := newTestSessionStore()
	mgr := NewSessionManager(store)

	mac := net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01}

	// Create 3 sessions for the same MAC.
	for i := 0; i < 3; i++ {
		mgr.Start(mac, "ws-01", domain.ArchBIOS)
	}

	// Get last 2.
	history, err := mgr.History(mac, 2)
	if err != nil {
		t.Fatalf("History() error = %v", err)
	}
	if len(history) != 2 {
		t.Errorf("History() count = %d, want 2", len(history))
	}
}

func TestSessionManagerConcurrent(t *testing.T) {
	store := newTestSessionStore()
	mgr := NewSessionManager(store)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			mac := net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, byte(i)}
			s, err := mgr.Start(mac, fmt.Sprintf("ws-%02d", i), domain.ArchBIOS)
			if err != nil {
				return
			}
			mgr.Transition(s, domain.StateOffer, "")
		}(i)
	}
	wg.Wait()
}
