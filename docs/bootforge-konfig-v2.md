# 🔥 BOOTFORGE v2 — Modulare Konfigurationsstruktur

---

## 1. Verzeichnisstruktur (Gesamtübersicht)

```
/etc/bootforge/
├── bootforge.toml                        # Globale Server-Konfiguration
│
├── computers/                            # ── Computer-Profile ──
│   ├── default-bios.toml                 # Fallback: unbekannte BIOS-Clients
│   ├── default-uefi.toml                 # Fallback: unbekannte UEFI-Clients
│   ├── computer-raum-01.toml             # Gruppe: alle PCs in Raum 01
│   ├── computer-raum-02.toml             # Gruppe: alle PCs in Raum 02
│   ├── server-rack-a.toml                # Gruppe: Server Rack A
│   └── einzeln-spezial.toml              # Ein einzelner Sonder-PC
│
└── data/                                 # ── Boot-Dateien ──
    ├── uefi-01/
    │   ├── tftp/
    │   │   └── ipxe.efi
    │   └── http/
    │       ├── boot.ipxe
    │       ├── vmlinuz
    │       └── initrd
    │
    ├── bios-01/
    │   ├── tftp/
    │   │   └── undionly.kpxe
    │   └── http/
    │       ├── boot.ipxe
    │       ├── vmlinuz
    │       └── initrd
    │
    ├── ubuntu-24-PC/
    │   ├── tftp/
    │   │   └── ipxe.efi
    │   └── http/
    │       ├── boot.ipxe
    │       ├── vmlinuz
    │       ├── initrd
    │       └── preseed.cfg
    │
    └── rescue/
        ├── tftp/
        │   ├── ipxe.efi
        │   └── undionly.kpxe
        └── http/
            ├── boot.ipxe
            ├── vmlinuz
            └── initrd
```

---

## 2. Globale Server-Konfiguration

```toml
# /etc/bootforge/bootforge.toml
# ═══════════════════════════════════════════════
#  BOOTFORGE — Globale Server-Konfiguration
# ═══════════════════════════════════════════════

[server]
interface = "eth0"              # Netzwerk-Interface (leer = auto-detect)
ip        = ""                  # Server-IP (leer = vom Interface lesen)
data_dir  = "/etc/bootforge/data"

# Verzeichnis mit Computer-Profil-Dateien
computers_dir = "/etc/bootforge/computers"

[server.logging]
level  = "info"                 # trace | debug | info | warn | error
format = "pretty"               # pretty | json
file   = ""                     # Leer = nur stdout

# ─── Dienste ──────────────────────────────────

[dhcp_proxy]
enabled           = true
port              = 67
proxy_port        = 4011
vendor_class      = "PXEClient"

[tftp]
enabled    = true
port       = 69
block_size = 1468
timeout    = "5s"
retries    = 5

[http]
enabled      = true
port         = 8080
read_timeout = "30s"

[http.tls]
enabled = false
cert    = ""
key     = ""

# ─── Health & Diagnostics ─────────────────────

[health]
enabled       = true
interval      = "30s"
startup_check = true

[health.checks]
dhcp_probe     = true
tftp_read      = true
http_get       = true
file_integrity = true
disk_space_min = "1GB"

[diagnostics]
enabled         = true
session_timeout = "10m"

[diagnostics.timeouts]
discover_to_offer = "5s"
offer_to_tftp     = "15s"
tftp_to_http      = "30s"
http_to_kernel    = "60s"
kernel_to_preseed = "120s"

[diagnostics.alerts]
missing_file_on_request = true
stalled_boot            = true
unknown_mac             = "warn"         # warn | ignore | reject
```

---

## 3. Computer-Profile (das Herzstück)

### 3.1 Aufbau einer Profil-Datei

