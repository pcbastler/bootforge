package wizard

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"bootforge/internal/domain"
	"bootforge/internal/infra/toml"
)

// Run executes the full interactive setup wizard.
// It returns the final FullConfig or an error if the user cancelled.
func Run(configDir string) (*domain.FullConfig, error) {
	state := DefaultState()
	state.ConfigDir = configDir

	steps := []struct {
		name string
		fn   func(*WizardState) error
	}{
		{"Network Interface", stepInterface},
		{"Data Directory", stepDataDir},
		{"iPXE Bootloader", stepBootloader},
		{"Services", stepServices},
		{"netboot.xyz", stepNetboot},
		{"Boot Menus", stepMenus},
		{"Clients", stepClients},
		{"Summary", stepSummary},
	}

	fmt.Println()
	fmt.Println("  Bootforge Setup Wizard")
	fmt.Println("  ─────────────────────")
	fmt.Println()

	for i, step := range steps {
		fmt.Printf("  Step %d/%d: %s\n\n", i+1, len(steps), step.name)
		if err := step.fn(state); err != nil {
			return nil, fmt.Errorf("step %q: %w", step.name, err)
		}
		fmt.Println()
	}

	cfg := StateToConfig(state)

	// Write config.
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("creating config dir: %w", err)
	}
	if err := toml.WriteConfig(cfg, configDir); err != nil {
		return nil, err
	}

	// Download iPXE if requested.
	if state.DownloadIPXE && len(state.IPXEArchs) > 0 {
		destDir := filepath.Join(configDir, state.DataDir, state.BootloaderDir)
		archs := state.IPXEDownloadArchs()
		fmt.Println("  Downloading iPXE bootloader files...")
		err := DownloadIPXEFiles(archs, destDir, func(r DownloadResult) {
			if r.Err != nil {
				fmt.Printf("    FAIL  %s: %v\n", r.Arch.Filename, r.Err)
			} else {
				fmt.Printf("    OK    %s (%d bytes)\n", r.Arch.Filename, r.Size)
			}
		})
		if err != nil {
			fmt.Printf("\n  Warning: %v\n", err)
			fmt.Println("  You can download bootloader files manually later.")
		}
	}

	// Download netboot.xyz if requested.
	if state.NetbootMode == NetbootSelfHosted {
		destDir := filepath.Join(configDir, state.DataDir, state.NetbootDir)
		assets := NetbootAssets(state.NetbootBaseURL)
		fmt.Println("  Downloading netboot.xyz files...")
		err := DownloadNetbootFiles(assets, destDir, func(r DownloadResult) {
			if r.Err != nil {
				fmt.Printf("    FAIL  %s: %v\n", r.Arch.Filename, r.Err)
			} else {
				fmt.Printf("    OK    %s (%d bytes)\n", r.Arch.Filename, r.Size)
			}
		})
		if err != nil {
			fmt.Printf("\n  Warning: %v\n", err)
			fmt.Println("  You can download netboot.xyz files manually later.")
		}
	}

	// Create data directory.
	dataDir := filepath.Join(configDir, state.DataDir)
	os.MkdirAll(dataDir, 0755)

	fmt.Println()
	fmt.Println("  Configuration written to:", filepath.Join(configDir, "bootforge.toml"))
	fmt.Println()
	fmt.Println("  Next steps:")
	fmt.Println("    bootforge validate --config", configDir)
	fmt.Println("    bootforge serve --config", configDir)
	fmt.Println()

	return cfg, nil
}

// RunEdit runs the wizard with pre-filled values from an existing config.
func RunEdit(configDir string, existing *domain.FullConfig) (*domain.FullConfig, error) {
	state := ConfigToState(existing)
	state.ConfigDir = configDir

	steps := []struct {
		name string
		fn   func(*WizardState) error
	}{
		{"Network Interface", stepInterface},
		{"Data Directory", stepDataDir},
		{"iPXE Bootloader", stepBootloader},
		{"Services", stepServices},
		{"netboot.xyz", stepNetboot},
		{"Boot Menus", stepMenusEdit},
		{"Clients", stepClientsEdit},
		{"Summary", stepSummary},
	}

	fmt.Println()
	fmt.Println("  Bootforge Configuration Editor")
	fmt.Println("  ──────────────────────────────")
	fmt.Println()

	for i, step := range steps {
		fmt.Printf("  Step %d/%d: %s\n\n", i+1, len(steps), step.name)
		if err := step.fn(state); err != nil {
			return nil, fmt.Errorf("step %q: %w", step.name, err)
		}
		fmt.Println()
	}

	cfg := StateToConfig(state)

	// Backup existing config.
	src := filepath.Join(configDir, "bootforge.toml")
	if _, err := os.Stat(src); err == nil {
		bak := src + ".bak"
		data, _ := os.ReadFile(src)
		if data != nil {
			os.WriteFile(bak, data, 0644)
			fmt.Println("  Backup saved:", bak)
		}
	}

	if err := toml.WriteConfig(cfg, configDir); err != nil {
		return nil, err
	}

	fmt.Println("  Configuration updated:", src)
	fmt.Println()

	return cfg, nil
}

