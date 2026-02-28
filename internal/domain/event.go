package domain

import (
	"fmt"
	"net"
	"time"
)

// EventType classifies system events.
type EventType int

const (
	EventBoot   EventType = iota // Boot session state change
	EventDHCP                    // DHCP request/response
	EventTFTP                    // TFTP file transfer
	EventHTTP                    // HTTP request
	EventHealth                  // Health check result
	EventConfig                  // Configuration change
)

var eventTypeNames = map[EventType]string{
	EventBoot:   "boot",
	EventDHCP:   "dhcp",
	EventTFTP:   "tftp",
	EventHTTP:   "http",
	EventHealth: "health",
	EventConfig: "config",
}

func (t EventType) String() string {
	if s, ok := eventTypeNames[t]; ok {
		return s
	}
	return fmt.Sprintf("EventType(%d)", int(t))
}

// Event is a system-wide event published through the EventBus.
type Event struct {
	Type    EventType
	At      time.Time
	MAC     net.HardwareAddr // optional, nil for non-client events
	Message string
	Data    map[string]any
}
