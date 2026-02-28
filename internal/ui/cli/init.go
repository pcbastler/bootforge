package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initDir string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new configuration directory",
	Long:  "Create a new configuration directory with example configuration files.",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().StringVar(&initDir, "dir", "", "Directory to initialize (required)")
	initCmd.MarkFlagRequired("dir")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	// Check if directory already exists and has content.
	entries, err := os.ReadDir(initDir)
	if err == nil && len(entries) > 0 {
		return fmt.Errorf("directory %s already exists and is not empty", initDir)
	}

	// Create directory structure.
	dirs := []string{
		initDir,
		filepath.Join(initDir, "data", "bootloader"),
		filepath.Join(initDir, "data", "tools"),
		filepath.Join(initDir, "data", "installers"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", d, err)
		}
	}

	// Write example configuration.
	cfgPath := filepath.Join(initDir, "bootforge.toml")
	if err := os.WriteFile(cfgPath, []byte(exampleConfig), 0644); err != nil {
		return fmt.Errorf("writing example config: %w", err)
	}

	fmt.Printf("Initialized configuration in %s\n\n", initDir)
	fmt.Println("Created:")
	fmt.Printf("  %s\n", cfgPath)
	for _, d := range dirs[1:] {
		fmt.Printf("  %s/\n", d)
	}
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Place bootloader files in %s/\n", filepath.Join(initDir, "data", "bootloader"))
	fmt.Printf("  2. Edit %s to match your network\n", cfgPath)
	fmt.Printf("  3. Run: bootforge validate --config %s\n", initDir)
	fmt.Printf("  4. Run: bootforge serve --config %s\n", initDir)

	return nil
}

const exampleConfig = `# Bootforge Configuration
# See documentation for all options.

[server]
interface = "eth0"
data_dir = "./data"

[server.logging]
level = "info"
format = "pretty"

[bootloader]
dir = "bootloader"
uefi_x64 = "ipxe.efi"
uefi_x86 = "ipxe-x86.efi"
bios = "undionly.kpxe"
arm64 = "ipxe-arm64.efi"
chain_url = "http://${server_ip}:${http_port}/boot/${mac}/menu.ipxe"

[dhcp_proxy]
enabled = true
port = 67
proxy_port = 4011

[tftp]
enabled = true
port = 69
block_size = 512
timeout = "5s"

[http]
enabled = true
port = 8080
read_timeout = "30s"

[health]
enabled = true
interval = "30s"
startup_check = true

# --- Boot Menus ---

[[menu]]
name = "local-disk"
label = "Boot from local disk"
type = "exit"

[[menu]]
name = "rescue"
label = "Rescue System"
type = "live"

[menu.boot]
kernel = "vmlinuz"
initrd = "initrd"

[menu.http]
path = "/tools/rescue/"
files = "data/tools/rescue"

# --- Clients ---

# Default client (matches any MAC not explicitly listed).
[[client]]
mac = "*"
name = "default"

[client.menu]
entries = ["local-disk"]

# Example client (uncomment and customize):
# [[client]]
# mac = "aa:bb:cc:dd:ee:01"
# name = "my-server"
#
# [client.menu]
# entries = ["rescue", "local-disk"]
# default = "rescue"
# timeout = 10
`