// --- Individual steps ---

func stepInterface(s *WizardState) error {
	ifaces, err := DetectInterfaces()
	if err != nil || len(ifaces) == 0 {
		// Fallback to manual input.
		return Input("Network interface", "eth0", &s.Interface, func(v string) error {
			if v == "" {
				return fmt.Errorf("interface name is required")
			}
			return nil
		})
	}

	// Build options from detected interfaces.
	opts := make([]Option[string], len(ifaces))
	for i, iface := range ifaces {
		opts[i] = Option[string]{Label: iface.String(), Value: iface.Name}
	}

	// Pre-select current value if editing.
	if s.Interface == "" && len(ifaces) > 0 {
		s.Interface = ifaces[0].Name
	}

	if err := Select("Network interface", opts, &s.Interface); err != nil {
		return err
	}

	// Remember the IP for later use.
	for _, iface := range ifaces {
		if iface.Name == s.Interface {
			s.ServerIP = iface.IP
			break
		}
	}

	return nil
}

func stepDataDir(s *WizardState) error {
	return Input("Data directory (boot files, etc.)", "./data", &s.DataDir, func(v string) error {
		if v == "" {
			return fmt.Errorf("data directory is required")
		}
		return nil
	})
}

func stepBootloader(s *WizardState) error {
	s.DownloadIPXE = true
	if err := Confirm("Download iPXE bootloader files?", &s.DownloadIPXE); err != nil {
		return err
	}

	if !s.DownloadIPXE {
		fmt.Println("  Skipping download. Place bootloader files manually in:")
		fmt.Printf("    %s/%s/%s/\n", s.ConfigDir, s.DataDir, s.BootloaderDir)
		return nil
	}

	// Select iPXE build variant.
	variantOpts := []Option[IPXEVariant]{
		{Label: "Full (recommended, better hardware support)", Value: IPXEVariantFull},
		{Label: "SNP Only (minimal, smaller download)", Value: IPXEVariantSNPOnly},
	}
	if err := Select("iPXE build variant", variantOpts, &s.IPXEVariant); err != nil {
		return err
	}

	// Select architectures.
	archOpts := []Option[string]{
		{Label: "UEFI x64", Value: "uefi_x64"},
		{Label: "UEFI x86", Value: "uefi_x86"},
		{Label: "BIOS (Legacy)", Value: "bios"},
		{Label: "ARM64", Value: "arm64"},
	}
	if len(s.IPXEArchs) == 0 {
		s.IPXEArchs = []string{"uefi_x64", "bios"}
	}
	if err := MultiSelect("Architectures to download", archOpts, &s.IPXEArchs); err != nil {
		return err
	}

	// Custom base URL.
	return Input("iPXE download base URL", DefaultIPXEBaseURL, &s.IPXEBaseURL, func(v string) error {
		if v == "" {
			return fmt.Errorf("URL is required")
		}
		return nil
	})
}

func stepServices(s *WizardState) error {
	// DHCP Proxy.
	if err := Confirm("Enable DHCP proxy?", &s.DHCPEnabled); err != nil {
		return err
	}
	if s.DHCPEnabled {
		if err := inputPort("DHCP port", &s.DHCPPort); err != nil {
			return err
		}
		if err := inputPort("DHCP proxy port (PXE option 43/60)", &s.DHCPProxyPort); err != nil {
			return err
		}
	}

	// TFTP.
	if err := Confirm("Enable TFTP server?", &s.TFTPEnabled); err != nil {
		return err
	}
	if s.TFTPEnabled {
		if err := inputPort("TFTP port", &s.TFTPPort); err != nil {
			return err
		}
	}

	// HTTP.
	if err := Confirm("Enable HTTP server?", &s.HTTPEnabled); err != nil {
		return err
	}
	if s.HTTPEnabled {
		if err := inputPort("HTTP port", &s.HTTPPort); err != nil {
			return err
		}
	}

	return nil
}

