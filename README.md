# Bootforge

**PXE Boot Server for automated network booting.** Single binary combining DHCP Proxy, TFTP, and HTTP into one cohesive tool. Auto-detects UEFI and BIOS clients, generates iPXE boot menus, and serves OS installers, live systems, and diagnostic tools over the network.

> **Note:** This project is under active development. APIs may change without notice.

> **Disclaimer:** This software is provided "as is", without warranty of any kind, express or
> implied, including but not limited to the warranties of merchantability, fitness for a particular
> purpose, correctness, or completeness. This software may contain bugs that can destroy or
> corrupt data, including virtual disk images, and may lead to complete data loss. Use at your own
> risk. The author assumes no liability for any damages, including data loss, arising from the use
> of this software.

---

## What Is Bootforge?

Bootforge is a self-contained PXE boot server written in Go. It replaces the typical patchwork of `dnsmasq`, `tftpd`, `nginx`, and hand-written iPXE scripts with a single binary and a set of TOML configuration files.

When a machine boots from the network, Bootforge:

1. **Detects the architecture** (UEFI x64, UEFI x86, BIOS, ARM64) via DHCP Option 93
2. **Serves the right iPXE bootloader** via TFTP (4 files, one per architecture)
3. **Generates a boot menu** via HTTP, tailored to the specific client (by MAC address)
4. **Serves OS files** (kernels, initrds, ISOs) via HTTP for the selected boot option

All of this happens without touching your existing DHCP server. Bootforge runs as a **DHCP proxy** — it adds PXE boot information to DHCP responses without assigning IP addresses.

## What Can You Do With It?

- **Install operating systems** over the network (Debian, Ubuntu, Windows, etc.)
- **Boot live systems** without touching the local disk (Alpine, rescue environments)
- **Run diagnostic tools** (memtest, disk utilities, custom kernels)
- **Manage multiple clients** with different boot menus per MAC address
- **Define a default menu** for unknown machines using a wildcard client
- **Monitor boot sessions** in real-time through the REST API
- **Run health checks** to verify bootloader files, disk space, and service availability
- **Wake machines remotely** via Wake-on-LAN (library included, CLI integration planned)

## How It Works

### The Three-Layer Boot Model

```
  ┌──────────────────┐     ┌─────────────────────┐     ┌───────────────┐
  │   Bootloader     │     │       Menu          │     │    Client     │
  │                  │     │                     │     │               │
  │  HOW does iPXE   │────▶│  WHAT can the       │◀────│  WHO gets     │
  │  get on the      │     │  machine do after   │     │  which menu?  │
  │  client?         │     │  iPXE loads?        │     │               │
  │                  │     │                     │     │  MAC-based    │
  │  Auto-detect     │     │  OS installs, live  │     │  assignment   │
  │  via DHCP Opt 93 │     │  systems, tools     │     │  + wildcard   │
  └──────────────────┘     └─────────────────────┘     └───────────────┘
```

Each layer is independent:
- **Menus** know nothing about UEFI vs BIOS
- **Clients** know nothing about file paths
- **Bootloaders** know nothing about menu content

### Boot Sequence

```
  Client powers on
       │
       ▼
  PXE firmware sends DHCP Discover
       │
       ▼
  ┌─────────────────────┐
  │  Existing DHCP      │ ◀── assigns IP (unchanged)
  │  Server             │
  └─────────────────────┘
       │
       ▼
  ┌─────────────────────┐
  │  Bootforge DHCP     │ ◀── adds next-server + bootfile
  │  Proxy (:67/:4011)  │     (no IP assignment)
  └─────────────────────┘
       │
       ▼
  Client downloads iPXE bootloader via TFTP (:69)
       │
       ▼
  iPXE chains to: http://server:8080/boot/{mac}/menu.ipxe
       │
       ▼
  ┌─────────────────────┐
  │  Bootforge HTTP     │ ◀── generates iPXE menu script
  │  Server (:8080)     │     based on client's MAC
  └─────────────────────┘
       │
       ▼
  User selects boot option (or timeout selects default)
       │
       ▼
  iPXE downloads kernel + initrd via HTTP and boots
```

### Services

| Service | Port | Purpose |
|---------|------|---------|
| DHCP Proxy | 67, 4011 | Responds to PXE boot requests, points to TFTP |
| TFTP | 69 | Serves iPXE bootloader files (4 files) |
| HTTP | 8080 | Serves iPXE menu scripts, kernels, initrds, ISOs |
| REST API | 8080 | `/api/v1/*` endpoints for management |

## Quick Start

### Prerequisites

- **Go 1.24+** (for building from source)
- **Root privileges** (DHCP proxy requires port 67)
- **An existing DHCP server** on the network (Bootforge does not assign IPs)
- **iPXE bootloader files** (`ipxe.efi`, `undionly.kpxe`, etc.)

### 1. Build

```bash
git clone <repo-url> && cd bootforge
make build
```

### 2. Initialize Configuration

```bash
sudo ./bootforge init --dir /etc/bootforge
```

The interactive wizard walks you through:
- Selecting the network interface
- Setting up the data directory
- Downloading iPXE bootloader files
- Creating an initial menu and default client

