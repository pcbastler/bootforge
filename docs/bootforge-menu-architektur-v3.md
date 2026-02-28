# 🔥 BOOTFORGE — Planung v3: Menü-basierte Architektur

---

## 1. Das Prinzip

```
  BISHER:
  Ein Client → ein festes Ziel (install ODER boot ODER iscsi)

  NEU:
  Ein Client → ein Menü aus N Optionen, individuell zusammengestellt

  ┌──────────────────────────────────────────────────────────────┐
  │                                                              │
  │  GLOBAL definiert: Was gibt es?                              │
  │  ───────────────────────────────                             │
  │  "ubuntu-install"    → Ubuntu 24 installieren                │
  │  "debian-install"    → Debian 12 installieren                │
  │  "win11-install"     → Windows 11 installieren               │
  │  "ubuntu-live"       → Ubuntu Live (RAM)                     │
  │  "rescue"            → Rescue-System starten                 │
  │  "memtest"           → RAM testen                            │
  │  "local-disk"        → Von Festplatte booten (PXE beenden)  │
  │                                                              │
  │  PRO CLIENT definiert: Was darf dieser Client?               │
  │  ─────────────────────────────────────────────               │
  │  raum01-pc01:  [local-disk, win11-install, rescue]           │
  │  raum01-pc02:  [local-disk, win11-install, rescue]           │
  │  labor-pc01:   [ubuntu-live, debian-install, memtest]        │
  │  kiosk-01:     [ubuntu-live]  ← nur eins = kein Menü,       │
  │                                  bootet direkt               │
  │                                                              │
  └──────────────────────────────────────────────────────────────┘
```

---

## 2. Drei Schichten: Bootloader → Menü → Client

```
  WICHTIG: Die Bootloader-Auswahl (UEFI/BIOS) passiert VOR dem Menü.
  iPXE abstrahiert UEFI/BIOS komplett — ab dem Menü ist es egal.
  Die Architektur-Erkennung erfolgt AUTOMATISCH via DHCP Option 93.

  Zeitlicher Ablauf:
  ═══════════════════════════════════════════════════════════════

  1. Client sendet DHCP DISCOVER (Option 93 = UEFI oder BIOS)
  2. Bootforge DHCP-Proxy: Option 93 lesen → Bootloader wählen
     0x0000 = BIOS      → undionly.kpxe
     0x0006 = UEFI x86  → ipxe-x86.efi
     0x0007 = UEFI x64  → ipxe.efi
     0x0009 = UEFI x64  → ipxe.efi
     0x000B = ARM64     → ipxe-arm64.efi
  3. Client lädt Bootloader via TFTP           ◄── [bootloader]
  4. iPXE läuft — ab hier ist UEFI/BIOS egal
  5. iPXE holt Menü-Script via HTTP            ◄── [[menu]]
  6. iPXE zeigt Menü                           ◄── [[client]].menu
  7. User wählt oder Timeout → Auto-Boot
  8. iPXE führt gewählte Aktion aus

  ═══════════════════════════════════════════════════════════════

  Drei Schichten, drei Aufgaben:

  ┌─────────────────┐     ┌────────────────────┐     ┌──────────────┐
  │ [bootloader]    │     │ [[menu]]           │     │ [[client]]   │
  │                 │     │                    │     │              │
  │ WIE kommt iPXE │────▶│ WAS kann iPXE tun  │◀────│ WER darf was │
  │ auf den Client? │     │ nach dem Start?    │     │              │
  │                 │     │                    │     │              │
  │ Auto-Detect     │     │ OS-agnostisch      │     │ MAC-basiert  │
  │ via Option 93   │     │ Schritt 5-8        │     │ Konfiguration│
  │ Schritt 1-4     │     │                    │     │              │
  └─────────────────┘     └────────────────────┘     └──────────────┘

  Die Menü-Einträge wissen NICHTS über UEFI/BIOS.
  Die Clients wissen NICHTS über Dateipfade.
  Der Bootloader weiß NICHTS über das Menü.
```

---

## 3. Was der Client sieht

```
  ┌──────────────────────────────────────────────────────────────┐
  │                                                              │
  │              BOOTFORGE — raum01-pc01                         │
  │              MAC: c6:c9:4b:45:bf:01                         │
  │                                                              │
  │     ┌───────────────────────────────────────────────┐       │
  │     │                                               │       │
  │     │  [1] ▶ Windows 11 installieren      (Standard)│       │
  │     │  [2]   Ubuntu 24 installieren                 │       │
  │     │  [3]   Rescue-System                          │       │
  │     │                                               │       │
  │     │  Automatischer Start: Windows 11 in 15s       │       │
  │     │                                               │       │
  │     └───────────────────────────────────────────────┘       │
  │                                                              │
  └──────────────────────────────────────────────────────────────┘

  Kiosk-01 sieht kein Menü — nur einen Eintrag = direkt booten:

  ┌──────────────────────────────────────────────────────────────┐
  │                                                              │
  │  BOOTFORGE — kiosk-01                                       │
  │  Starte Ubuntu Live...                                      │
  │                                                              │
  └──────────────────────────────────────────────────────────────┘
```