func stepNetboot(s *WizardState) error {
	modeOpts := []Option[NetbootMode]{
		{Label: "Remote (chain to boot.netboot.xyz, requires internet)", Value: NetbootRemote},
		{Label: "Self-hosted (download files, serve locally)", Value: NetbootSelfHosted},
		{Label: "Skip", Value: NetbootSkip},
	}
	if err := Select("Add netboot.xyz to your boot menu?", modeOpts, &s.NetbootMode); err != nil {
		return err
	}

	switch s.NetbootMode {
	case NetbootRemote:
		s.Menus = append(s.Menus, MenuState{
			Name:  "netboot-xyz",
			Label: "netboot.xyz",
			Type:  "chain",
			Chain: "https://boot.netboot.xyz",
		})
	case NetbootSelfHosted:
		if err := Input("netboot.xyz download URL", DefaultNetbootBaseURL, &s.NetbootBaseURL, func(v string) error {
			if v == "" {
				return fmt.Errorf("URL is required")
			}
			return nil
		}); err != nil {
			return err
		}
		s.Menus = append(s.Menus, MenuState{
			Name:      "netboot-xyz",
			Label:     "netboot.xyz (local)",
			Type:      "chain",
			Chain:     "http://${server_ip}:${http_port}/netboot/netboot.xyz.lkrn",
			HTTPPath:  "/netboot/",
			HTTPFiles: "netboot/",
		})
	}

	return nil
}

func stepMenus(s *WizardState) error {
	// local-disk is already in defaults. Ask about additional entries.
	fmt.Println("  The 'local-disk' (exit) menu is included by default.")

	for {
		add := false
		if err := Confirm("Add a boot menu entry?", &add); err != nil {
			return err
		}
		if !add {
			break
		}

		m, err := promptMenuEntry()
		if err != nil {
			return err
		}
		s.Menus = append(s.Menus, m)
	}

	return nil
}

func stepMenusEdit(s *WizardState) error {
	// Show existing menus.
	fmt.Println("  Current menus:")
	for _, m := range s.Menus {
		fmt.Printf("    - %s (%s): %s\n", m.Name, m.Type, m.Label)
	}
	fmt.Println()

	for {
		add := false
		if err := Confirm("Add another boot menu entry?", &add); err != nil {
			return err
		}
		if !add {
			break
		}

		m, err := promptMenuEntry()
		if err != nil {
			return err
		}
		s.Menus = append(s.Menus, m)
	}

	return nil
}

func promptMenuEntry() (MenuState, error) {
	var m MenuState

	if err := Input("Menu name (slug)", "ubuntu-install", &m.Name, func(v string) error {
		if v == "" {
			return fmt.Errorf("name is required")
		}
		if strings.Contains(v, " ") {
			return fmt.Errorf("name must not contain spaces")
		}
		return nil
	}); err != nil {
		return m, err
	}

	if err := Input("Menu label (display text)", "Ubuntu 24.04 Install", &m.Label, func(v string) error {
		if v == "" {
			return fmt.Errorf("label is required")
		}
		return nil
	}); err != nil {
		return m, err
	}

	typeOpts := []Option[string]{
		{Label: "Install (full OS installation)", Value: "install"},
		{Label: "Live (RAM-based live system)", Value: "live"},
		{Label: "Tool (diagnostic/utility)", Value: "tool"},
		{Label: "Chain (chainload external iPXE menu)", Value: "chain"},
		{Label: "Exit (boot from local disk)", Value: "exit"},
	}
	m.Type = "install"
	if err := Select("Menu type", typeOpts, &m.Type); err != nil {
		return m, err
	}

	if m.Type == "chain" {
		if err := Input("Chain URL", "https://boot.netboot.xyz", &m.Chain, func(v string) error {
			if v == "" {
				return fmt.Errorf("chain URL is required")
			}
			return nil
		}); err != nil {
			return m, err
		}
	} else if m.Type != "exit" {
		if err := Input("Kernel filename", "vmlinuz", &m.Kernel, nil); err != nil {
			return m, err
		}
		if err := Input("Initrd filename", "initrd", &m.Initrd, nil); err != nil {
			return m, err
		}
		if err := Input("Kernel command line (optional)", "", &m.Cmdline, nil); err != nil {
			return m, err
		}
		if err := Input("HTTP path prefix (e.g. /installers/ubuntu/)", "", &m.HTTPPath, nil); err != nil {
			return m, err
		}
	}

	return m, nil
}

func stepClients(s *WizardState) error {
	// Wildcard client is always included. Configure its menu entries.
	fmt.Println("  Default client (wildcard *) is always included.")

	menuNames := s.MenuNames()
	menuOpts := make([]Option[string], len(menuNames))
	for i, n := range menuNames {
		menuOpts[i] = Option[string]{Label: n, Value: n}
	}

	// Default client menu entries.
	if len(s.Clients) > 0 && s.Clients[len(s.Clients)-1].MAC == "*" {
		wildcard := &s.Clients[len(s.Clients)-1]
		if err := MultiSelect("Menu entries for default client", menuOpts, &wildcard.Entries); err != nil {
			return err
		}
	}

	// Additional specific clients.
	for {
		add := false
		if err := Confirm("Add a specific client (by MAC)?", &add); err != nil {
			return err
		}
		if !add {
			break
		}

		c, err := promptClient(menuOpts)
		if err != nil {
			return err
		}
		// Insert before wildcard (wildcard should be last).
		s.Clients = append(s.Clients[:len(s.Clients)-1], c, s.Clients[len(s.Clients)-1])
	}

	return nil
}