```
┌──────────────────────────────────────────────────────────────────┐
│  Eine .toml Datei in computers/ enthält:                        │
│                                                                  │
│  [[client]]       ← Beliebig viele Clients pro Datei            │
│  [[client]]       ← Noch einer                                  │
│  [[client]]       ← Und noch einer...                           │
│                                                                  │
│  Jeder [[client]] hat:                                          │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │  mac       = "aa:bb:cc:dd:ee:ff"   (Pflicht, oder "*")    │ │
│  │  name      = "webserver-01"         (Optional, für Logs)   │ │
│  │  type      = "uefi-only"           (Pflicht)               │ │
│  │                                                             │ │
│  │  Mögliche type-Werte:                                      │ │
│  │  ┌──────────────┬────────────────────────────────────────┐ │ │
│  │  │ uefi-only    │ Nur [client.uefi] wird gelesen         │ │ │
│  │  │ bios-only    │ Nur [client.bios] wird gelesen         │ │ │
│  │  │ auto         │ Beide Sektionen, Auswahl per Option 93 │ │ │
│  │  └──────────────┴────────────────────────────────────────┘ │ │
│  │                                                             │ │
│  │  [client.uefi]  → UEFI Boot-Konfiguration                 │ │
│  │  [client.bios]  → BIOS Boot-Konfiguration                 │ │
│  │  [client.vars]  → Template-Variablen (optional)            │ │
│  └────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────┘
```

### 3.2 TFTP/HTTP Quelle: Intern vs. Extern

```
┌──────────────────────────────────────────────────────────────────┐
│  Jede Sektion (uefi/bios) kann pro Dienst wählen:              │
│                                                                  │
│  TFTP:                                                          │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │  OPTION A — Interner TFTP (bootforge serviert selbst)     │ │
│  │  tftp_files = "data/uefi-01/tftp/"                        │ │
│  │  bootfile   = "ipxe.efi"                                  │ │
│  │  → Bootforge stellt Dateien aus diesem Ordner via TFTP    │ │
│  │                                                            │ │
│  │  OPTION B — Externer TFTP (anderer Server)                │ │
│  │  tftp_server = "10.0.0.200"                               │ │
│  │  tftp_port   = 69                                         │ │
│  │  bootfile    = "ipxe.efi"                                 │ │
│  │  → Bootforge sagt dem Client nur: "geh dorthin"           │ │
│  │  → DHCP-Proxy setzt next-server auf externen Server       │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  HTTP:                                                          │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │  OPTION A — Interner HTTP (bootforge serviert selbst)     │ │
│  │  http_files = "data/uefi-01/http/"                        │ │
│  │  http_path  = "/uefi-01/"                                 │ │
│  │  → Dateien erreichbar unter http://bootforge:8080/uefi-01 │ │
│  │                                                            │ │
│  │  OPTION B — Externer HTTP (anderer Server)                │ │
│  │  http_server = "http://10.0.0.202:8080/uefi-01/"         │ │
│  │  → iPXE-Script verweist auf externen HTTP-Server          │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  Mischformen sind möglich:                                      │
│  TFTP intern + HTTP extern                                      │
│  TFTP extern + HTTP intern                                      │
│  Alles intern                                                   │
│  Alles extern (Bootforge = reiner DHCP-Proxy)                  │
└──────────────────────────────────────────────────────────────────┘
```

---

### 3.3 Beispiel-Dateien

#### `computers/default-uefi.toml` — Fallback für unbekannte UEFI-Clients

```toml
# Fallback-Profil: Jeder unbekannte UEFI-Client bekommt dies
# mac = "*" bedeutet: Wildcard/Default

[[client]]
mac  = "*"
name = "default-uefi"
type = "uefi-only"

    [client.uefi]
    tftp_files = "data/uefi-01/tftp/"
    bootfile   = "ipxe.efi"

    http_files = "data/uefi-01/http/"
    http_path  = "/default-uefi/"
```

#### `computers/default-bios.toml` — Fallback für unbekannte BIOS-Clients

```toml
[[client]]
mac  = "*"
name = "default-bios"
type = "bios-only"

    [client.bios]
    tftp_files = "data/bios-01/tftp/"
    bootfile   = "undionly.kpxe"

    http_files = "data/bios-01/http/"
    http_path  = "/default-bios/"
```

#### `computers/computer-raum-01.toml` — Ganzer Raum, gleiche Config