---

## 4. Konfiguration

### 4.1 Flexible Config-Struktur

```
  Bootforge parst ALLE .toml Dateien im Config-Verzeichnis
  und erkennt anhand des Inhalts, was was ist:

  ┌──────────────────────────────────────────────────────────────┐
  │                                                              │
  │  Enthält [server], [dhcp_proxy], [tftp], [http]?            │
  │    → Server-Konfiguration                                    │
  │                                                              │
  │  Enthält [bootloader]?                                       │
  │    → Bootloader-Konfiguration                                │
  │                                                              │
  │  Enthält [[menu]]?                                           │
  │    → Menü-Einträge                                           │
  │                                                              │
  │  Enthält [[client]]?                                         │
  │    → Client-Profile                                          │
  │                                                              │
  │  Eine Datei kann ALLES enthalten (alles-in-einem),          │
  │  oder man verteilt es auf beliebig viele Dateien.            │
  │                                                              │
  │  Beispiel: Alles in einer Datei:                             │
  │    /etc/bootforge/bootforge.toml                             │
  │                                                              │
  │  Beispiel: Aufgeteilt:                                       │
  │    /etc/bootforge/server.toml        [server] + Dienste      │
  │    /etc/bootforge/bootloader.toml    [bootloader]            │
  │    /etc/bootforge/menus.toml         [[menu]] Einträge       │
  │    /etc/bootforge/menus-tools.toml   [[menu]] weitere        │
  │    /etc/bootforge/raum-01.toml       [[client]] Einträge     │
  │    /etc/bootforge/raum-02.toml       [[client]] weitere      │
  │    /etc/bootforge/kiosk.toml         [[client]] weitere      │
  │                                                              │
  │  Bootforge ist es egal — es liest einfach alles und          │
  │  merged die Ergebnisse zusammen.                             │
  │                                                              │
  │  Validierung:                                                │
  │  ✗ Doppelte MAC-Adressen über Dateien hinweg                │
  │  ✗ Doppelte Menü-Namen über Dateien hinweg                  │
  │  ✗ Mehr als eine [server] Sektion                            │
  │  ✗ Client referenziert nicht-existierenden Menü-Eintrag     │
  │                                                              │
  └──────────────────────────────────────────────────────────────┘
```

### 4.2 Server-Config + Bootloader (`server.toml` oder `bootforge.toml`)

```toml
# /etc/bootforge/bootforge.toml
# (oder aufgeteilt — Bootforge ist es egal)

# ─── Server ───────────────────────────────────
[server]
interface = "eth0"
data_dir  = "/etc/bootforge/data"

[server.logging]
level  = "info"                      # trace | debug | info | warn | error
format = "pretty"                    # pretty | json

# ─── Dienste ──────────────────────────────────

[dhcp_proxy]
enabled      = true
port         = 67
proxy_port   = 4011
vendor_class = "PXEClient"

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

# ─── HTTP Caching Proxy ─────────────────────
# Optional: Bootforge kann als Caching Proxy vor
# einem externen HTTP-Server agieren. Boot-Dateien
# werden beim ersten Abruf gecacht.

[http.proxy]
enabled   = false
# upstream = "http://10.0.0.200:8080"
# cache_dir = "data/cache/"
# max_cache_size = "50GB"
# ttl = "24h"

# ─── Health & Diagnostics ─────────────────────

[health]
enabled       = true
interval      = "30s"
startup_check = true

[diagnostics]
enabled         = true
session_timeout = "10m"

# ═════════════════════════════════════════════════
#  BOOTLOADER
#  Global, VOR dem Menü. Der DHCP-Proxy wählt
#  automatisch basierend auf Option 93 des Clients.
#  Kein manuelles type-Feld nötig — reine Auto-Erkennung.
#  Diese Dateien werden via TFTP ausgeliefert.
# ═════════════════════════════════════════════════

[bootloader]
dir      = "data/bootloader/"         # Verzeichnis mit allen Bootloadern
uefi_x64 = "ipxe.efi"                # Option 93 = 0x0007 / 0x0009
uefi_x86 = "ipxe-x86.efi"            # Option 93 = 0x0006
bios     = "undionly.kpxe"            # Option 93 = 0x0000
arm64    = "ipxe-arm64.efi"           # Option 93 = 0x000B

# iPXE Chain-URL: Was iPXE nach dem Start als erstes lädt.
# ${mac} wird durch die MAC-Adresse des Clients ersetzt.
chain_url = "http://${server_ip}:${http_port}/boot/${mac}/menu.ipxe"
```

### 4.3 Menü-Einträge (in beliebiger .toml Datei)

