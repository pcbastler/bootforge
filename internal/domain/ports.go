package domain

import (
	"context"
	"net"
)

// ClientRepository provides access to client configurations.
type ClientRepository interface {
	FindByMAC(mac net.HardwareAddr) (*Client, error)
	FindDefault() (*Client, error) // wildcard mac="*"
	ListAll() ([]*Client, error)
	Reload() error
}

// MenuRepository provides access to menu entry definitions.
type MenuRepository interface {
	FindByName(name string) (*MenuEntry, error)
	ListAll() ([]*MenuEntry, error)
	Reload() error
}

// SessionStore persists boot session state.
type SessionStore interface {
	Create(session *Session) error
	Update(session *Session) error
	FindByMAC(mac net.HardwareAddr) (*Session, error)
	FindActive() ([]*Session, error)
	History(mac net.HardwareAddr, limit int) ([]*Session, error)
}

// EventBus provides in-process publish/subscribe for system events.
type EventBus interface {
	Publish(ctx context.Context, event Event)
	Subscribe(ctx context.Context) <-chan Event
}
