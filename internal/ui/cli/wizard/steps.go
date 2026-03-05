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

// RunEdit runs the wizard with section-based navigation for an existing config.
func RunEdit(configDir string, existing *domain.FullConfig) (*domain.FullConfig, error) {
	state := ConfigToState(existing)
	state.ConfigDir = configDir

	type editSection string
	const (
		editServer     editSection = "server"
		editServices   editSection = "services"
		editBootloader editSection = "bootloader"
		editNetboot    editSection = "netboot"
		editMenus      editSection = "menus"
		editClients    editSection = "clients"
		editSave       editSection = "save"
	)

	fmt.Println()
	fmt.Println("  Bootforge Configuration Editor")
	fmt.Println("  ──────────────────────────────")
	fmt.Println()

	// Show current config summary.
	stepSummary(state)
	fmt.Println()

	sectionOpts := []Option[editSection]{
		{Label: "Server (interface, data directory)", Value: editServer},
		{Label: "Services (DHCP, TFTP, HTTP)", Value: editServices},
		{Label: "Bootloader (iPXE files)", Value: editBootloader},
		{Label: "netboot.xyz", Value: editNetboot},
		{Label: "Menu entries", Value: editMenus},
		{Label: "Clients", Value: editClients},
		{Label: "Save and exit", Value: editSave},
	}

	for {
		section := editSave
		if err := Select("What would you like to edit?", sectionOpts, &section); err != nil {
			return nil, err
		}

		var err error
		switch section {
		case editServer:
			if err = stepInterface(state); err == nil {
				err = stepDataDir(state)
			}
		case editServices:
			err = stepServices(state)
		case editBootloader:
			err = stepBootloader(state)
		case editNetboot:
			err = stepNetboot(state)
		case editMenus:
			err = stepMenusEdit(state)
		case editClients:
			err = stepClientsEdit(state)
		case editSave:
			fmt.Println()
			stepSummary(state)

			write := true
			if err := Confirm("Save configuration?", &write); err != nil {
				return nil, err
			}
			if !write {
				continue
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

		if err != nil {
			return nil, err
		}
		fmt.Println()
	}
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
	// Detect current netboot mode from existing menus.
	for _, m := range s.Menus {
		if m.Name == "netboot-xyz" && m.Type == "chain" {
			if strings.Contains(m.Chain, "boot.netboot.xyz") {
				s.NetbootMode = NetbootRemote
			} else if m.Chain != "" {
				s.NetbootMode = NetbootSelfHosted
			}
			break
		}
	}

	modeOpts := []Option[NetbootMode]{
		{Label: "Remote (chain to boot.netboot.xyz, requires internet)", Value: NetbootRemote},
		{Label: "Self-hosted (download files, serve locally)", Value: NetbootSelfHosted},
		{Label: "Skip", Value: NetbootSkip},
	}
	if err := Select("Add netboot.xyz to your boot menu?", modeOpts, &s.NetbootMode); err != nil {
		return err
	}

	// Remove any existing netboot-xyz from menus and client entries.
	filtered := s.Menus[:0]
	for _, m := range s.Menus {
		if m.Name != "netboot-xyz" {
			filtered = append(filtered, m)
		}
	}
	s.Menus = filtered

	for i := range s.Clients {
		var kept []string
		for _, e := range s.Clients[i].Entries {
			if e != "netboot-xyz" {
				kept = append(kept, e)
			}
		}
		s.Clients[i].Entries = kept
	}

	switch s.NetbootMode {
	case NetbootRemote:
		s.Menus = append(s.Menus, MenuState{
			Name:  "netboot-xyz",
			Label: "netboot.xyz",
			Type:  "chain",
			Chain: "https://boot.netboot.xyz",
		})
		addNetbootToClients(s)
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
		addNetbootToClients(s)
	}

	return nil
}

// addNetbootToClients appends "netboot-xyz" to every client's entries list.
func addNetbootToClients(s *WizardState) {
	for i := range s.Clients {
		s.Clients[i].Entries = append(s.Clients[i].Entries, "netboot-xyz")
	}
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
	type menuAction string
	const (
		menuAdd    menuAction = "add"
		menuEdit   menuAction = "edit"
		menuRemove menuAction = "remove"
		menuDone   menuAction = "done"
	)

	for {
		fmt.Println("  Current menus:")
		for _, m := range s.Menus {
			fmt.Printf("    - %s (%s): %s\n", m.Name, m.Type, m.Label)
		}
		fmt.Println()

		opts := []Option[menuAction]{
			{Label: "Add new menu entry", Value: menuAdd},
		}
		if len(s.Menus) > 0 {
			opts = append(opts,
				Option[menuAction]{Label: "Edit existing menu entry", Value: menuEdit},
				Option[menuAction]{Label: "Remove menu entry", Value: menuRemove},
			)
		}
		opts = append(opts, Option[menuAction]{Label: "Done", Value: menuDone})

		action := menuDone
		if err := Select("What would you like to do?", opts, &action); err != nil {
			return err
		}

		switch action {
		case menuAdd:
			m, err := promptMenuEntry()
			if err != nil {
				return err
			}
			s.Menus = append(s.Menus, m)

		case menuEdit:
			menuOpts := make([]Option[int], len(s.Menus))
			for i, m := range s.Menus {
				menuOpts[i] = Option[int]{
					Label: fmt.Sprintf("%s (%s): %s", m.Name, m.Type, m.Label),
					Value: i,
				}
			}
			idx := 0
			if err := Select("Which menu entry to edit?", menuOpts, &idx); err != nil {
				return err
			}
			oldName := s.Menus[idx].Name
			if err := promptMenuEntryEdit(&s.Menus[idx]); err != nil {
				return err
			}
			// Cascade name change to client references.
			if newName := s.Menus[idx].Name; oldName != newName {
				for i := range s.Clients {
					for j, e := range s.Clients[i].Entries {
						if e == oldName {
							s.Clients[i].Entries[j] = newName
						}
					}
					if s.Clients[i].Default == oldName {
						s.Clients[i].Default = newName
					}
				}
			}

		case menuRemove:
			var removeOpts []Option[int]
			for i, m := range s.Menus {
				if m.Name == "local-disk" {
					continue
				}
				removeOpts = append(removeOpts, Option[int]{
					Label: fmt.Sprintf("%s (%s): %s", m.Name, m.Type, m.Label),
					Value: i,
				})
			}
			if len(removeOpts) == 0 {
				fmt.Println("  No removable menu entries (local-disk is protected).")
				continue
			}
			idx := removeOpts[0].Value
			if err := Select("Which menu entry to remove?", removeOpts, &idx); err != nil {
				return err
			}
			removedName := s.Menus[idx].Name

			confirm := false
			if err := Confirm(fmt.Sprintf("Remove %q?", removedName), &confirm); err != nil {
				return err
			}
			if !confirm {
				continue
			}

			s.Menus = append(s.Menus[:idx], s.Menus[idx+1:]...)
			for i := range s.Clients {
				var kept []string
				for _, e := range s.Clients[i].Entries {
					if e != removedName {
						kept = append(kept, e)
					}
				}
				s.Clients[i].Entries = kept
				if s.Clients[i].Default == removedName {
					s.Clients[i].Default = ""
				}
			}
			fmt.Printf("  Removed %q.\n", removedName)

		case menuDone:
			return nil
		}
		fmt.Println()
	}
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

// promptMenuEntryEdit edits a MenuState in-place with pre-filled prompts.
func promptMenuEntryEdit(m *MenuState) error {
	if err := Input("Menu name (slug)", m.Name, &m.Name, func(v string) error {
		if v == "" {
			return fmt.Errorf("name is required")
		}
		if strings.Contains(v, " ") {
			return fmt.Errorf("name must not contain spaces")
		}
		return nil
	}); err != nil {
		return err
	}

	if err := Input("Menu label (display text)", m.Label, &m.Label, func(v string) error {
		if v == "" {
			return fmt.Errorf("label is required")
		}
		return nil
	}); err != nil {
		return err
	}

	typeOpts := []Option[string]{
		{Label: "Install (full OS installation)", Value: "install"},
		{Label: "Live (RAM-based live system)", Value: "live"},
		{Label: "Tool (diagnostic/utility)", Value: "tool"},
		{Label: "Chain (chainload external iPXE menu)", Value: "chain"},
		{Label: "Exit (boot from local disk)", Value: "exit"},
	}
	if err := Select("Menu type", typeOpts, &m.Type); err != nil {
		return err
	}

	if m.Type == "chain" {
		if err := Input("Chain URL", m.Chain, &m.Chain, func(v string) error {
			if v == "" {
				return fmt.Errorf("chain URL is required")
			}
			return nil
		}); err != nil {
			return err
		}
		m.Kernel = ""
		m.Initrd = ""
		m.Cmdline = ""
	} else if m.Type != "exit" {
		if err := Input("Kernel filename", m.Kernel, &m.Kernel, nil); err != nil {
			return err
		}
		if err := Input("Initrd filename", m.Initrd, &m.Initrd, nil); err != nil {
			return err
		}
		if err := Input("Kernel command line (optional)", m.Cmdline, &m.Cmdline, nil); err != nil {
			return err
		}
		if err := Input("HTTP path prefix (e.g. /installers/ubuntu/)", m.HTTPPath, &m.HTTPPath, nil); err != nil {
			return err
		}
		m.Chain = ""
	} else {
		m.Kernel = ""
		m.Initrd = ""
		m.Cmdline = ""
		m.Chain = ""
	}

	return nil
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
	type clientAction string
	const (
		clientAdd    clientAction = "add"
		clientEdit   clientAction = "edit"
		clientRemove clientAction = "remove"
		clientDone   clientAction = "done"
	)

	for {
		fmt.Println("  Current clients:")
		for _, c := range s.Clients {
			defStr := ""
			if c.Default != "" {
				defStr = fmt.Sprintf(", default=%s", c.Default)
			}
			fmt.Printf("    - %s (%s): menus=%v%s\n", c.Name, c.MAC, c.Entries, defStr)
		}
		fmt.Println()

		menuNames := s.MenuNames()
		menuOpts := make([]Option[string], len(menuNames))
		for i, n := range menuNames {
			menuOpts[i] = Option[string]{Label: n, Value: n}
		}

		// Check if there are non-wildcard clients for remove.
		hasSpecific := false
		for _, c := range s.Clients {
			if c.MAC != "*" {
				hasSpecific = true
				break
			}
		}

		opts := []Option[clientAction]{
			{Label: "Add new client", Value: clientAdd},
		}
		if len(s.Clients) > 0 {
			opts = append(opts, Option[clientAction]{Label: "Edit existing client", Value: clientEdit})
		}
		if hasSpecific {
			opts = append(opts, Option[clientAction]{Label: "Remove client", Value: clientRemove})
		}
		opts = append(opts, Option[clientAction]{Label: "Done", Value: clientDone})

		action := clientDone
		if err := Select("What would you like to do?", opts, &action); err != nil {
			return err
		}

		switch action {
		case clientAdd:
			c, err := promptClient(menuOpts)
			if err != nil {
				return err
			}
			if len(s.Clients) > 0 && s.Clients[len(s.Clients)-1].MAC == "*" {
				s.Clients = append(s.Clients[:len(s.Clients)-1], c, s.Clients[len(s.Clients)-1])
			} else {
				s.Clients = append(s.Clients, c)
			}

		case clientEdit:
			clientOpts := make([]Option[int], len(s.Clients))
			for i, c := range s.Clients {
				clientOpts[i] = Option[int]{
					Label: fmt.Sprintf("%s (%s)", c.Name, c.MAC),
					Value: i,
				}
			}
			idx := 0
			if err := Select("Which client to edit?", clientOpts, &idx); err != nil {
				return err
			}
			if err := promptClientEdit(&s.Clients[idx], menuOpts); err != nil {
				return err
			}

		case clientRemove:
			var removeOpts []Option[int]
			for i, c := range s.Clients {
				if c.MAC == "*" {
					continue
				}
				removeOpts = append(removeOpts, Option[int]{
					Label: fmt.Sprintf("%s (%s)", c.Name, c.MAC),
					Value: i,
				})
			}
			if len(removeOpts) == 0 {
				fmt.Println("  No removable clients (wildcard is protected).")
				continue
			}
			idx := removeOpts[0].Value
			if err := Select("Which client to remove?", removeOpts, &idx); err != nil {
				return err
			}
			confirm := false
			if err := Confirm(fmt.Sprintf("Remove %q (%s)?", s.Clients[idx].Name, s.Clients[idx].MAC), &confirm); err != nil {
				return err
			}
			if !confirm {
				continue
			}
			s.Clients = append(s.Clients[:idx], s.Clients[idx+1:]...)
			fmt.Println("  Removed client.")

		case clientDone:
			return nil
		}
		fmt.Println()
	}
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

// promptClientEdit edits a ClientState in-place with pre-filled prompts.
func promptClientEdit(c *ClientState, menuOpts []Option[string]) error {
	fmt.Printf("  Editing client: %s (%s)\n\n", c.Name, c.MAC)

	if err := Input("Hostname", c.Name, &c.Name, func(v string) error {
		if v == "" {
			return fmt.Errorf("hostname is required")
		}
		return nil
	}); err != nil {
		return err
	}

	if err := MultiSelect("Menu entries", menuOpts, &c.Entries); err != nil {
		return err
	}

	if len(c.Entries) > 1 {
		// Validate current default is still in entries.
		found := false
		for _, e := range c.Entries {
			if e == c.Default {
				found = true
				break
			}
		}
		if !found {
			c.Default = c.Entries[0]
		}

		defOpts := make([]Option[string], len(c.Entries))
		for i, e := range c.Entries {
			defOpts[i] = Option[string]{Label: e, Value: e}
		}
		if err := Select("Default boot entry", defOpts, &c.Default); err != nil {
			return err
		}

		timeoutStr := strconv.Itoa(c.Timeout)
		if err := Input("Boot timeout (seconds, 0 = wait forever)", timeoutStr, &timeoutStr, func(v string) error {
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("must be a number")
			}
			if n < 0 {
				return fmt.Errorf("must not be negative")
			}
			return nil
		}); err != nil {
			return err
		}
		c.Timeout, _ = strconv.Atoi(timeoutStr)
	} else {
		c.Default = ""
		c.Timeout = 0
	}

	return nil
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