```toml
# ═══════════════════════════════════════════════
#  Computerraum 01 — 24x Desktop PCs
#  Ubuntu 24 LTS, UEFI Boot
# ═══════════════════════════════════════════════

# ─── PC-01 ────────────────────────────────────
[[client]]
mac  = "c6:c9:4b:45:bf:01"
name = "raum01-pc01"
type = "uefi-only"

    [client.uefi]
    tftp_files = "data/ubuntu-24-PC/tftp/"
    bootfile   = "ipxe.efi"

    http_files = "data/ubuntu-24-PC/http/"
    http_path  = "/raum01/pc01/"

    [client.vars]
    hostname = "raum01-pc01"
    locale   = "de_DE.UTF-8"
    timezone = "Europe/Berlin"
    disk     = "/dev/sda"
    packages = "libreoffice,firefox,thunderbird"

# ─── PC-02 ────────────────────────────────────
[[client]]
mac  = "c6:c9:4b:45:bf:02"
name = "raum01-pc02"
type = "uefi-only"

    [client.uefi]
    tftp_files = "data/ubuntu-24-PC/tftp/"
    bootfile   = "ipxe.efi"

    http_files = "data/ubuntu-24-PC/http/"
    http_path  = "/raum01/pc02/"

    [client.vars]
    hostname = "raum01-pc02"
    locale   = "de_DE.UTF-8"
    timezone = "Europe/Berlin"
    disk     = "/dev/sda"
    packages = "libreoffice,firefox,thunderbird"

# ─── PC-03 bis PC-24: gleiches Schema ────────
# ...
```

#### `computers/server-rack-a.toml` — Server mit Auto-Detect

```toml
# ═══════════════════════════════════════════════
#  Server Rack A — Gemischt UEFI/BIOS
#  Bootforge erkennt automatisch per Option 93
# ═══════════════════════════════════════════════

[[client]]
mac  = "aa:bb:cc:dd:ee:01"
name = "db-master"
type = "auto"

    # Wenn UEFI erkannt wird:
    [client.uefi]
    tftp_files = "data/uefi-01/tftp/"
    bootfile   = "ipxe.efi"

    http_files = "data/ubuntu-24-PC/http/"
    http_path  = "/rack-a/db-master/"

    # Wenn BIOS erkannt wird:
    [client.bios]
    tftp_files = "data/bios-01/tftp/"
    bootfile   = "undionly.kpxe"

    http_files = "data/ubuntu-24-PC/http/"
    http_path  = "/rack-a/db-master/"

    [client.vars]
    hostname = "db-master"
    disk     = "/dev/nvme0n1"
    packages = "postgresql-16,pg-stat-statements"

# ─── Server mit EXTERNEM TFTP ────────────────
[[client]]
mac  = "aa:bb:cc:dd:ee:02"
name = "legacy-box"
type = "bios-only"

    [client.bios]
    # TFTP kommt von einem anderen Server!
    tftp_server = "10.0.0.200"
    tftp_port   = 69
    bootfile    = "pxelinux.0"

    # HTTP aber lokal
    http_files = "data/rescue/http/"
    http_path  = "/rack-a/legacy/"

    [client.vars]
    hostname = "legacy-box"
```

#### `computers/einzeln-spezial.toml` — Spezialfall

```toml
# ═══════════════════════════════════════════════
#  Spezial-Maschine: komplett extern
#  Bootforge = reiner DHCP-Proxy
# ═══════════════════════════════════════════════

[[client]]
mac  = "ff:ee:dd:cc:bb:aa"
name = "pxe-test-bench"
type = "auto"

    [client.uefi]
    tftp_server = "10.0.0.200"
    bootfile    = "grubx64.efi"

    http_server = "http://10.0.0.202:8080/test/"

    [client.bios]
    tftp_server = "10.0.0.200"
    bootfile    = "pxelinux.0"

    http_server = "http://10.0.0.202:8080/test/"
```

---

## 4. Profil-Auflösung (Priorität)