```toml
# /etc/bootforge/menus.toml
# (oder in bootforge.toml, oder aufgeteilt auf mehrere Dateien)

# ═════════════════════════════════════════════════
#  MENÜ-EINTRÄGE
#  Jeder Eintrag ist eine boot-bare Aktion.
#  Clients referenzieren diese per Name.
#  KEINE Bootloader/TFTP-Angaben hier — das ist
#  Aufgabe der [bootloader] Sektion.
# ═════════════════════════════════════════════════

# ─── INSTALL: Betriebssystem installieren ─────

[[menu]]
name        = "ubuntu-install"
label       = "Ubuntu 24.04 installieren"
description = "Installiert Ubuntu auf die lokale Festplatte"
type        = "install"

    [menu.http]
    files = "data/installers/ubuntu/"
    path  = "/install/ubuntu/"

    [menu.boot]
    kernel  = "vmlinuz"
    initrd  = "initrd"
    cmdline = "ip=dhcp auto=true url=http://${server_ip}:${http_port}/install/ubuntu/preseed.cfg"

[[menu]]
name        = "debian-install"
label       = "Debian 12 installieren"
description = "Installiert Debian auf die lokale Festplatte"
type        = "install"

    [menu.http]
    files = "data/installers/debian/"
    path  = "/install/debian/"

    [menu.boot]
    kernel  = "vmlinuz"
    initrd  = "initrd.gz"
    cmdline = "ip=dhcp auto=true url=http://${server_ip}:${http_port}/install/debian/preseed.cfg"

[[menu]]
name        = "win11-install"
label       = "Windows 11 installieren"
description = "Installiert Windows 11 auf die lokale Festplatte"
type        = "install"

    [menu.http]
    files = "data/installers/win11/"
    path  = "/install/win11/"

    [menu.boot]
    loader  = "wimboot"              # Spezieller Loader für Windows
    files   = ["boot.sdi", "BCD", "boot.wim"]

# ─── LIVE: Im RAM ──────────────────────────────

[[menu]]
name        = "ubuntu-live"
label       = "Ubuntu Live (RAM)"
description = "Startet Ubuntu komplett im RAM, nichts wird gespeichert"
type        = "live"

    [menu.http]
    files = "data/live/ubuntu/"
    path  = "/live/ubuntu/"

    [menu.boot]
    kernel  = "vmlinuz"
    initrd  = "initrd.img"
    image   = "ubuntu-desktop.squashfs"
    cmdline = "ip=dhcp boot=live toram"

# ─── TOOLS ────────────────────────────────────

[[menu]]
name        = "rescue"
label       = "Rescue-System"
description = "Minimales Linux zum Debuggen und Reparieren"
type        = "live"

    [menu.http]
    files = "data/tools/rescue/"
    path  = "/tools/rescue/"

    [menu.boot]
    kernel  = "vmlinuz"
    initrd  = "initrd"
    cmdline = "ip=dhcp rescue"

[[menu]]
name        = "memtest"
label       = "Memtest86+"
description = "Arbeitsspeicher testen"
type        = "tool"

    # Memtest wird als EFI-Binary direkt via iPXE geladen
    # iPXE: chain http://server/tools/memtest/memtest.efi
    [menu.http]
    files = "data/tools/memtest/"
    path  = "/tools/memtest/"

    [menu.boot]
    binary = "memtest.efi"

# ─── EXIT: Lokale Festplatte booten ─────────

[[menu]]
name        = "local-disk"
label       = "Von Festplatte booten"
description = "Beendet PXE und bootet von der lokalen Festplatte"
type        = "exit"

    # Kein [menu.http], kein [menu.boot] — iPXE führt nur "exit" aus.
    # UEFI/BIOS bootet dann das nächste Gerät (lokale Platte).
```

### 4.4 Client-Profile — nur noch Referenzen

```toml
# /etc/bootforge/raum-01.toml
# ═══════════════════════════════════════════════
#  Computerraum 01 — 24 PCs
#  Windows als Standard, Ubuntu als Option
# ═══════════════════════════════════════════════

# Kein "type" Feld — Bootforge erkennt UEFI/BIOS
# automatisch via DHCP Option 93 bei jedem Boot.

[[client]]
mac  = "c6:c9:4b:45:bf:01"
name = "raum01-pc01"

    # ─── Das Menü für diesen Client ──────────
    [client.menu]
    entries = ["local-disk", "win11-install", "ubuntu-install", "rescue"]
    default = "local-disk"
    timeout = 5                      # Sekunden bis Auto-Boot

    # ─── Variablen (für Templates) ───────────
    [client.vars]
    hostname = "raum01-pc01"
    locale   = "de_DE.UTF-8"
    timezone = "Europe/Berlin"


[[client]]
mac  = "c6:c9:4b:45:bf:02"
name = "raum01-pc02"

    [client.menu]
    entries = ["local-disk", "win11-install", "ubuntu-install", "rescue"]
    default = "local-disk"
    timeout = 5

    [client.vars]
    hostname = "raum01-pc02"
    locale   = "de_DE.UTF-8"
    timezone = "Europe/Berlin"
```

