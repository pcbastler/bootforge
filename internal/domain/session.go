package domain

import (
	"fmt"
	"net"
	"time"
)

// ClientArch represents the client system architecture detected via DHCP Option 93.
type ClientArch int

const (
	ArchBIOS    ClientArch = iota // x86 BIOS (Option 93 = 0x0000)
	ArchUEFIx64                   // x86-64 UEFI (Option 93 = 0x0007, 0x0009)
	ArchUEFIx86                   // x86 UEFI (Option 93 = 0x0006)
	ArchARM64                     // ARM64 UEFI (Option 93 = 0x000B)
)

var archNames = map[ClientArch]string{
	ArchBIOS:    "bios",
	ArchUEFIx64: "uefi-x64",
	ArchUEFIx86: "uefi-x86",
	ArchARM64:   "arm64",
}

func (a ClientArch) String() string {
	if s, ok := archNames[a]; ok {
		return s
	}
	return fmt.Sprintf("ClientArch(%d)", int(a))
}

// ParseArch maps DHCP Option 93 (Client System Architecture) to ClientArch.
func ParseArch(option93 uint16) (ClientArch, error) {
	switch option93 {
	case 0x0000:
		return ArchBIOS, nil
	case 0x0006:
		return ArchUEFIx86, nil
	case 0x0007, 0x0009:
		return ArchUEFIx64, nil
	case 0x000B:
		return ArchARM64, nil
	default:
		return 0, fmt.Errorf("unknown client architecture: option 93 = 0x%04X", option93)
	}
}

// BootState represents a stage in the PXE boot process.
type BootState int

const (
	StateDiscover BootState = iota // DHCP Discover received
	StateOffer                     // DHCP Offer/Ack sent
	StateTFTP                      // TFTP bootloader transfer
	StateIPXE                      // iPXE script requested via HTTP
	StateMenu                      // Menu displayed to user
	StateBoot                      // OS boot started
	StateDone                      // Boot completed successfully
	StateFailed                    // Boot failed at some stage
)

var stateNames = map[BootState]string{
	StateDiscover: "discover",
	StateOffer:    "offer",
	StateTFTP:     "tftp",
	StateIPXE:     "ipxe",
	StateMenu:     "menu",
	StateBoot:     "boot",
	StateDone:     "done",
	StateFailed:   "failed",
}

func (s BootState) String() string {
	if n, ok := stateNames[s]; ok {
		return n
	}
	return fmt.Sprintf("BootState(%d)", int(s))
}

// validTransitions defines allowed state transitions.
// From any non-terminal state, transition to StateFailed is always allowed.
var validTransitions = map[BootState][]BootState{
	StateDiscover: {StateOffer, StateFailed},
	StateOffer:    {StateTFTP, StateFailed},
	StateTFTP:     {StateIPXE, StateFailed},
	StateIPXE:     {StateMenu, StateBoot, StateFailed}, // direct boot (single entry) skips menu
	StateMenu:     {StateBoot, StateFailed},
	StateBoot:     {StateDone, StateFailed},
	// StateDone and StateFailed are terminal states
}

// SessionEvent records a state transition with timestamp and detail.
type SessionEvent struct {
	State  BootState
	At     time.Time
	Detail string
}

// Session tracks a single PXE boot attempt for a client.
type Session struct {
	ID         string
	MAC        net.HardwareAddr
	ClientName string
	State      BootState
	Arch       ClientArch
	StartedAt  time.Time
	UpdatedAt  time.Time
	Events     []SessionEvent
	Error      string
}

// Transition moves the session to a new state, validating the transition.
func (s *Session) Transition(state BootState, detail string) error {
	allowed, ok := validTransitions[s.State]
	if !ok {
		return fmt.Errorf("session %s: no transitions from terminal state %s", s.ID, s.State)
	}

	valid := false
	for _, a := range allowed {
		if a == state {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("session %s: invalid transition from %s to %s", s.ID, s.State, state)
	}

	now := time.Now()
	s.State = state
	s.UpdatedAt = now
	s.Events = append(s.Events, SessionEvent{
		State:  state,
		At:     now,
		Detail: detail,
	})

	if state == StateFailed {
		s.Error = detail
	}

	return nil
}