```
  Client sendet DHCP DISCOVER mit MAC + Option 93 (Arch)
                        │
                        ▼
  ┌─────────────────────────────────────────────────┐
  │  1. Exakte MAC-Suche in allen computers/*.toml  │
  │     Gefunden? ──── JA ────▶ Profil verwenden    │
  │         │                                        │
  │        NEIN                                      │
  │         │                                        │
  │  2. Arch prüfen via Option 93                    │
  │         │                                        │
  │         ├── UEFI erkannt?                        │
  │         │     └─▶ default-uefi.toml (mac="*")   │
  │         │                                        │
  │         ├── BIOS erkannt?                        │
  │         │     └─▶ default-bios.toml (mac="*")   │
  │         │                                        │
  │         └── Unbekannt?                           │
  │               └─▶ diagnostics.alerts.unknown_mac │
  │                   warn / ignore / reject         │
  └─────────────────────────────────────────────────┘

  Bei type = "auto":
  ┌─────────────────────────────────────────────────┐
  │  Option 93 Wert  →  Sektion                     │
  │  ─────────────────────────────────               │
  │  0x0000 (BIOS)   →  [client.bios]               │
  │  0x0006 (UEFI32) →  [client.uefi]               │
  │  0x0007 (UEFI64) →  [client.uefi]               │
  │  0x0009 (UEFI64) →  [client.uefi]               │
  │  0x000B (ARM64)  →  [client.uefi]               │
  │                                                   │
  │  Wenn passende Sektion fehlt:                     │
  │  ⚠ WARN "MAC aa:bb:cc type=auto, BIOS erkannt,  │
  │          aber [client.bios] nicht konfiguriert!"  │
  └─────────────────────────────────────────────────┘
```

---

## 5. Interner HTTP Routing

```
  Bootforge baut die HTTP-Routen automatisch aus allen
  geladenen Profilen zusammen:

  computers/computer-raum-01.toml:
    client mac=...bf:01  http_path="/raum01/pc01/"  http_files="data/ubuntu-24-PC/http/"
    client mac=...bf:02  http_path="/raum01/pc02/"  http_files="data/ubuntu-24-PC/http/"

  computers/server-rack-a.toml:
    client mac=...ee:01  http_path="/rack-a/db-master/"  http_files="data/ubuntu-24-PC/http/"

  ═══════════════════════════════════════════════════════════

  Resultierende HTTP-Routen:

  GET /raum01/pc01/*       → data/ubuntu-24-PC/http/*
  GET /raum01/pc02/*       → data/ubuntu-24-PC/http/*
  GET /rack-a/db-master/*  → data/ubuntu-24-PC/http/*
  GET /default-uefi/*      → data/uefi-01/http/*
  GET /default-bios/*      → data/bios-01/http/*
  GET /healthz             → 200 OK (immer)
  GET /status              → JSON Status aller Sessions
  GET /metrics             → Prometheus Metriken

  ┌──────────────────────────────────────────────────────────┐
  │  HINWEIS: Mehrere Clients können auf denselben           │
  │  http_files Ordner zeigen — Bootforge dedupliziert       │
  │  die Routen automatisch. Aber http_path MUSS             │
  │  pro Client eindeutig sein!                              │
  │                                                           │
  │  Beim Start:                                              │
  │  ✗ FEHLER wenn zwei Clients gleichen http_path haben     │
  │  ✗ FEHLER wenn http_files Verzeichnis nicht existiert    │
  │  ⚠ WARNUNG wenn http_files leer ist                     │
  └──────────────────────────────────────────────────────────┘
```

---

## 6. Interner TFTP Routing

```
  TFTP ist MAC-basiert, nicht pfad-basiert:

  Client mit MAC aa:bb:cc:dd:ee:01 fragt "ipxe.efi" an
                        │
                        ▼
  ┌─────────────────────────────────────────────────────────┐
  │  1. MAC nachschlagen → Profil gefunden                  │
  │  2. Arch bestimmen → [client.uefi]                      │
  │  3. tftp_files = "data/uefi-01/tftp/"                   │
  │  4. Suche: data/uefi-01/tftp/ipxe.efi                  │
  │  5. Gefunden → Transfer starten                         │
  │                                                          │
  │  Nicht gefunden?                                         │
  │  ✗ ERROR "TFTP RRQ 'ipxe.efi' von aa:bb:cc:dd:ee:01   │
  │           — Datei nicht in data/uefi-01/tftp/            │
  │           Vorhandene Dateien: [snponly.efi, grub.efi]"  │
  └─────────────────────────────────────────────────────────┘

  Bei tftp_server (extern):
  ┌─────────────────────────────────────────────────────────┐
  │  Bootforge beantwortet KEINE TFTP-Anfrage für           │
  │  diesen Client — der DHCP-Proxy setzt                   │
  │  next-server = 10.0.0.200 statt eigene IP.              │
  │                                                          │
  │  Diagnostics trackt trotzdem: "Client sollte jetzt      │
  │  TFTP bei 10.0.0.200 anfragen..."                       │
  └─────────────────────────────────────────────────────────┘
```