### 4.5 Minimalste Config — 1 Client, 1 Eintrag

```toml
# /etc/bootforge/kiosk.toml

[[client]]
mac  = "aa:bb:cc:dd:ee:01"
name = "kiosk-01"

    [client.menu]
    entries = ["ubuntu-live"]
    # Kein default nötig — nur 1 Eintrag = kein Menü
    # Kein timeout nötig — bootet sofort
```

### 4.6 Viel Auswahl — Labor-PC

```toml
# /etc/bootforge/labor.toml

[[client]]
mac  = "dd:ee:ff:00:11:01"
name = "labor-pc01"

    [client.menu]
    entries = [
        "win11-install",
        "ubuntu-install",
        "debian-install",
        "ubuntu-live",
        "rescue",
        "memtest",
    ]
    default = "ubuntu-live"
    timeout = 30

    [client.vars]
    hostname = "labor-pc01"
```

### 4.7 Default-Clients (Fallback für unbekannte MACs)

```toml
# /etc/bootforge/defaults.toml

# Unbekannte Clients bekommen ein eingeschränktes Menü.
# mac = "*" ist der Wildcard — greift wenn keine exakte MAC passt.

[[client]]
mac  = "*"
name = "unknown"

    [client.menu]
    entries = ["rescue", "memtest"]
    default = "rescue"
    timeout = 30
```

---

## 5. Was daraus generiert wird

### 5.1 iPXE Boot-Menü (automatisch generiert)

```
  Bootforge generiert für jeden Client ein individuelles
  iPXE-Script basierend auf client.menu.entries.

  Der Bootloader (ipxe.efi/undionly.kpxe) ist bereits geladen
  und ruft dieses Script via HTTP auf (chain_url aus [bootloader]):

  ┌──────────────────────────────────────────────────────────────┐
  │  GET /boot/c6:c9:4b:45:bf:01/menu.ipxe                     │
  │                                                              │
  │  #!ipxe                                                      │
  │                                                              │
  │  :start                                                      │
  │  menu BOOTFORGE - raum01-pc01 (c6:c9:4b:45:bf:01)           │
  │  item --default local-disk --timeout 5000                    │
  │  item local-disk     Von Festplatte booten                   │
  │  item win11-install  Windows 11 installieren                 │
  │  item ubuntu-install Ubuntu 24.04 installieren               │
  │  item rescue         Rescue-System                           │
  │  choose selected && goto ${selected}                         │
  │                                                              │
  │  :local-disk                                                 │
  │  exit                                                        │
  │                                                              │
  │  :win11-install                                              │
  │  kernel http://10.0.0.10:8080/install/win11/wimboot          │
  │  initrd http://10.0.0.10:8080/install/win11/boot.sdi        │
  │  initrd http://10.0.0.10:8080/install/win11/BCD             │
  │  initrd http://10.0.0.10:8080/install/win11/boot.wim        │
  │  boot                                                        │
  │                                                              │
  │  :ubuntu-install                                             │
  │  kernel http://10.0.0.10:8080/install/ubuntu/vmlinuz ...     │
  │  initrd http://10.0.0.10:8080/install/ubuntu/initrd          │
  │  boot                                                        │
  │                                                              │
  │  :rescue                                                     │
  │  kernel http://10.0.0.10:8080/tools/rescue/vmlinuz ...       │
  │  initrd http://10.0.0.10:8080/tools/rescue/initrd            │
  │  boot                                                        │
  │                                                              │
  └──────────────────────────────────────────────────────────────┘

  Für kiosk-01 (nur 1 Eintrag, kein Menü):

  ┌──────────────────────────────────────────────────────────────┐
  │  GET /boot/aa:bb:cc:dd:ee:01/menu.ipxe                      │
  │                                                              │
  │  #!ipxe                                                      │
  │  kernel http://10.0.0.10:8080/live/ubuntu/vmlinuz            │
  │    ip=dhcp boot=live toram                                   │
  │  initrd http://10.0.0.10:8080/live/ubuntu/initrd.img         │
  │  initrd http://10.0.0.10:8080/live/ubuntu/desktop.squashfs   │
  │  boot                                                        │
  │                                                              │
  └──────────────────────────────────────────────────────────────┘
```

---

## 6. Wake-on-LAN mit gezieltem Boot