### 3. Add Boot Files

Place OS files into the data directory:

```
/etc/bootforge/data/
├── bootloader/           # iPXE files (created by init)
│   ├── ipxe.efi          #   UEFI x64
│   ├── ipxe-i386.efi     #   UEFI x86
│   ├── ipxe-arm64.efi    #   ARM64
│   └── undionly.kpxe     #   BIOS
├── installers/
│   └── debian/           # Example: Debian netinstall
│       ├── linux          #   kernel
│       └── initrd.gz      #   initrd
└── live/
    └── alpine/           # Example: Alpine live
        ├── vmlinuz-lts
        └── initramfs-lts
```

### 4. Configure Menus and Clients

Edit `/etc/bootforge/bootforge.toml` (or split across multiple `.toml` files):

```toml
[server]
interface = "eth0"
data_dir = "./data"

[dhcp_proxy]
enabled = true

[tftp]
enabled = true

[http]
enabled = true
port = 8080

[bootloader]
dir = "bootloader"
uefi_x64 = "ipxe.efi"
bios = "undionly.kpxe"
chain_url = "http://${server_ip}:${http_port}/boot/${mac}/menu.ipxe"

# --- Menu Entries ---

[[menu]]
name = "debian-install"
label = "Debian 12 (Netinstall)"
type = "install"

[menu.http]
files = "installers/debian/"
path = "/data/installers/debian/"

[menu.boot]
kernel = "linux"
initrd = "initrd.gz"
cmdline = "---"

[[menu]]
name = "alpine-live"
label = "Alpine Linux (Live)"
type = "live"

[menu.http]
files = "live/alpine/"
path = "/data/live/alpine/"

[menu.boot]
kernel = "vmlinuz-lts"
initrd = "initramfs-lts"
cmdline = "ip=dhcp console=tty0"

[[menu]]
name = "local-disk"
label = "Boot from local disk"
type = "exit"

# --- Clients ---

# Default: all unknown machines get this menu
[[client]]
mac = "*"
name = "default"
enabled = true

[client.menu]
entries = ["debian-install", "alpine-live", "local-disk"]
default = "alpine-live"
timeout = 10

# Specific machine gets a different menu
[[client]]
mac = "AA:BB:CC:DD:EE:FF"
name = "build-server"
enabled = true

[client.menu]
entries = ["debian-install", "local-disk"]
default = "debian-install"
timeout = 5
```

### 5. Validate and Start

```bash
# Check configuration without starting
sudo ./bootforge validate --config /etc/bootforge

# Start the server
sudo ./bootforge serve --config /etc/bootforge
```

Output:

```
Bootforge v0.1.0 (abc1234)

Pre-Flight Checks

  OK    Bootloader file: ipxe.efi
  OK    Bootloader file: undionly.kpxe
  OK    Disk space: /etc/bootforge/data (2.1 GB free)
  OK    HTTP health endpoint

All checks passed. Starting services...

Services running:
  TFTP:  :69
  HTTP:  :8080
  API:   :8080/api/v1/
  DHCP:  :67 (proxy :4011) -- requires root
```

Now PXE-boot any machine on the network.

## CLI Reference

```
bootforge serve                              # Start all services
bootforge init [--dir PATH]                  # Interactive setup wizard
bootforge validate [--config PATH]           # Check config without starting
bootforge status                             # Query running server
bootforge precheck [--config PATH]           # Run pre-flight health checks
bootforge edit                               # Edit config interactively

bootforge client list                        # List all clients
bootforge client show <mac>                  # Show client details

bootforge menu list                          # List all menu entries
bootforge menu show <name>                   # Show menu entry details

bootforge bootloader list                    # List configured bootloaders
bootforge bootloader check                   # Verify bootloader files exist

bootforge session list                       # List active boot sessions
bootforge session show <mac>                 # Show session details

bootforge logs [--level LEVEL]               # View server logs
bootforge config show                        # Show current configuration
bootforge test                               # Run health checks on-demand
bootforge version                            # Print version info
```

Global flags: `--config PATH` (config directory), `--debug` (verbose logging).

## REST API

All endpoints are served on the HTTP port (default 8080) under `/api/v1/`.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/status` | Server status and health |
| GET | `/api/v1/clients` | List all clients |
| GET | `/api/v1/clients/{mac}` | Client details |
| GET | `/api/v1/menus` | List all menu entries |
| GET | `/api/v1/sessions` | Active boot sessions |
| POST | `/api/v1/reload` | Hot-reload configuration |
| POST | `/api/v1/test` | Run health checks |
| GET | `/api/v1/logs` | Recent log entries (supports `?mac=`, `?service=`, `?limit=`) |
| GET | `/healthz` | Health check endpoint |

Boot endpoints (called by iPXE, not for human use):

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/boot/{mac}/menu.ipxe` | Generated iPXE menu script |

## Configuration

Bootforge reads **all `.toml` files** in the config directory and detects content by structure. You can use a single file or split by concern:

```
/etc/bootforge/
├── bootforge.toml      # All-in-one, OR split into:
├── server.toml          # [server], [dhcp_proxy], [tftp], [http], [bootloader]
├── menus.toml           # [[menu]] entries
├── workstations.toml    # [[client]] entries for workstations
├── servers.toml         # [[client]] entries for servers
└── defaults.toml        # [[client]] mac="*" wildcard
```

### Menu Types

| Type | Purpose | iPXE behavior |
|------|---------|---------------|
| `install` | OS installation (Debian, Ubuntu, Windows) | Boot kernel + initrd |
| `live` | Live system, no disk writes | Boot kernel + initrd |
| `tool` | Diagnostic tool (memtest, rescue) | Chain binary or boot kernel |
| `exit` | Return to local disk | `exit 0` in iPXE |

### Client Wildcards

A client with `mac = "*"` serves as the default for any machine not explicitly configured. This is how you provide a boot menu for unknown or new machines without listing every MAC address.

### Per-Client Bootloader Overrides

Clients can override the global bootloader configuration:

```toml
[[client]]
mac = "AA:BB:CC:DD:EE:FF"
name = "legacy-box"

[client.bootloader]
bios = "custom-undionly.kpxe"
ipxe_variant = "snponly"
```

### Variable Substitution

Boot command lines and URLs support variables:

| Variable | Replaced with |
|----------|--------------|
| `${server_ip}` | Server's IP address |
| `${http_port}` | HTTP port |
| `${mac}` | Client's MAC address |

Custom per-client variables can be defined in `[client.vars]`.

## Health Checks

Bootforge runs automatic health checks at a configurable interval:

- **Bootloader files** — verifies all configured iPXE files exist
- **Disk space** — monitors free space in the data directory
- **HTTP endpoint** — checks that the HTTP server responds
- **File integrity** — validates boot file checksums
- **DHCP probe** — tests DHCP response
- **TFTP probe** — tests TFTP file retrieval

Pre-flight checks run at startup (skippable with `--force`) and catch configuration problems before the server begins accepting connections.

## Advantages

- **Single binary** — no dnsmasq, no tftpd, no nginx, no scripting glue. One binary does everything.
- **Zero interference with existing DHCP** — runs as a proxy, never assigns IP addresses. Drop it into any network without reconfiguring your DHCP server.
- **Automatic architecture detection** — UEFI x64, UEFI x86, BIOS, and ARM64 are detected via DHCP Option 93. No manual bootfile selection needed.
- **Clean separation of concerns** — menus, clients, and bootloaders are independent. Add an OS once, assign it to any number of clients.
- **Flexible configuration** — split TOML files by team, room, purpose, or keep everything in one file. Bootforge merges all `.toml` files automatically.
- **Built-in health checks** — catches missing files, disk space issues, and service failures before users notice.
- **Hot-reload** — change menus or clients without restarting the server (`POST /api/v1/reload`).
- **Interactive setup** — `bootforge init` walks through network detection, bootloader download, and initial configuration.
- **Boot session tracking** — see which machines are booting, what state they're in, and whether they succeeded.

## Limitations

- **Requires root** — the DHCP proxy binds to port 67, which requires root privileges on Linux. TFTP on port 69 also requires root.
- **No DHCP server** — Bootforge is a proxy only. You need an existing DHCP server on the network that assigns IP addresses.
- **IPv4 only** — PXE booting is an IPv4 protocol. IPv6 network boot (HTTP Boot) is not supported.
- **iPXE only** — the generated menu system is iPXE-specific. Legacy PXE clients without iPXE chainloading support are handled (BIOS/UEFI auto-detection serves the right iPXE binary), but the menu itself requires iPXE.
- **No Web UI yet** — the REST API is functional, but the browser-based dashboard is not yet implemented. Management is CLI and API only.
- **No iSCSI / diskless boot** — planned for a future release but not yet available.
- **No authentication** — the REST API has no authentication or authorization. Secure it with a reverse proxy or firewall rules if exposed beyond the local network.
- **No HTTPS for boot files** — iPXE has limited TLS support. Boot files are served over plain HTTP. This is standard for PXE environments but means boot traffic is not encrypted.
- **Single server** — no clustering or high-availability mode. Bootforge runs on one machine.
- **Linux only** — the DHCP proxy uses `SO_BINDTODEVICE` and other Linux-specific socket options. It does not run on macOS or Windows.

## Building from Source

```bash
# Debug build
make build

# Release build (stripped binary)
make release

# Run tests
make test

# Run tests with race detector
make test-race

# Lint
make lint

# All checks (vet + race tests)
make check
```

## Requirements

| Requirement | Details |
|-------------|---------|
| Go | 1.24+ |
| OS | Linux (kernel 3.10+) |
| Privileges | Root (for DHCP port 67, TFTP port 69) |
| Network | Existing DHCP server on the same broadcast domain |
| Files | iPXE bootloader binaries (downloadable via `bootforge init`) |

## Roadmap

See [TODO.md](TODO.md) for planned features, including additional CLI commands, extended API endpoints, WebSocket streaming, and a web UI.

## Development

This project was developed with AI assistance.

## License

See [LICENSE](LICENSE) file.
