package server

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"bootforge/internal/domain"
)

// SessionManager manages boot sessions using a SessionStore.
type SessionManager struct {
	store domain.SessionStore
	idGen func() string
}

// NewSessionManager creates a new session manager.
func NewSessionManager(store domain.SessionStore) *SessionManager {
	return &SessionManager{
		store: store,
		idGen: defaultIDGen,
	}
}

// Start creates a new boot session for the given MAC address.
func (m *SessionManager) Start(mac net.HardwareAddr, clientName string, arch domain.ClientArch) (*domain.Session, error) {
	now := time.Now()
	session := &domain.Session{
		ID:         m.idGen(),
		MAC:        mac,
		ClientName: clientName,
		State:      domain.StateDiscover,
		Arch:       arch,
		StartedAt:  now,
		UpdatedAt:  now,
	}

	if err := m.store.Create(session); err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}
	return session, nil
}

// Transition moves a session to a new state.
func (m *SessionManager) Transition(session *domain.Session, state domain.BootState, detail string) error {
	if err := session.Transition(state, detail); err != nil {
		return err
	}
	if err := m.store.Update(session); err != nil {
		return fmt.Errorf("updating session: %w", err)
	}
	return nil
}

// FindByMAC returns the active session for a MAC address.
func (m *SessionManager) FindByMAC(mac net.HardwareAddr) (*domain.Session, error) {
	return m.store.FindByMAC(mac)
}

// Active returns all active (non-terminal) sessions.
func (m *SessionManager) Active() ([]*domain.Session, error) {
	return m.store.FindActive()
}

// History returns the boot history for a MAC address.
func (m *SessionManager) History(mac net.HardwareAddr, limit int) ([]*domain.Session, error) {
	return m.store.History(mac, limit)
}

var sessionCounter atomic.Int64

func defaultIDGen() string {
	n := sessionCounter.Add(1)
	return fmt.Sprintf("session-%d-%d", time.Now().UnixMilli(), n)
}