```
  Bootforge kann einen Client per WoL aufwecken UND gleichzeitig
  einen bestimmten Menü-Eintrag für den nächsten Boot erzwingen.

  Das funktioniert, weil Bootforge BEIDE Seiten kontrolliert:
  den WoL-Sender UND den iPXE-Script-Generator.

  ═══════════════════════════════════════════════════════════════

  CLI:
  $ bootforge client wake c6:c9:4b:45:bf:01 --boot ubuntu-install

  Web-UI:
  Button "Aufwecken & Booten" → Dropdown mit Menü-Einträgen

  API:
  POST /api/v1/clients/c6:c9:4b:45:bf:01/wake
  { "boot": "ubuntu-install" }

  ═══════════════════════════════════════════════════════════════

  Ablauf:

  ┌────────────────────────────────────────────────────────────┐
  │                                                            │
  │  1. Bootforge setzt One-Time Boot Override im Registry:   │
  │     MAC c6:...:01 → nächster Boot: "ubuntu-install"       │
  │     (nur im Speicher, kein Config-Change)                  │
  │                                                            │
  │  2. Bootforge sendet WoL Magic Packet                     │
  │     → Rechner wacht auf                                    │
  │                                                            │
  │  3. Rechner bootet PXE → DHCP → TFTP → iPXE              │
  │                                                            │
  │  4. iPXE: GET /boot/c6:...:01/menu.ipxe                   │
  │     Bootforge sieht den Override für diese MAC             │
  │     → Generiert KEIN Menü, sondern bootet direkt:         │
  │                                                            │
  │     #!ipxe                                                 │
  │     kernel http://10.0.0.10:8080/install/ubuntu/vmlinuz   │
  │       ip=dhcp auto=true url=...preseed.cfg                │
  │     initrd http://10.0.0.10:8080/install/ubuntu/initrd     │
  │     boot                                                   │
  │                                                            │
  │  5. Override wird gelöscht (einmalig)                     │
  │     → Nächster Boot zeigt wieder das normale Menü          │
  │                                                            │
  └────────────────────────────────────────────────────────────┘

  Ohne --boot:
  $ bootforge client wake c6:c9:4b:45:bf:01
  → Kein Override, Rechner bekommt sein normales Menü.

  Validierung:
  ✗ --boot Eintrag muss im client.menu.entries des Clients sein
  ✗ --boot Eintrag muss ein gültiger [[menu]] Name sein
  → Fehler: "ubuntu-install ist nicht im Menü von raum01-pc01"

  Bulk-Wake (alle Clients einer Datei):
  $ bootforge client wake --file raum-01.toml --boot win11-install
  → Weckt alle 24 PCs und installiert Windows auf allen.
```

---

## 7. HTTP Caching Proxy

```
  ┌──────────────────────────────────────────────────────────────┐
  │                                                              │
  │  Standardmäßig: Bootforge liefert Boot-Dateien direkt aus   │
  │  dem lokalen Dateisystem aus (data/).                        │
  │                                                              │
  │  Optional: Bootforge als Caching Proxy                      │
  │  ─────────────────────────────────────                       │
  │  Wenn [http.proxy] aktiviert ist, kann Bootforge            │
  │  Boot-Dateien von einem externen HTTP-Server cachen.        │
  │                                                              │
  │  Anwendungsfall:                                             │
  │  • Zentraler Image-Server im Rechenzentrum                  │
  │  • Bootforge in der Außenstelle cached die Dateien lokal    │
  │  • Erster Abruf: upstream fetch + cache                     │
  │  • Weitere Abrufe: direkt aus dem Cache                     │
  │                                                              │
  │  ┌──────────┐         ┌──────────────┐         ┌─────────┐ │
  │  │  Client   │──HTTP──▶│  Bootforge   │──HTTP──▶│ Upstream│ │
  │  │ (iPXE)   │         │  (Cache)     │         │ Server  │ │
  │  │          │◀────────│              │◀────────│         │ │
  │  └──────────┘  Datei   └──────────────┘  Datei  └─────────┘ │
  │                aus                        nur beim           │
  │                Cache                      ersten Mal         │
  │                                                              │
  │  Menü-Eintrag mit Proxy:                                    │
  │  Ein [[menu]] Eintrag kann statt lokaler Dateien            │
  │  eine upstream URL referenzieren:                            │
  │                                                              │
  │  [menu.http]                                                 │
  │  upstream = "http://images.intern/ubuntu-24/"               │
  │  path     = "/install/ubuntu/"                               │
  │  # → Bootforge cached die Dateien beim ersten Abruf        │
  │  # → Danach lokal, ohne upstream-Kontakt                    │
  │                                                              │
  │  TFTP ist davon NICHT betroffen — Bootloader sind immer     │
  │  lokal, nie geproxied.                                       │
  │                                                              │
  └──────────────────────────────────────────────────────────────┘
```

---

## 7. Gesamtbild: Wie alles zusammenhängt

