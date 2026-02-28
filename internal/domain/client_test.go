package domain

import (
	"net"
	"testing"
)

func TestParseMAC(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string // expected MAC string or "wildcard"
		wantErr bool
	}{
		{"colon lowercase", "aa:bb:cc:dd:ee:ff", "aa:bb:cc:dd:ee:ff", false},
		{"colon uppercase", "AA:BB:CC:DD:EE:FF", "aa:bb:cc:dd:ee:ff", false},
		{"colon mixed case", "Aa:Bb:Cc:Dd:Ee:Ff", "aa:bb:cc:dd:ee:ff", false},
		{"dash separator", "aa-bb-cc-dd-ee-ff", "aa:bb:cc:dd:ee:ff", false},
		{"wildcard", "*", "wildcard", false},
		{"empty string", "", "", true},
		{"whitespace only", "  ", "", true},
		{"wildcard with spaces", " * ", "wildcard", false},
		{"too short", "aa:bb:cc", "", true},
		{"invalid hex", "gg:hh:ii:jj:kk:ll", "", true},
		{"no separators", "aabbccddeeff", "", true},
		{"partial", "aa:bb:", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMAC(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMAC(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if tt.want == "wildcard" {
				if got.String() != WildcardMAC.String() {
					t.Errorf("ParseMAC(%q) = %v, want WildcardMAC", tt.input, got)
				}
			} else if got.String() != tt.want {
				t.Errorf("ParseMAC(%q) = %v, want %v", tt.input, got.String(), tt.want)
			}
		})
	}
}

func TestClientIsWildcard(t *testing.T) {
	tests := []struct {
		name string
		mac  net.HardwareAddr
		want bool
	}{
		{"wildcard MAC", WildcardMAC, true},
		{"regular MAC", net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}, false},
		{"nil MAC", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{MAC: tt.mac}
			if got := c.IsWildcard(); got != tt.want {
				t.Errorf("Client.IsWildcard() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClientValidate(t *testing.T) {
	validMAC := net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}

	tests := []struct {
		name    string
		client  Client
		wantErr bool
	}{
		{
			name: "valid client",
			client: Client{
				MAC:  validMAC,
				Name: "workstation-01",
				Menu: MenuConfig{
					Entries: []string{"ubuntu-install", "rescue"},
					Default: "ubuntu-install",
					Timeout: 30,
				},
				Enabled: true,
			},
			wantErr: false,
		},
		{
			name: "valid wildcard client",
			client: Client{
				MAC:  WildcardMAC,
				Name: "default",
				Menu: MenuConfig{
					Entries: []string{"rescue"},
					Default: "rescue",
					Timeout: 30,
				},
				Enabled: true,
			},
			wantErr: false,
		},
		{
			name: "nil MAC",
			client: Client{
				Name: "bad",
				Menu: MenuConfig{Entries: []string{"rescue"}},
			},
			wantErr: true,
		},
		{
			name: "empty name",
			client: Client{
				MAC:  validMAC,
				Menu: MenuConfig{Entries: []string{"rescue"}},
			},
			wantErr: true,
		},
		{
			name: "empty entries",
			client: Client{
				MAC:  validMAC,
				Name: "bad",
				Menu: MenuConfig{Entries: []string{}},
			},
			wantErr: true,
		},
		{
			name: "default not in entries",
			client: Client{
				MAC:  validMAC,
				Name: "bad",
				Menu: MenuConfig{
					Entries: []string{"rescue"},
					Default: "nonexistent",
				},
			},
			wantErr: true,
		},
		{
			name: "negative timeout",
			client: Client{
				MAC:  validMAC,
				Name: "bad",
				Menu: MenuConfig{
					Entries: []string{"rescue"},
					Timeout: -1,
				},
			},
			wantErr: true,
		},
		{
			name: "zero timeout is valid",
			client: Client{
				MAC:  validMAC,
				Name: "ok",
				Menu: MenuConfig{
					Entries: []string{"rescue"},
					Timeout: 0,
				},
			},
			wantErr: false,
		},
		{
			name: "empty default is valid with single entry",
			client: Client{
				MAC:  validMAC,
				Name: "ok",
				Menu: MenuConfig{
					Entries: []string{"rescue"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.client.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMenuConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		mc      MenuConfig
		wantErr bool
	}{
		{
			name:    "valid single entry",
			mc:      MenuConfig{Entries: []string{"rescue"}},
			wantErr: false,
		},
		{
			name:    "valid multiple entries with default",
			mc:      MenuConfig{Entries: []string{"a", "b", "c"}, Default: "b", Timeout: 10},
			wantErr: false,
		},
		{
			name:    "empty entries",
			mc:      MenuConfig{Entries: []string{}},
			wantErr: true,
		},
		{
			name:    "nil entries",
			mc:      MenuConfig{},
			wantErr: true,
		},
		{
			name:    "default not in entries",
			mc:      MenuConfig{Entries: []string{"a", "b"}, Default: "c"},
			wantErr: true,
		},
		{
			name:    "negative timeout",
			mc:      MenuConfig{Entries: []string{"a"}, Timeout: -5},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mc.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("MenuConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