---

## 7. Startup-Validierung

```
 ╔══════════════════════════════════════════════════════════════════╗
 ║  BOOTFORGE v2.0.0                                               ║
 ╚══════════════════════════════════════════════════════════════════╝

 ───────── Loading Config ─────────────────────────────────────────
 ✓ bootforge.toml                   parsed OK
 ✓ Interface                        eth0 → 10.0.0.10
 ✓ Ports                            :67 ✓  :4011 ✓  :69 ✓  :8080 ✓

 ───────── Loading Computer Profiles ──────────────────────────────
 ✓ default-bios.toml                1 client  (wildcard)
 ✓ default-uefi.toml                1 client  (wildcard)
 ✓ computer-raum-01.toml            24 clients
 ✓ computer-raum-02.toml            24 clients
 ✓ server-rack-a.toml               2 clients
 ✓ einzeln-spezial.toml             1 client  (extern)
                                     ──────────
                                     52 clients + 2 defaults geladen

 ───────── Validating Profiles ────────────────────────────────────
 ✓ MAC-Adressen                     52 unique, keine Duplikate
 ✓ HTTP-Pfade                       alle eindeutig
 ✓ type=auto Clients                [client.uefi] + [client.bios] vorhanden

 ───────── Checking Files ─────────────────────────────────────────
 ✓ data/uefi-01/tftp/ipxe.efi      984.2 KB  sha256:a3f8...
 ✓ data/bios-01/tftp/undionly.kpxe  67.1 KB   sha256:c912...
 ✓ data/ubuntu-24-PC/tftp/ipxe.efi  984.2 KB  sha256:a3f8...
 ✓ data/ubuntu-24-PC/http/vmlinuz   8.2 MB    sha256:44b1...
 ✓ data/ubuntu-24-PC/http/initrd    52.1 MB   sha256:e7c3...
 ✓ data/ubuntu-24-PC/http/preseed.. 2.4 KB    sha256:9a12...
 ✗ data/rescue/http/boot.ipxe       MISSING!
   └─ Betrifft: "legacy-box" (aa:bb:cc:dd:ee:02)
      HTTP-Pfad /rack-a/legacy/ wird 404 liefern!

 ───────── Extern-Referenzen (nicht prüfbar) ──────────────────────
 ⚠ einzeln-spezial.toml            tftp_server=10.0.0.200 (nicht testbar)
 ⚠ einzeln-spezial.toml            http_server=10.0.0.202 (nicht testbar)
 ⚠ server-rack-a/legacy-box        tftp_server=10.0.0.200 (nicht testbar)

 ───────── Result ─────────────────────────────────────────────────
 ⚠ 1 Fehler, 3 Warnungen — Starte mit Einschränkungen
   (--strict Flag würde hier abbrechen)
```

---

## 8. Übersicht: Was sich geändert hat (v1 → v2)