```
  ┌─────────────────────┐
  │ *.toml Dateien      │  Bootforge liest ALLE .toml
  │                     │  im Config-Verzeichnis und
  │  [server]           │  merged sie zusammen.
  │  [dhcp_proxy]       │
  │  [tftp]             │
  │  [http]             │
  │                     │
  │  [bootloader] ──────│──┐ Schicht 1: UEFI/BIOS Bootloader
  │   uefi_x64, bios..  │  │ (VOR dem Menü, via TFTP)
  │                     │  │ Auto-Detect via Option 93
  │  [[menu]]  ─────────│──│──┐ Schicht 2: Boot-Optionen
  │  [[menu]]           │  │  │ (NACH iPXE-Start, via HTTP)
  │  [[menu]]           │  │  │
  │                     │  │  │
  │  [[client]]         │  │  │ Schicht 3: Wer darf was
  │    mac = "..."    ──│──┘  │
  │                     │     │
  │    menu.entries = [ │─────┘ → bestimmt welche Menü-Einträge
  │      "win11-inst",  │
  │      "rescue",      │
  │    ]                │
  │    menu.default     │──── Was bei Timeout bootet
  │    menu.timeout     │──── Sekunden bis Auto-Boot
  │                     │
  │    [client.vars]    │──── Template-Variablen
  │                     │
  └──────────┬──────────┘
             │
             │ generiert automatisch
             ▼
  ┌──────────────────────────────────────────────────────────┐
  │                                                          │
  │  Pro Client automatisch erstellt:                        │
  │                                                          │
  │  1. DHCP-Proxy Antwort                                  │
  │     → Bootloader-Auswahl via Option 93 (Auto-Detect)   │
  │     → next-server + bootfile setzen                     │
  │                                                          │
  │  2. iPXE Boot-Menü (individuell pro Client)             │
  │     → Nur die entries die der Client haben darf          │
  │     → Mit Default + Timeout                              │
  │     → Dynamisch generiert bei HTTP-Request              │
  │                                                          │
  │  3. HTTP-Routen (für jeden Menü-Eintrag)               │
  │     → Kernel, Initrd, Squashfs, Preseed erreichbar      │
  │     → Optional: Caching Proxy vor Upstream-Server       │
  │                                                          │
  └──────────────────────────────────────────────────────────┘
```

---

## 8. Verzeichnisstruktur (final)

```
/etc/bootforge/
├── bootforge.toml                         # Alles-in-einem ODER aufgeteilt:
├── server.toml                            # ← optional: nur [server] + Dienste
├── menus.toml                             # ← optional: nur [[menu]] Einträge
├── raum-01.toml                           # ← optional: nur [[client]] Einträge
├── kiosk.toml                             # ← optional: weitere [[client]]
├── defaults.toml                          # ← optional: Wildcard-Client (mac="*")
│
│   Bootforge liest ALLE .toml Dateien im Verzeichnis.
│   Unterverzeichnisse werden NICHT rekursiv durchsucht.
│   Die Dateinamen sind egal — nur der Inhalt zählt.
│
└── data/
    │
    ├── bootloader/                        # [bootloader] — Global, nur TFTP
    │   ├── ipxe.efi                       #   UEFI x64
    │   ├── ipxe-x86.efi                  #   UEFI x86
    │   ├── ipxe-arm64.efi                #   ARM64
    │   └── undionly.kpxe                  #   BIOS
    │
    ├── installers/                        # [[menu]] type = "install"
    │   ├── ubuntu/                        #   (nur HTTP-Dateien, kein TFTP)
    │   │   ├── vmlinuz
    │   │   ├── initrd
    │   │   └── preseed.cfg
    │   ├── debian/
    │   │   ├── vmlinuz
    │   │   ├── initrd.gz
    │   │   └── preseed.cfg
    │   └── win11/
    │       ├── wimboot
    │       ├── boot.sdi
    │       ├── BCD
    │       └── boot.wim
    │
    ├── live/                              # [[menu]] type = "live"
    │   └── ubuntu/                        #   (nur HTTP-Dateien, kein TFTP)
    │       ├── vmlinuz
    │       ├── initrd.img
    │       └── ubuntu-desktop.squashfs
    │
    ├── tools/                             # [[menu]] type = "live" / "tool"
    │   ├── rescue/                        #   (nur HTTP-Dateien, kein TFTP)
    │   │   ├── vmlinuz
    │   │   └── initrd
    │   └── memtest/
    │       └── memtest.efi
    │
    └── cache/                             # [http.proxy] — Caching Proxy
        └── ...                            #   Auto-managed, kann gelöscht werden

  ══════════════════════════════════════════════════════════════
  WICHTIG: Klare Trennung von TFTP und HTTP
  ══════════════════════════════════════════════════════════════

  TFTP liefert NUR aus:     data/bootloader/
                            → 4 Dateien (iPXE Bootloader)
                            → Ändern sich fast nie
                            → Immer lokal, nie proxied

  HTTP liefert aus:         data/installers/
                            data/live/
                            data/tools/
                            → Kernel, Initrd, Squashfs, Preseed
                            → Plus dynamische iPXE Menü-Scripts
                            → Optional via Caching Proxy

  Es gibt KEINE tftp/ Unterordner in installers/live/tools.
  Bootloader sind global — nicht pro Menü-Eintrag.
  ══════════════════════════════════════════════════════════════
```

