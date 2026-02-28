package domain

import (
	"net"
	"testing"
	"time"
)

func TestParseArch(t *testing.T) {
	tests := []struct {
		name      string
		option93  uint16
		want      ClientArch
		wantErr   bool
	}{
		{"BIOS", 0x0000, ArchBIOS, false},
		{"UEFI x86", 0x0006, ArchUEFIx86, false},
		{"UEFI x64 type 7", 0x0007, ArchUEFIx64, false},
		{"UEFI x64 type 9", 0x0009, ArchUEFIx64, false},
		{"ARM64", 0x000B, ArchARM64, false},
		{"unknown 0x0001", 0x0001, 0, true},
		{"unknown 0x0005", 0x0005, 0, true},
		{"unknown 0xFFFF", 0xFFFF, 0, true},
		{"unknown 0x000A", 0x000A, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseArch(tt.option93)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseArch(0x%04X) error = %v, wantErr %v", tt.option93, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseArch(0x%04X) = %v, want %v", tt.option93, got, tt.want)
			}
		})
	}
}

func TestClientArchString(t *testing.T) {
	tests := []struct {
		arch ClientArch
		want string
	}{
		{ArchBIOS, "bios"},
		{ArchUEFIx64, "uefi-x64"},
		{ArchUEFIx86, "uefi-x86"},
		{ArchARM64, "arm64"},
		{ClientArch(99), "ClientArch(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.arch.String(); got != tt.want {
				t.Errorf("ClientArch.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBootStateString(t *testing.T) {
	tests := []struct {
		state BootState
		want  string
	}{
		{StateDiscover, "discover"},
		{StateOffer, "offer"},
		{StateTFTP, "tftp"},
		{StateIPXE, "ipxe"},
		{StateMenu, "menu"},
		{StateBoot, "boot"},
		{StateDone, "done"},
		{StateFailed, "failed"},
		{BootState(99), "BootState(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("BootState.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func newTestSession() *Session {
	return &Session{
		ID:        "test-001",
		MAC:       net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF},
		State:     StateDiscover,
		StartedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func TestSessionTransitionValidPaths(t *testing.T) {
	// Test the happy path: Discover → Offer → TFTP → IPXE → Menu → Boot → Done
	t.Run("full boot path with menu", func(t *testing.T) {
		s := newTestSession()
		steps := []BootState{StateOffer, StateTFTP, StateIPXE, StateMenu, StateBoot, StateDone}
		for _, next := range steps {
			if err := s.Transition(next, "test"); err != nil {
				t.Fatalf("Transition to %s failed: %v", next, err)
			}
			if s.State != next {
				t.Fatalf("State = %s, want %s", s.State, next)
			}
		}
		if len(s.Events) != len(steps) {
			t.Errorf("Events count = %d, want %d", len(s.Events), len(steps))
		}
	})

	// Direct boot path (single entry, skips menu): Discover → Offer → TFTP → IPXE → Boot → Done
	t.Run("direct boot without menu", func(t *testing.T) {
		s := newTestSession()
		steps := []BootState{StateOffer, StateTFTP, StateIPXE, StateBoot, StateDone}
		for _, next := range steps {
			if err := s.Transition(next, "test"); err != nil {
				t.Fatalf("Transition to %s failed: %v", next, err)
			}
		}
	})

	// Failure from any non-terminal state
	t.Run("fail from discover", func(t *testing.T) {
		s := newTestSession()
		if err := s.Transition(StateFailed, "dhcp timeout"); err != nil {
			t.Fatalf("Transition to failed: %v", err)
		}
		if s.Error != "dhcp timeout" {
			t.Errorf("Error = %q, want %q", s.Error, "dhcp timeout")
		}
	})

	t.Run("fail from offer", func(t *testing.T) {
		s := newTestSession()
		s.Transition(StateOffer, "")
		if err := s.Transition(StateFailed, "tftp timeout"); err != nil {
			t.Fatalf("Transition to failed: %v", err)
		}
	})

	t.Run("fail from boot", func(t *testing.T) {
		s := newTestSession()
		s.Transition(StateOffer, "")
		s.Transition(StateTFTP, "")
		s.Transition(StateIPXE, "")
		s.Transition(StateBoot, "")
		if err := s.Transition(StateFailed, "kernel panic"); err != nil {
			t.Fatalf("Transition to failed: %v", err)
		}
	})
}

func TestSessionTransitionInvalid(t *testing.T) {
	tests := []struct {
		name      string
		fromState BootState
		toState   BootState
	}{
		{"discover to tftp (skip offer)", StateDiscover, StateTFTP},
		{"discover to done", StateDiscover, StateDone},
		{"discover to boot", StateDiscover, StateBoot},
		{"offer to ipxe (skip tftp)", StateOffer, StateIPXE},
		{"offer to done", StateOffer, StateDone},
		{"tftp to menu (skip ipxe)", StateTFTP, StateMenu},
		{"menu to done (skip boot)", StateMenu, StateDone},
		{"boot to offer (backwards)", StateBoot, StateOffer},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Session{ID: "test", State: tt.fromState}
			err := s.Transition(tt.toState, "test")
			if err == nil {
				t.Errorf("Transition from %s to %s should fail", tt.fromState, tt.toState)
			}
		})
	}
}

func TestSessionTransitionFromTerminalState(t *testing.T) {
	t.Run("from done", func(t *testing.T) {
		s := &Session{ID: "test", State: StateDone}
		if err := s.Transition(StateDiscover, "retry"); err == nil {
			t.Error("Transition from Done should fail")
		}
	})

	t.Run("from failed", func(t *testing.T) {
		s := &Session{ID: "test", State: StateFailed}
		if err := s.Transition(StateDiscover, "retry"); err == nil {
			t.Error("Transition from Failed should fail")
		}
	})

	t.Run("failed to failed", func(t *testing.T) {
		s := &Session{ID: "test", State: StateFailed}
		if err := s.Transition(StateFailed, "double fail"); err == nil {
			t.Error("Transition from Failed to Failed should fail")
		}
	})
}

func TestSessionTransitionTimestamps(t *testing.T) {
	s := newTestSession()
	before := time.Now()
	s.Transition(StateOffer, "test")
	after := time.Now()

	if s.UpdatedAt.Before(before) || s.UpdatedAt.After(after) {
		t.Errorf("UpdatedAt not in expected range")
	}
	if len(s.Events) != 1 {
		t.Fatalf("Events count = %d, want 1", len(s.Events))
	}
	ev := s.Events[0]
	if ev.State != StateOffer {
		t.Errorf("Event.State = %s, want offer", ev.State)
	}
	if ev.Detail != "test" {
		t.Errorf("Event.Detail = %q, want %q", ev.Detail, "test")
	}
}
