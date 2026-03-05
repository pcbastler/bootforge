package domain

import (
	"testing"
)

func TestParseMenuType(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    MenuType
		wantErr bool
	}{
		{"install lowercase", "install", MenuInstall, false},
		{"live lowercase", "live", MenuLive, false},
		{"tool lowercase", "tool", MenuTool, false},
		{"exit lowercase", "exit", MenuExit, false},
		{"chain lowercase", "chain", MenuChain, false},
		{"install uppercase", "INSTALL", MenuInstall, false},
		{"mixed case", "Install", MenuInstall, false},
		{"exit mixed", "Exit", MenuExit, false},
		{"empty string", "", 0, true},
		{"unknown type", "unknown", 0, true},
		{"numeric", "42", 0, true},
		{"whitespace", " ", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMenuType(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMenuType(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseMenuType(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestMenuTypeString(t *testing.T) {
	tests := []struct {
		mt   MenuType
		want string
	}{
		{MenuInstall, "install"},
		{MenuLive, "live"},
		{MenuTool, "tool"},
		{MenuExit, "exit"},
		{MenuChain, "chain"},
		{MenuType(99), "MenuType(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.mt.String(); got != tt.want {
				t.Errorf("MenuType.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMenuEntryValidate(t *testing.T) {
	tests := []struct {
		name    string
		entry   MenuEntry
		wantErr bool
	}{
		{
			name: "valid install entry",
			entry: MenuEntry{
				Name:  "ubuntu-install",
				Label: "Ubuntu 24.04",
				Type:  MenuInstall,
				Boot:  BootParams{Kernel: "vmlinuz", Initrd: "initrd"},
			},
			wantErr: false,
		},
		{
			name: "valid live entry",
			entry: MenuEntry{
				Name:  "ubuntu-live",
				Label: "Ubuntu Live",
				Type:  MenuLive,
				Boot:  BootParams{Kernel: "vmlinuz"},
			},
			wantErr: false,
		},
		{
			name: "valid tool with kernel",
			entry: MenuEntry{
				Name:  "memtest",
				Label: "Memtest86+",
				Type:  MenuTool,
				Boot:  BootParams{Kernel: "memtest"},
			},
			wantErr: false,
		},
		{
			name: "valid tool with binary",
			entry: MenuEntry{
				Name:  "memtest",
				Label: "Memtest86+",
				Type:  MenuTool,
				Boot:  BootParams{Binary: "memtest.bin"},
			},
			wantErr: false,
		},
		{
			name: "valid exit entry",
			entry: MenuEntry{
				Name:  "local-disk",
				Label: "Boot from local disk",
				Type:  MenuExit,
			},
			wantErr: false,
		},
		{
			name:    "empty name",
			entry:   MenuEntry{Label: "test", Type: MenuExit},
			wantErr: true,
		},
		{
			name:    "empty label",
			entry:   MenuEntry{Name: "test", Type: MenuExit},
			wantErr: true,
		},
		{
			name: "install without kernel",
			entry: MenuEntry{
				Name:  "bad",
				Label: "Bad",
				Type:  MenuInstall,
			},
			wantErr: true,
		},
		{
			name: "live without kernel",
			entry: MenuEntry{
				Name:  "bad",
				Label: "Bad",
				Type:  MenuLive,
			},
			wantErr: true,
		},
		{
			name: "tool without kernel or binary",
			entry: MenuEntry{
				Name:  "bad",
				Label: "Bad",
				Type:  MenuTool,
			},
			wantErr: true,
		},
		{
			name: "valid chain entry",
			entry: MenuEntry{
				Name:  "netboot-xyz",
				Label: "netboot.xyz",
				Type:  MenuChain,
				Boot:  BootParams{Chain: "https://boot.netboot.xyz"},
			},
			wantErr: false,
		},
		{
			name: "chain without URL",
			entry: MenuEntry{
				Name:  "bad-chain",
				Label: "Bad Chain",
				Type:  MenuChain,
			},
			wantErr: true,
		},
		{
			name: "exit with kernel is still valid",
			entry: MenuEntry{
				Name:  "exit-extra",
				Label: "Exit",
				Type:  MenuExit,
				Boot:  BootParams{Kernel: "whatever"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.entry.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("MenuEntry.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
