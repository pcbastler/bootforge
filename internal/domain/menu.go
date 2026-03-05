// Package domain defines the core types and interfaces of Bootforge.
// It has zero external dependencies — only stdlib.
package domain

import (
	"fmt"
	"strings"
)

// MenuType classifies what a menu entry does.
type MenuType int

const (
	MenuInstall MenuType = iota // Full OS installation
	MenuLive                    // RAM-based live system
	MenuTool                    // Diagnostic / utility tool
	MenuExit                    // Return to local disk boot
	MenuChain                   // Chainload external iPXE endpoint
)

var menuTypeNames = map[MenuType]string{
	MenuInstall: "install",
	MenuLive:    "live",
	MenuTool:    "tool",
	MenuExit:    "exit",
	MenuChain:   "chain",
}

var menuTypeValues = map[string]MenuType{
	"install": MenuInstall,
	"live":    MenuLive,
	"tool":    MenuTool,
	"exit":    MenuExit,
	"chain":   MenuChain,
}

func (t MenuType) String() string {
	if s, ok := menuTypeNames[t]; ok {
		return s
	}
	return fmt.Sprintf("MenuType(%d)", int(t))
}

// ParseMenuType converts a string to MenuType. Case-insensitive.
func ParseMenuType(s string) (MenuType, error) {
	if t, ok := menuTypeValues[strings.ToLower(s)]; ok {
		return t, nil
	}
	return 0, fmt.Errorf("unknown menu type %q", s)
}

// MenuHTTP defines where boot files are served from.
type MenuHTTP struct {
	Files    string // local directory containing boot files
	Path     string // URL path prefix, e.g. "/installers/ubuntu/"
	Upstream string // optional upstream URL for caching proxy
}

// BootParams defines how iPXE boots this entry.
type BootParams struct {
	Kernel  string   // kernel filename (vmlinuz, etc.)
	Initrd  string   // initrd filename
	Cmdline string   // kernel command line
	Loader  string   // special loader (e.g. "wimboot" for Windows)
	Files   []string // additional files for wimboot
	Binary  string   // single binary to boot (e.g. memtest)
	Image   string   // ISO/disk image
	Chain   string   // URL to chainload (e.g. netboot.xyz)
}

// MenuEntry is a global boot option (e.g. "ubuntu-install", "rescue", "local-disk").
type MenuEntry struct {
	Name        string     // unique identifier, e.g. "ubuntu-install"
	Label       string     // display text for iPXE menu
	Description string     // optional longer description
	Type        MenuType   // install, live, tool, exit
	HTTP        MenuHTTP   // file serving config
	Boot        BootParams // boot parameters
	SourceFile  string     // which .toml file defined this entry
}

// Validate checks that the menu entry is well-formed.
func (e *MenuEntry) Validate() error {
	if e.Name == "" {
		return fmt.Errorf("menu entry: name is required")
	}
	if e.Label == "" {
		return fmt.Errorf("menu entry %q: label is required", e.Name)
	}

	switch e.Type {
	case MenuInstall, MenuLive:
		if e.Boot.Kernel == "" {
			return fmt.Errorf("menu entry %q: kernel is required for type %s", e.Name, e.Type)
		}
	case MenuTool:
		if e.Boot.Kernel == "" && e.Boot.Binary == "" {
			return fmt.Errorf("menu entry %q: kernel or binary is required for type %s", e.Name, e.Type)
		}
	case MenuExit:
		// exit entries need no boot params
	case MenuChain:
		if e.Boot.Chain == "" {
			return fmt.Errorf("menu entry %q: chain URL is required for type %s", e.Name, e.Type)
		}
	default:
		return fmt.Errorf("menu entry %q: unknown type %d", e.Name, int(e.Type))
	}

	return nil
}