---

## 9. Startup-Validierung (überarbeitet)

```
 ╔══════════════════════════════════════════════════════════════════╗
 ║  BOOTFORGE v3.0.0                                               ║
 ╚══════════════════════════════════════════════════════════════════╝

 ───────── Config ─────────────────────────────────────────────────
 ✓ bootforge.toml               [server] [dhcp_proxy] [tftp] [http]
 ✓ bootforge.toml               [bootloader]
 ✓ menus.toml                   6 [[menu]] Einträge
 ✓ raum-01.toml                 24 [[client]] Einträge
 ✓ labor.toml                   4 [[client]] Einträge
 ✓ kiosk.toml                   6 [[client]] Einträge
 ✓ defaults.toml                1 [[client]] Eintrag (wildcard)
                                 ────────
                                 7 Dateien geladen, 0 Fehler

 ───────── Server ───────────────────────────────────────────────
 ✓ Interface                    ens18 → 10.0.0.10
 ✓ Ports                        :67 ✓ :4011 ✓ :69 ✓ :8080 ✓

 ───────── Bootloader (TFTP) ─────────────────────────────────────
 ✓ uefi_x64  ipxe.efi              984.2 KB  sha256:a3f8..
 ✓ uefi_x86  ipxe-x86.efi          921.0 KB  sha256:d1c4..
 ✓ bios      undionly.kpxe           67.1 KB  sha256:c912..
 ✓ arm64     ipxe-arm64.efi        1012.8 KB  sha256:f891..
 ✓ Chain-URL http://10.0.0.10:8080/boot/${mac}/menu.ipxe

 ───────── Menü-Einträge ─────────────────────────────────────────
 ✓ ubuntu-install    install   Dateien: ✓ vmlinuz ✓ initrd ✓ preseed
 ✓ debian-install    install   Dateien: ✓ vmlinuz ✓ initrd ✓ preseed
 ✓ win11-install     install   Dateien: ✓ wimboot ✓ boot.sdi ✓ BCD ✓ boot.wim
 ✓ ubuntu-live       live      Dateien: ✓ vmlinuz ✓ initrd ✓ squashfs (2.1 GB)
 ✓ rescue            live      Dateien: ✓ vmlinuz ✓ initrd
 ✓ memtest           tool      Dateien: ✓ memtest.efi
 ✓ local-disk        exit      (keine Dateien — iPXE exit)
                     ────────
                     7 Menü-Einträge geladen, alle Dateien vorhanden

 ───────── Clients ────────────────────────────────────────────────
 ✓ raum-01.toml                 24 clients
   └─ Menü: local-disk (default), win11-install, ubuntu-install, rescue
 ✓ labor.toml                   4 clients
   └─ Menü: ubuntu-install, debian-install, ubuntu-live,
            rescue, memtest
 ✓ kiosk.toml                   6 clients
   └─ Menü: ubuntu-live (direkt-boot, kein Menü)
 ✓ defaults.toml                1 wildcard (mac="*")
   └─ Menü: rescue, memtest
                                 ────────
                                 34 clients + 1 default
                                 Alle Menü-Referenzen gültig

 ───────── HTTP Proxy ────────────────────────────────────────────
 ○ Caching Proxy               deaktiviert

 ───────── Dienste ────────────────────────────────────────────────
 ✓ DHCP Proxy   :67 + :4011   (Auto-Detect via Option 93)
 ✓ TFTP         :69            (nur Bootloader, 4 Dateien)
 ✓ HTTP         :8080          (Menü-Scripts + Boot-Dateien, 8 Routen)
 ✓ Health       interval: 30s

 ───────── Self-Test ──────────────────────────────────────────────
 ✓ DHCP Probe       ProxyOffer in 2ms (UEFI: ipxe.efi, BIOS: undionly.kpxe)
 ✓ TFTP Probe       ipxe.efi OK (984KB, 12ms)
 ✓ TFTP Probe       undionly.kpxe OK (67KB, 8ms)
 ✓ HTTP Probe       /healthz → 200 in 1ms
 ✓ HTTP Probe       /boot/test/menu.ipxe → generiert OK
 ✓ File Integrity   Alle Checksummen OK

 ═══════════════════════════════════════════════════════════════════
   BOOTFORGE READY — 10.0.0.10
   Bootloader: 4 Architekturen │ 7 Menü-Einträge │ 34 Clients
 ═══════════════════════════════════════════════════════════════════
```

---

## 10. CLI Erweiterung: Menü + Bootloader-Verwaltung