```
┌──────────────────────┬──────────────────────┬──────────────────────┐
│                      │ v1 (alt)             │ v2 (neu)             │
├──────────────────────┼──────────────────────┼──────────────────────┤
│ Config-Format        │ 1x großes YAML       │ 1x TOML (Server)    │
│                      │                      │ + N× TOML (Clients)  │
├──────────────────────┼──────────────────────┼──────────────────────┤
│ Client-Definition    │ Alles in einer Datei │ Pro Raum/Gruppe/     │
│                      │ unter mac_profiles   │ Zweck eine Datei     │
├──────────────────────┼──────────────────────┼──────────────────────┤
│ UEFI/BIOS            │ Global definiert     │ Pro Client separat   │
│                      │ (bootfiles.uefi_x64) │ [client.uefi/bios]   │
├──────────────────────┼──────────────────────┼──────────────────────┤
│ TFTP/HTTP Quelle     │ Immer intern         │ Intern ODER extern   │
│                      │                      │ pro Client wählbar   │
├──────────────────────┼──────────────────────┼──────────────────────┤
│ Boot-Dateien         │ Zentral unter        │ Pro Profil eigenes   │
│                      │ tftp/ und http/      │ data/*/tftp + http   │
├──────────────────────┼──────────────────────┼──────────────────────┤
│ Skalierung           │ 100+ MACs = riesige  │ 100+ MACs = mehrere  │
│                      │ unübersichtliche     │ kleine übersichtliche│
│                      │ YAML-Datei           │ TOML-Dateien         │
├──────────────────────┼──────────────────────┼──────────────────────┤
│ Nur DHCP-Proxy       │ Nicht vorgesehen     │ tftp_server +        │
│ (alles extern)       │                      │ http_server = nur    │
│                      │                      │ Proxy-Weiterleitung  │
└──────────────────────┴──────────────────────┴──────────────────────┘
```

---

## 9. Mögliche Erweiterung: MAC-Ranges statt Einzeleinträge

```toml
# Für große Räume: MAC-Range statt 24x copy-paste

[[client]]
mac_range = ["c6:c9:4b:45:bf:01", "c6:c9:4b:45:bf:18"]  # 01 bis 18 (hex)
name      = "raum01-pc{index}"                             # {index} = 01..24
type      = "uefi-only"

    [client.uefi]
    tftp_files = "data/ubuntu-24-PC/tftp/"
    bootfile   = "ipxe.efi"

    http_files = "data/ubuntu-24-PC/http/"
    http_path  = "/raum01/pc{index}/"

    [client.vars]
    hostname = "raum01-pc{index}"
    locale   = "de_DE.UTF-8"
    timezone = "Europe/Berlin"
    disk     = "/dev/sda"
    packages = "libreoffice,firefox,thunderbird"
```

---

## 10. Zusammenfassung der Architektur

```
  ┌──────────────────┐
  │  bootforge.toml  │  Globale Server-Config (Ports, Logging, Health)
  └────────┬─────────┘
           │ lädt
           ▼
  ┌──────────────────┐
  │  computers/*.toml│  N Dateien mit je M [[client]] Einträgen
  └────────┬─────────┘
           │ baut auf
           ▼
  ┌──────────────────────────────────────────────────────────────┐
  │                     MAC REGISTRY                             │
  │                                                              │
  │  MAC             │ Name         │ Type │ UEFI     │ BIOS    │
  │  ────────────────┼──────────────┼──────┼──────────┼─────────│
  │  c6:...:bf:01    │ raum01-pc01  │ uefi │ intern   │ —       │
  │  c6:...:bf:02    │ raum01-pc02  │ uefi │ intern   │ —       │
  │  aa:...:ee:01    │ db-master    │ auto │ intern   │ intern  │
  │  aa:...:ee:02    │ legacy-box   │ bios │ —        │ extern  │
  │  ff:...:bb:aa    │ test-bench   │ auto │ extern   │ extern  │
  │  * (uefi)        │ default-uefi │ uefi │ intern   │ —       │
  │  * (bios)        │ default-bios │ bios │ —        │ intern  │
  └──────────────────────────────────────────────────────────────┘
           │
           │ steuert
           ▼
  ┌────────────┐  ┌────────────┐  ┌────────────┐
  │ DHCP Proxy │  │ TFTP Server│  │ HTTP Server│
  │            │  │            │  │            │
  │ next-server│  │ Datei aus  │  │ Route aus  │
  │ = eigene IP│  │ tftp_files │  │ http_path  │
  │ oder       │  │ ODER       │  │ ODER       │
  │ tftp_server│  │ (nix, wenn │  │ Redirect   │
  │            │  │  extern)   │  │ zu extern  │
  └────────────┘  └────────────┘  └────────────┘
```
