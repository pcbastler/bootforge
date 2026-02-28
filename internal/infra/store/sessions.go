// Package store implements persistence for sessions and log entries.
package store

import (
	"fmt"
	"net"
	"sync"

	"bootforge/internal/domain"
)

// MemorySessionStore is an in-memory implementation of domain.SessionStore.
type MemorySessionStore struct {
	mu       sync.RWMutex
	sessions []*domain.Session
}

// NewMemorySessionStore creates a new in-memory session store.
func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{}
}

// Create adds a new session to the store.
func (s *MemorySessionStore) Create(session *domain.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions = append(s.sessions, session)
	return nil
}

// Update replaces an existing session in the store.
func (s *MemorySessionStore) Update(session *domain.Session) error {
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

// FindByMAC returns the most recent session for the given MAC address.
func (s *MemorySessionStore) FindByMAC(mac net.HardwareAddr) (*domain.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := mac.String()
	for i := len(s.sessions) - 1; i >= 0; i-- {
		if s.sessions[i].MAC.String() == key {
			return s.sessions[i], nil
		}
	}
	return nil, fmt.Errorf("no session for MAC %s", mac)
}

// FindActive returns all sessions in non-terminal states.
func (s *MemorySessionStore) FindActive() ([]*domain.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var active []*domain.Session
	for _, sess := range s.sessions {
		if sess.State != domain.StateDone && sess.State != domain.StateFailed {
			active = append(active, sess)
		}
	}
	return active, nil
}

// History returns the most recent sessions for the given MAC address,
// up to the specified limit. Results are ordered newest first.
func (s *MemorySessionStore) History(mac net.HardwareAddr, limit int) ([]*domain.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := mac.String()
	var result []*domain.Session
	for i := len(s.sessions) - 1; i >= 0 && len(result) < limit; i-- {
		if s.sessions[i].MAC.String() == key {
			result = append(result, s.sessions[i])
		}
	}
	return result, nil
}
