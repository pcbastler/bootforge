package server

import (
	"fmt"
	"strings"

	"bootforge/internal/domain"
)

// IPXEVars holds runtime variables for iPXE script generation.
type IPXEVars struct {
	ServerIP string
	HTTPPort int
	MAC      string
	Custom   map[string]string
}

// IPXEGenerator creates iPXE boot scripts from menu entries.
type IPXEGenerator struct{}

// NewIPXEGenerator creates a new generator.
func NewIPXEGenerator() *IPXEGenerator {
	return &IPXEGenerator{}
}

// Generate creates an iPXE script for the given menu entries and configuration.
// If a single non-exit entry is provided, it generates a direct boot script.
// Otherwise, it generates a full menu with timeout and default selection.
func (g *IPXEGenerator) Generate(entries []*domain.MenuEntry, mc domain.MenuConfig, vars IPXEVars) string {
	var b strings.Builder
	b.WriteString("#!ipxe\n\n")

	// Single entry: direct boot (no menu display).
	if len(entries) == 1 && entries[0].Type != domain.MenuExit {
		g.writeBootEntry(&b, entries[0], vars)
		return b.String()
	}

	// Multiple entries: full menu.
	g.writeMenu(&b, entries, mc, vars)
	return b.String()
}

// GenerateOverride creates a direct-boot script for a one-time boot override.
func (g *IPXEGenerator) GenerateOverride(entry *domain.MenuEntry, vars IPXEVars) string {
	var b strings.Builder
	b.WriteString("#!ipxe\n\n")
	b.WriteString("# One-time boot override\n")
	g.writeBootEntry(&b, entry, vars)
	return b.String()
}

func (g *IPXEGenerator) writeMenu(b *strings.Builder, entries []*domain.MenuEntry, mc domain.MenuConfig, vars IPXEVars) {
	b.WriteString("menu Bootforge Boot Menu\n")

	// Timeout.
	if mc.Timeout > 0 {
		b.WriteString(fmt.Sprintf("timeout %d000\n", mc.Timeout))
	}

	// Default.
	if mc.Default != "" {
		b.WriteString(fmt.Sprintf("default %s\n", mc.Default))
	}

	b.WriteString("\n")

	// Menu items.
	for _, entry := range entries {
		b.WriteString(fmt.Sprintf("item %s %s\n", entry.Name, entry.Label))
	}

	b.WriteString("\nchoose selected || goto failed\n")

	// Entry labels.
	for _, entry := range entries {
		b.WriteString(fmt.Sprintf("\n:%s\n", entry.Name))
		if entry.Type == domain.MenuExit {
			b.WriteString("exit 0\n")
		} else {
			g.writeBootEntry(b, entry, vars)
		}
	}

	// Failure handler.
	b.WriteString("\n:failed\n")
	b.WriteString("echo Boot failed, retrying in 5 seconds...\n")
	b.WriteString("sleep 5\n")
	b.WriteString("goto start\n")
}

func (g *IPXEGenerator) writeBootEntry(b *strings.Builder, entry *domain.MenuEntry, vars IPXEVars) {
	// Special loader (e.g. wimboot for Windows).
	if entry.Boot.Loader == "wimboot" {
		g.writeWimboot(b, entry, vars)
		return
	}

	// Binary boot (e.g. memtest).
	if entry.Boot.Binary != "" {
		url := g.fileURL(entry, entry.Boot.Binary, vars)
		b.WriteString(fmt.Sprintf("chain %s\n", url))
		return
	}

	// Standard kernel boot.
	if entry.Boot.Kernel != "" {
		url := g.fileURL(entry, entry.Boot.Kernel, vars)
		b.WriteString(fmt.Sprintf("kernel %s", url))
		if entry.Boot.Cmdline != "" {
			cmdline := g.substituteVars(entry.Boot.Cmdline, vars)
			b.WriteString(fmt.Sprintf(" %s", cmdline))
		}
		b.WriteString("\n")
	}

	if entry.Boot.Initrd != "" {
		url := g.fileURL(entry, entry.Boot.Initrd, vars)
		b.WriteString(fmt.Sprintf("initrd %s\n", url))
	}

	if entry.Boot.Image != "" {
		url := g.fileURL(entry, entry.Boot.Image, vars)
		b.WriteString(fmt.Sprintf("initrd %s\n", url))
	}

	b.WriteString("boot\n")
}

func (g *IPXEGenerator) writeWimboot(b *strings.Builder, entry *domain.MenuEntry, vars IPXEVars) {
	url := g.fileURL(entry, "wimboot", vars)
	b.WriteString(fmt.Sprintf("kernel %s\n", url))

	for _, file := range entry.Boot.Files {
		furl := g.fileURL(entry, file, vars)
		b.WriteString(fmt.Sprintf("initrd %s\n", furl))
	}

	b.WriteString("boot\n")
}

func (g *IPXEGenerator) fileURL(entry *domain.MenuEntry, filename string, vars IPXEVars) string {
	if entry.HTTP.Path != "" {
		path := entry.HTTP.Path
		if !strings.HasSuffix(path, "/") {
			path += "/"
		}
		url := fmt.Sprintf("http://%s:%d%s%s", vars.ServerIP, vars.HTTPPort, path, filename)
		return g.substituteVars(url, vars)
	}
	// Fallback: use the menu entry name as path prefix.
	url := fmt.Sprintf("http://%s:%d/%s/%s", vars.ServerIP, vars.HTTPPort, entry.Name, filename)
	return g.substituteVars(url, vars)
}

func (g *IPXEGenerator) substituteVars(s string, vars IPXEVars) string {
	s = strings.ReplaceAll(s, "${server_ip}", vars.ServerIP)
	s = strings.ReplaceAll(s, "${http_port}", fmt.Sprintf("%d", vars.HTTPPort))
	s = strings.ReplaceAll(s, "${mac}", vars.MAC)

	for k, v := range vars.Custom {
		s = strings.ReplaceAll(s, "${"+k+"}", v)
	}
	return s
}