func stepClientsEdit(s *WizardState) error {
	fmt.Println("  Current clients:")
	for _, c := range s.Clients {
		fmt.Printf("    - %s (%s): menus=%v\n", c.Name, c.MAC, c.Entries)
	}
	fmt.Println()

	menuNames := s.MenuNames()
	menuOpts := make([]Option[string], len(menuNames))
	for i, n := range menuNames {
		menuOpts[i] = Option[string]{Label: n, Value: n}
	}

	for {
		add := false
		if err := Confirm("Add another client?", &add); err != nil {
			return err
		}
		if !add {
			break
		}

		c, err := promptClient(menuOpts)
		if err != nil {
			return err
		}
		// Insert before wildcard if it exists.
		if len(s.Clients) > 0 && s.Clients[len(s.Clients)-1].MAC == "*" {
			s.Clients = append(s.Clients[:len(s.Clients)-1], c, s.Clients[len(s.Clients)-1])
		} else {
			s.Clients = append(s.Clients, c)
		}
	}

	return nil
}

func promptClient(menuOpts []Option[string]) (ClientState, error) {
	var c ClientState

	if err := Input("MAC address", "aa:bb:cc:dd:ee:ff", &c.MAC, func(v string) error {
		_, err := domain.ParseMAC(v)
		return err
	}); err != nil {
		return c, err
	}

	if err := Input("Hostname", "server-1", &c.Name, func(v string) error {
		if v == "" {
			return fmt.Errorf("hostname is required")
		}
		return nil
	}); err != nil {
		return c, err
	}

	if err := MultiSelect("Menu entries", menuOpts, &c.Entries); err != nil {
		return c, err
	}

	if len(c.Entries) > 1 {
		// Default menu.
		defOpts := make([]Option[string], len(c.Entries))
		for i, e := range c.Entries {
			defOpts[i] = Option[string]{Label: e, Value: e}
		}
		if err := Select("Default boot entry", defOpts, &c.Default); err != nil {
			return c, err
		}

		// Timeout.
		var timeoutStr string = "30"
		if err := Input("Boot timeout (seconds, 0 = wait forever)", "30", &timeoutStr, func(v string) error {
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("must be a number")
			}
			if n < 0 {
				return fmt.Errorf("must not be negative")
			}
			return nil
		}); err != nil {
			return c, err
		}
		c.Timeout, _ = strconv.Atoi(timeoutStr)
	}

	return c, nil
}

func stepSummary(s *WizardState) error {
	fmt.Println("  ── Configuration Summary ──")
	fmt.Println()
	fmt.Printf("  Interface:    %s\n", s.Interface)
	fmt.Printf("  Data dir:     %s\n", s.DataDir)
	fmt.Println()

	fmt.Println("  Services:")
	if s.DHCPEnabled {
		fmt.Printf("    DHCP proxy: port %d, proxy port %d\n", s.DHCPPort, s.DHCPProxyPort)
	} else {
		fmt.Println("    DHCP proxy: disabled")
	}
	if s.TFTPEnabled {
		fmt.Printf("    TFTP:       port %d\n", s.TFTPPort)
	} else {
		fmt.Println("    TFTP:       disabled")
	}
	if s.HTTPEnabled {
		fmt.Printf("    HTTP:       port %d\n", s.HTTPPort)
	} else {
		fmt.Println("    HTTP:       disabled")
	}
	fmt.Println()

	if s.DownloadIPXE {
		fmt.Printf("  iPXE download: %v from %s\n", s.IPXEArchs, s.IPXEBaseURL)
	}
	fmt.Println()

	fmt.Printf("  Menus: %d entries\n", len(s.Menus))
	for _, m := range s.Menus {
		fmt.Printf("    - %s (%s)\n", m.Name, m.Type)
	}
	fmt.Println()

	fmt.Printf("  Clients: %d entries\n", len(s.Clients))
	for _, c := range s.Clients {
		fmt.Printf("    - %s (%s)\n", c.Name, c.MAC)
	}
	fmt.Println()

	write := true
	return Confirm("Write configuration?", &write)
}

// inputPort prompts for a port number with validation.
func inputPort(title string, port *int) error {
	portStr := strconv.Itoa(*port)
	if err := Input(title, strconv.Itoa(*port), &portStr, func(v string) error {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("must be a number")
		}
		if n < 1 || n > 65535 {
			return fmt.Errorf("port must be 1-65535")
		}
		return nil
	}); err != nil {
		return err
	}
	*port, _ = strconv.Atoi(portStr)
	return nil
}