```
  bootforge
  │
  ├── bootloader                       Bootloader verwalten
  │   ├── list                         Alle Bootloader + Arch anzeigen
  │   ├── check                        Dateien prüfen + Checksummen
  │   └── download                     iPXE Bootloader herunterladen
  │
  ├── menu                             Menü-Einträge verwalten
  │   ├── list                         Alle Einträge anzeigen
  │   ├── show <name>                  Details eines Eintrags
  │   ├── validate                     Alle Dateien prüfen
  │   └── used-by <name>              Welche Clients nutzen diesen Eintrag?
  │
  ├── client
  │   ├── add
  │   │   └── --menu "a,b,c"          Menü-Einträge zuweisen
  │   │
  │   ├── menu <mac>                   Menü eines Clients anzeigen
  │   ├── menu-add <mac> <entry>       Eintrag hinzufügen
  │   ├── menu-remove <mac> <entry>    Eintrag entfernen
  │   └── menu-set-default <mac> <e>   Standard ändern
  │
  ...
```

---

## 11. Vorteile dieser Architektur

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                  │
│  1. KLARE TRENNUNG der drei Schichten                           │
│     → [bootloader]: Wie kommt iPXE auf den Client (Auto-Detect)│
│     → [[menu]]: Was kann iPXE tun (OS-agnostisch)               │
│     → [[client]]: Wer darf was (MAC-basiert)                    │
│     → Keine Vermischung, keine Redundanz                        │
│                                                                  │
│  2. TFTP ist minimal                                            │
│     → Nur 4 Dateien (die Bootloader), sonst nichts              │
│     → Alles andere geht über HTTP (schneller, caching-fähig)    │
│     → Bootloader ändern sich fast nie                            │
│                                                                  │
│  3. TRENNUNG: Was es gibt vs. Wer es bekommt                    │
│     → Menü-Einträge einmal definieren, beliebig oft zuweisen   │
│     → Neues OS? Einen [[menu]] Eintrag, 34 Clients haben es   │
│                                                                  │
│  4. SICHERHEIT: Client sieht nur was er darf                    │
│     → Kiosk sieht nur "Ubuntu Live", kann nichts installieren  │
│     → Labor sieht alles, kann experimentieren                   │
│                                                                  │
│  5. EINFACHHEIT: Client-Config ist minimal                      │
│     → MAC + Menü-Liste + optional Vars                          │
│     → Keine Pfade, keine Bootloader, kein UEFI/BIOS            │
│     → Pfade kommen vom Menü-Eintrag                            │
│     → Bootloader kommt von [bootloader]                         │
│                                                                  │
│  6. FLEXIBILITÄT: Mischbetrieb selbstverständlich               │
│     → Ein Client kann verschiedene OS installieren              │
│     → Windows UND Linux auf dem gleichen PC                     │
│     → Installer UND Live UND Tools                              │
│                                                                  │
│  7. KONSISTENZ: Gleicher Menü-Eintrag = gleiche Dateien        │
│     → "ubuntu-install" ist überall identisch                    │
│     → Update an einer Stelle, alle Clients betroffen            │
│                                                                  │
│  8. SKALIERUNG: Client hinzufügen = 5 Zeilen                   │
│     → MAC, Name, Menü-Liste — fertig                           │
│     → Kein Pfade-Copy-Paste mehr                               │
│                                                                  │
│  9. FLEXIBLE CONFIG: Struktur nach Wunsch                       │
│     → Alles in einer Datei oder aufgeteilt                      │
│     → Pro Raum eine Datei, pro Team, pro Zweck                 │
│     → Bootforge ist es egal — es liest alles                   │
│                                                                  │
│ 10. ERWEITERBAR: iSCSI als Phase 2                              │
│     → Architektur ist vorbereitet (menu.type = "iscsi")        │
│     → Client-Disks, Base+Overlay, iSCSI-Dienst                 │
│     → Kann später hinzugefügt werden ohne Umbau                │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

---

## 12. Roadmap

```
  Phase 1 (MVP):
  ─────────────
  • DHCP Proxy (Option 93 Auto-Detect)
  • TFTP (Bootloader)
  • HTTP (Menü-Scripts + Boot-Dateien)
  • iPXE Menü-Generierung
  • CLI (client, menu, bootloader, validate, serve)
  • Config: Flexible TOML-Parsing
  • Health / Self-Tests
  • menu.type: install, live, tool, exit

  Phase 2 (iSCSI):
  ────────────────
  • iSCSI Target-Dienst (:3260)
  • menu.type: iscsi (Diskless Boot)
  • Base-Images + Copy-on-Write Overlays
  • client.disk (dedizierte Daten-Disks)
  • menu.type: action (z.B. "new-disk" → Target erstellen)
  • Overlay-Reset (Zurücksetzen auf Base-Image)

  Phase 3 (Web-UI):
  ─────────────────
  • REST API + WebSocket
  • Web-Dashboard (Status, Sessions, Logs)
  • Client-Verwaltung im Browser
  • Datei-Upload (Boot-Dateien)
  • HTTP Caching Proxy
```
