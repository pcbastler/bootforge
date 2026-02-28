# 🔥 BOOTFORGE — Planung: CLI & Web-Interface

---

## 1. Grundsatzfrage: Architektur der Interaktion

```
  Wie redet der Admin mit Bootforge?

  Option A: CLI spricht direkt mit Config-Dateien
  Option B: CLI spricht mit einem laufenden Server (API)
  Option C: Beides — je nach Kontext

  ════════════════════════════════════════════════════

  Antwort: Option C — und hier ist warum:

  ┌──────────────────────────────────────────────────────────────────┐
  │                                                                  │
  │  Es gibt zwei Kategorien von Aktionen:                          │
  │                                                                  │
  │  OFFLINE-Aktionen (Server muss NICHT laufen)                    │
  │  ─────────────────────────────────────────────                   │
  │  • bootforge init             Config-Gerüst erstellen            │
  │  • bootforge validate         Config prüfen ohne zu starten      │
  │  • bootforge client add       Neuen Client anlegen               │
  │  • bootforge client list      Clients aus Config lesen           │
  │  • bootforge download-ipxe    Bootloader herunterladen           │
  │  • bootforge config show      Aktuelle Config anzeigen           │
  │  • bootforge config edit      Config-Wert ändern                 │
  │                                                                  │
  │  → Diese arbeiten direkt auf den TOML-Dateien                   │
  │  → Kein Server nötig, kein Socket, kein API                     │
  │  → Funktioniert auch auf einem Laptop ohne Netzwerk             │
  │                                                                  │
  │  ONLINE-Aktionen (Server MUSS laufen)                           │
  │  ─────────────────────────────────────────────                   │
  │  • bootforge status           Live-Status aller Dienste          │
  │  • bootforge sessions         Aktive Boot-Sessions               │
  │  • bootforge test             Self-Test jetzt auslösen           │
  │  • bootforge test --history   Letzte Self-Test-Ergebnisse        │
  │  • bootforge reload           Config neu laden (ohne Neustart)   │
  │  • bootforge restart          Server komplett neustarten          │
  │  • bootforge logs             Live-Logs streamen                 │
  │  • bootforge client wake      WoL an Client senden               │
  │                                                                  │
  │  → Diese sprechen mit dem laufenden Server via Unix-Socket       │
  │  → Server nicht erreichbar? Klare Fehlermeldung:                │
  │    "Bootforge Server läuft nicht. Starte mit: bootforge serve"  │
  │                                                                  │
  └──────────────────────────────────────────────────────────────────┘
```

---

## 2. Kommunikation CLI ↔ Server

```
  ┌──────────┐         ┌──────────────────────────────────────┐
  │          │  Unix   │           BOOTFORGE SERVER            │
  │   CLI    │─Socket──│                                      │
  │          │         │  ┌──────────────┐                    │
  │  oder    │         │  │ Control API  │ (intern, nur       │
  │          │  HTTP   │  │              │  localhost/socket)  │
  │  Web UI  │─:9090───│  │ /api/v1/*    │                    │
  │          │         │  └──────┬───────┘                    │
  └──────────┘         │         │                            │
                       │         ▼                            │
                       │  ┌──────────────┐  ┌──────────────┐ │
                       │  │ Config       │  │ Session      │ │
                       │  │ Manager      │  │ Store        │ │
                       │  └──────────────┘  └──────────────┘ │
                       │                                      │
                       │  ┌────────┐ ┌──────┐ ┌────────────┐│
                       │  │ DHCP   │ │ TFTP │ │ HTTP       ││
                       │  │ Proxy  │ │      │ │ (Boot)     ││
                       │  │ :67    │ │ :69  │ │ :8080      ││
                       │  └────────┘ └──────┘ └────────────┘│
                       └──────────────────────────────────────┘

  Warum Unix-Socket + HTTP?
  ┌────────────────────────────────────────────────────────────┐
  │                                                            │
  │  Unix-Socket (/run/bootforge.sock):                       │
  │  → Schnell, sicher, keine Port-Konflikte                  │
  │  → CLI auf dem gleichen Server — Standard-Weg             │
  │  → Keine Authentifizierung nötig (Filesystem-Permissions) │
  │                                                            │
  │  HTTP API (127.0.0.1:9090):                               │
  │  → Web-UI braucht HTTP                                    │
  │  → Optional auch von Remote erreichbar (0.0.0.0:9090)     │
  │  → Dann mit Auth-Token / Basic-Auth absichern             │
  │                                                            │
  │  Beide sprechen die GLEICHE API — nur anderer Transport   │
  │                                                            │
  └────────────────────────────────────────────────────────────┘
```

---

## 3. Ein Binary — alles drin

```
  ┌──────────────────────────────────────────────────────────────┐
  │                                                              │
  │  $ bootforge                                                 │
  │                                                              │
  │  Ein einziges Binary, Verhalten bestimmt durch Subcommand:  │
  │                                                              │
  │  bootforge serve          ← Startet den Server              │
  │  bootforge <command>      ← CLI-Tool                        │
  │                                                              │
  │  Kein separates Binary, keine Installation von zwei Paketen │
  │  Kein "bootforgectl" oder "bootforge-cli"                   │
  │                                                              │
  │  Wie bei:                                                    │
  │  • docker (client + daemon in einem Binary)                 │
  │  • consul (agent + cli)                                     │
  │  • nomad  (server + client + cli)                           │
  │                                                              │
  └──────────────────────────────────────────────────────────────┘
```

---

## 4. CLI Kommando-Struktur

```
  bootforge
  │
  ├── serve                          Startet den Server (Vordergrund)
  │   ├── --config, -c <path>        Config-Pfad (default: /etc/bootforge/)
  │   ├── --strict                   Bei jeder Warnung abbrechen
  │   ├── --dry-run                  Alles prüfen, aber nicht starten
  │   └── --debug                    Log-Level auf debug setzen
  │
  ├── init                           Erstellt Beispiel-Konfiguration
  │   ├── --dir <path>               Zielverzeichnis
  │   └── --minimal                  Nur das Nötigste
  │
  ├── validate                       Prüft Config ohne Server zu starten
  │   └── --config, -c <path>
  │
  ├── status                         ● Live-Status (braucht Server)
  │   ├── --watch, -w                Kontinuierlich aktualisieren
  │   └── --json                     Maschinenlesbar
  │
  ├── test                           ● Self-Test auslösen
  │   ├── --all                      Alle Tests
  │   ├── --dhcp                     Nur DHCP
  │   ├── --tftp                     Nur TFTP
  │   ├── --http                     Nur HTTP
  │   └── --history, -h              Letzte N Ergebnisse anzeigen
  │
  ├── logs                           ● Live-Logs streamen
  │   ├── --follow, -f               Fortlaufend
  │   ├── --mac <mac>                Nur Logs für diese MAC
  │   ├── --level <level>            Mindest-Level
  │   └── --service <dhcp|tftp|http> Nur ein Dienst
  │
  ├── reload                         ● Config neu laden (kein Neustart)
  │
  ├── restart                        ● Server neustarten
  │
  ├── client                         Client/Computer-Verwaltung
  │   │
  │   ├── list                       Alle Clients auflisten
  │   │   ├── --file <file>          Nur aus dieser Datei
  │   │   ├── --type <uefi|bios>     Nach Typ filtern
  │   │   └── --verbose, -v          Mit Details
  │   │
  │   ├── show <mac>                 Details eines Clients
  │   │
  │   ├── add                        Neuen Client interaktiv anlegen
  │   │   ├── --mac <mac>            MAC-Adresse
  │   │   ├── --name <name>          Hostname
  │   │   ├── --type <type>          uefi-only | bios-only | auto
  │   │   ├── --file <file>          In welche Datei schreiben
  │   │   └── --from <mac>           Config von anderem Client kopieren
  │   │
  │   ├── edit <mac>                 Client-Config bearbeiten
  │   │   └── --editor <editor>      Öffnet $EDITOR mit Client-Block
  │   │
  │   ├── remove <mac>               Client entfernen
  │   │   └── --yes                  Ohne Bestätigung
  │   │
  │   ├── copy <src-mac> <dst-mac>   Client duplizieren
  │   │   ├── --name <name>          Neuer Hostname
  │   │   └── --file <file>          In andere Datei schreiben
  │   │
  │   ├── move <mac> <file>          Client in andere Datei verschieben
  │   │
  │   ├── enable <mac>               Client aktivieren
  │   ├── disable <mac>              Client deaktivieren (auskommentieren)
  │   │
  │   └── wake <mac>                 ● Wake-on-LAN senden
  │
  ├── session                        ● Boot-Sessions (braucht Server)
  │   ├── list                       Aktive + letzte Sessions
  │   │   ├── --active               Nur laufende
  │   │   └── --failed               Nur fehlgeschlagene
  │   ├── show <mac|session-id>      Session-Details + Timeline
  │   └── history <mac>              Boot-Historie eines Clients
  │
  ├── config                         Konfiguration verwalten
  │   ├── show                       Gesamte Config anzeigen (aufgelöst)
  │   ├── get <key>                  Einzelnen Wert lesen
  │   ├── set <key> <value>          Einzelnen Wert setzen
  │   └── diff                       Laufende vs. Datei vergleichen
  │
  ├── download                       Bootloader herunterladen
  │   ├── ipxe                       iPXE Bootloader (alle Architekturen)
  │   └── --dir <path>               Zielverzeichnis
  │
  ├── web                            ● Web-UI starten/Status
  │   ├── --bind <addr>              Bind-Adresse (default: 127.0.0.1:9090)
  │   └── --no-auth                  Ohne Authentifizierung (dev)
  │
  └── version                        Version + Build-Info

  ● = braucht laufenden Server (kommuniziert via Socket/API)
```

---

## 5. Interaktiver Modus: `bootforge client add`

```
  $ bootforge client add

  ┌─────────────────────────────────────────────────┐
  │  Neuen Client anlegen                           │
  └─────────────────────────────────────────────────┘

  MAC-Adresse: c6:c9:4b:45:bf:4c
  ✓ Format gültig

  Name (optional): webserver-01

  Boot-Typ:
   ▶ [1] uefi-only    Nur UEFI-Boot
     [2] bios-only    Nur BIOS/Legacy-Boot
     [3] auto         Beides — automatische Erkennung
  Wahl: 1

  TFTP-Quelle für UEFI:
   ▶ [1] Interner TFTP    Bootforge liefert den Bootloader aus
     [2] Externer TFTP    Ein anderer TFTP-Server liefert aus
  Wahl: 1

  TFTP-Verzeichnis: data/uefi-01/tftp/
  ✓ Verzeichnis existiert, 2 Dateien gefunden
  Bootfile [ipxe.efi]: ipxe.efi
  ✓ data/uefi-01/tftp/ipxe.efi existiert (984.2 KB)

  HTTP-Quelle für UEFI:
   ▶ [1] Interner HTTP    Bootforge liefert Kernel/Initrd aus
     [2] Externer HTTP    Ein anderer HTTP-Server liefert aus
  Wahl: 1

  HTTP-Verzeichnis: data/uefi-01/http/
  ✓ Verzeichnis existiert
  HTTP-Pfad [/webserver-01/]: /webserver-01/
  ✓ Pfad /webserver-01/ ist noch nicht vergeben

  In welche Datei speichern?
    Vorhandene Dateien:
    [1] computers/default-uefi.toml     (1 Client)
    [2] computers/computer-raum-01.toml (24 Clients)
    [3] computers/server-rack-a.toml    (2 Clients)
    [4] Neue Datei erstellen
  Wahl: 4
  Dateiname: computers/webserver.toml

  ┌─────────────────────────────────────────────────┐
  │  Zusammenfassung:                               │
  │                                                  │
  │  MAC:       c6:c9:4b:45:bf:4c                  │
  │  Name:      webserver-01                        │
  │  Typ:       uefi-only                           │
  │  TFTP:      intern → data/uefi-01/tftp/         │
  │  Bootfile:  ipxe.efi                            │
  │  HTTP:      intern → data/uefi-01/http/         │
  │  HTTP-Pfad: /webserver-01/                      │
  │  Datei:     computers/webserver.toml            │
  └─────────────────────────────────────────────────┘

  Speichern? [j/n]: j

  ✓ Client gespeichert in computers/webserver.toml

  Server läuft — soll die Config jetzt neu geladen werden? [j/n]: j
  ✓ Config neu geladen. Client ist aktiv.
```

---

## 6. Schnellmodus: Alles als Flags

```
  # Dasselbe wie oben, aber nicht-interaktiv:

  $ bootforge client add \
      --mac c6:c9:4b:45:bf:4c \
      --name webserver-01 \
      --type uefi-only \
      --tftp-files data/uefi-01/tftp/ \
      --bootfile ipxe.efi \
      --http-files data/uefi-01/http/ \
      --http-path /webserver-01/ \
      --file computers/webserver.toml

  ✓ Client gespeichert in computers/webserver.toml

  # Client von einem anderen kopieren:

  $ bootforge client copy c6:c9:4b:45:bf:4c c6:c9:4b:45:bf:4d \
      --name webserver-02 \
      --file computers/webserver.toml

  ✓ Client kopiert: webserver-02 (c6:c9:4b:45:bf:4d)
    Quelle: webserver-01 (c6:c9:4b:45:bf:4c)
```

---

## 7. Web-UI — Was und Warum

```
  ┌────────────────────────────────────────────────────────────────┐
  │  Brauchen wir eine Web-UI?                                    │
  │                                                                │
  │  JA — und zwar aus einem einfachen Grund:                     │
  │                                                                │
  │  Das CLI ist für den Admin der das Tool aufgesetzt hat.       │
  │  Die Web-UI ist für JEDEN der es benutzen muss.              │
  │                                                                │
  │  Beispiel: Du setzt Bootforge auf. Dein Kollege soll einen   │
  │  neuen PC zum PXE-Boot hinzufügen. Willst du ihm SSH-Zugang  │
  │  geben und CLI-Kommandos erklären? Oder sagst du:            │
  │  "Geh auf bootforge.local:9090 und klick auf 'Neuer Client'" │
  │                                                                │
  │  Die Web-UI ist NICHT der primäre Weg — sie ist der           │
  │  zugängliche Weg.                                             │
  └────────────────────────────────────────────────────────────────┘
```

### 7.1 Web-UI Seitenstruktur

```
  ┌──────────────────────────────────────────────────────────────┐
  │  BOOTFORGE                              admin ▾   ⚙ Settings│
  ├──────────┬───────────────────────────────────────────────────┤
  │          │                                                   │
  │ ■ Dash-  │  Dashboard — Alles auf einen Blick               │
  │   board  │                                                   │
  │          │  ┌─────────┐ ┌─────────┐ ┌─────────┐            │
  │ ■ Clients│  │ DHCP ✓  │ │ TFTP ✓  │ │ HTTP ✓  │            │
  │          │  │ Port 67 │ │ Port 69 │ │ Port8080│            │
  │ ■ Sess-  │  └─────────┘ └─────────┘ └─────────┘            │
  │   ions   │                                                   │
  │          │  Letzte Aktivität:                                │
  │ ■ Self-  │  ● 14:23  aa:bb:cc:..  BOOT COMPLETE  47s       │
  │   Tests  │  ● 14:25  dd:ee:ff:..  ⚠ STALLED      TFTP→HTTP│
  │          │  ● 14:32  99:88:77:..  ✗ FILE MISSING  pxe..0   │
  │ ■ Logs   │                                                   │
  │          │  Self-Test: ✓ vor 2 Minuten                      │
  │ ■ Config │  Clients: 52 konfiguriert, 3 aktiv               │
  │          │                                                   │
  │ ■ Files  │                                                   │
  │          │                                                   │
  └──────────┴───────────────────────────────────────────────────┘
```

### 7.2 Seiten im Detail

```
  ═══════════════════════════════════════════════════════════════
  SEITE: Dashboard
  ═══════════════════════════════════════════════════════════════

  Zweck: Sofort sehen ob alles läuft, und wenn nicht, was kaputt ist

  Inhalte:
  • Dienst-Status (DHCP/TFTP/HTTP) mit Ampel-Farben
  • Letzte 10 Boot-Aktivitäten (Live-Updates via WebSocket)
  • Letzter Self-Test: Ergebnis + Zeitpunkt
  • Fehlerzähler seit Start
  • Uptime

  ═══════════════════════════════════════════════════════════════
  SEITE: Clients
  ═══════════════════════════════════════════════════════════════

  Zweck: Alle Computer verwalten

  ┌────────────────────────────────────────────────────────────┐
  │  Clients                              [+ Neuer Client]    │
  │                                                            │
  │  Filter: [Alle ▾] [Alle Typen ▾]  Suche: [___________]   │
  │                                                            │
  │  ┌── computer-raum-01.toml (24 Clients) ──────────────┐  │
  │  │                                                      │  │
  │  │  ● raum01-pc01   c6:c9:..:bf:01  uefi  ✓ zuletzt   │  │
  │  │                                         14:23 heute │  │
  │  │  ○ raum01-pc02   c6:c9:..:bf:02  uefi  — nie       │  │
  │  │  ○ raum01-pc03   c6:c9:..:bf:03  uefi  — nie       │  │
  │  │  ...                                                 │  │
  │  └──────────────────────────────────────────────────────┘  │
  │                                                            │
  │  ┌── server-rack-a.toml (2 Clients) ──────────────────┐  │
  │  │                                                      │  │
  │  │  ● db-master    aa:bb:..:ee:01  auto  ✓ 2025-01-15 │  │
  │  │  ⚠ legacy-box   aa:bb:..:ee:02  bios  ✗ fehlgeschl.│  │
  │  │                                                      │  │
  │  └──────────────────────────────────────────────────────┘  │
  │                                                            │
  │  Gruppiert nach Config-Datei — genau wie auf der Platte   │
  └────────────────────────────────────────────────────────────┘

  Client-Detail-Ansicht (Klick auf einen Client):
  ┌────────────────────────────────────────────────────────────┐
  │  ← Zurück                                                  │
  │                                                            │
  │  raum01-pc01                              [Bearbeiten]    │
  │  c6:c9:4b:45:bf:01                       [Kopieren]      │
  │  Datei: computer-raum-01.toml            [Deaktivieren]  │
  │                                           [Löschen]       │
  │                                                            │
  │  ┌─ Konfiguration ─────────────────────────────────────┐  │
  │  │  Typ:       uefi-only                                │  │
  │  │  TFTP:      intern → data/ubuntu-24-PC/tftp/         │  │
  │  │  Bootfile:  ipxe.efi (984 KB, ✓ vorhanden)          │  │
  │  │  HTTP:      intern → data/ubuntu-24-PC/http/         │  │
  │  │  HTTP-Pfad: /raum01/pc01/                            │  │
  │  │  Variablen: hostname=raum01-pc01, locale=de_DE...    │  │
  │  └─────────────────────────────────────────────────────┘  │
  │                                                            │
  │  ┌─ Boot-Historie ─────────────────────────────────────┐  │
  │  │                                                      │  │
  │  │  2025-01-20 14:23:01  ✓ COMPLETE (47s)              │  │
  │  │  ├─ 14:23:01  DISCOVER (UEFI x64)                   │  │
  │  │  ├─ 14:23:01  OFFER sent                            │  │
  │  │  ├─ 14:23:02  TFTP ipxe.efi (984KB, 340ms)         │  │
  │  │  ├─ 14:23:03  HTTP /boot.ipxe → 200                 │  │
  │  │  ├─ 14:23:04  HTTP /vmlinuz → 200 (8.2MB)          │  │
  │  │  ├─ 14:23:06  HTTP /initrd → 200 (52MB)            │  │
  │  │  └─ 14:23:41  HTTP /preseed.cfg → 200               │  │
  │  │                                                      │  │
  │  │  2025-01-18 09:11:33  ✗ FAILED (stalled at TFTP)   │  │
  │  │  ├─ 09:11:33  DISCOVER (UEFI x64)                   │  │
  │  │  ├─ 09:11:33  OFFER sent                            │  │
  │  │  └─ 09:11:48  ⚠ TIMEOUT: Kein TFTP Request         │  │
  │  │               Hint: Firewall? Anderer PXE-Server?   │  │
  │  │                                                      │  │
  │  └─────────────────────────────────────────────────────┘  │
  └────────────────────────────────────────────────────────────┘

  ═══════════════════════════════════════════════════════════════
  SEITE: Sessions (Live)
  ═══════════════════════════════════════════════════════════════

  Zweck: Was passiert JETZT GERADE?

  ┌────────────────────────────────────────────────────────────┐
  │  Aktive Boot-Sessions                    Auto-Refresh: 2s │
  │                                                            │
  │  ┌── c6:c9:4b:45:bf:01 (raum01-pc01) ─────────────────┐  │
  │  │                                                      │  │
  │  │  ■────■────■────□────□────□                          │  │
  │  │  DISC  OFFR  TFTP  iPXE  KERN  DONE                 │  │
  │  │             ▲                                        │  │
  │  │        HIER ┘ (TFTP Transfer: 45% ████░░░░ 440KB)   │  │
  │  │                                                      │  │
  │  │  Gestartet: vor 3s   Erwartete Dauer: ~45s           │  │
  │  └──────────────────────────────────────────────────────┘  │
  │                                                            │
  │  ┌── aa:bb:cc:dd:ee:02 (legacy-box) ──────────────────┐  │
  │  │                                                      │  │
  │  │  ■────■────□────□────□────□                          │  │
  │  │  DISC  OFFR  TFTP  iPXE  KERN  DONE                 │  │
  │  │        ▲                                             │  │
  │  │   HIER ┘ ⚠ Warte auf TFTP seit 12s (timeout: 15s)  │  │
  │  │                                                      │  │
  │  └──────────────────────────────────────────────────────┘  │
  └────────────────────────────────────────────────────────────┘

  ═══════════════════════════════════════════════════════════════
  SEITE: Self-Tests
  ═══════════════════════════════════════════════════════════════

  Zweck: Funktioniert mein Setup?

  ┌────────────────────────────────────────────────────────────┐
  │  Self-Tests                         [▶ Jetzt ausführen]   │
  │                                                            │
  │  Letzter Lauf: vor 28s (automatisch alle 30s)             │
  │                                                            │
  │  ✓ DHCP Probe       ProxyOffer empfangen in 2ms           │
  │  ✓ TFTP Read        ipxe.efi OK (984KB, 12ms)             │
  │  ✓ TFTP Read        undionly.kpxe OK (67KB, 8ms)           │
  │  ✓ HTTP /healthz    200 OK in 1ms                          │
  │  ✓ HTTP Boot-Pfade  26/26 erreichbar                       │
  │  ✓ File Integrity   Alle Checksummen OK                    │
  │  ✓ Disk Space       42.1 GB frei (min: 1 GB)              │
  │                                                            │
  │  ─── Historie ──────────────────────────────────────────── │
  │                                                            │
  │  14:30:00  ✓ Alle Tests bestanden  (7/7)                  │
  │  14:29:30  ✓ Alle Tests bestanden  (7/7)                  │
  │  14:29:00  ⚠ 1 Warnung            (6/7)                  │
  │            └─ HTTP: /rack-a/legacy/ → 404                  │
  │  14:28:30  ✓ Alle Tests bestanden  (7/7)                  │
  │  ...                                                       │
  └────────────────────────────────────────────────────────────┘

  ═══════════════════════════════════════════════════════════════
  SEITE: Logs
  ═══════════════════════════════════════════════════════════════

  Zweck: Was ist passiert? Live mitlesen oder filtern.

  ┌────────────────────────────────────────────────────────────┐
  │  Logs                                                      │
  │                                                            │
  │  Filter: [Alle Dienste ▾] [Alle Level ▾]                 │
  │  MAC:    [________________]  [▶ Live] [⏸ Pause]          │
  │                                                            │
  │  14:32:15 ERR  tftp  RRQ "pxelinux.0" von 10.0.0.77      │
  │                      — DATEI NICHT GEFUNDEN                │
  │                      MAC: 99:88:77:66:55:44 (unbekannt)   │
  │                                                            │
  │  14:30:00 INFO health Self-Test: alle 7 Tests bestanden   │
  │                                                            │
  │  14:23:41 INFO http   GET /config/.../preseed.cfg → 200   │
  │  14:23:06 INFO http   GET /images/.../initrd → 200 (52MB) │
  │  14:23:04 INFO http   GET /images/.../vmlinuz → 200 (8MB) │
  │  14:23:03 INFO http   GET /boot/.../boot.ipxe → 200       │
  │  14:23:02 INFO tftp   Transfer OK: ipxe.efi (984KB,340ms) │
  │  14:23:01 INFO dhcp   OFFER → c6:c9:4b:45:bf:01 (UEFI)   │
  │  14:23:01 INFO dhcp   DISCOVER ← c6:c9:4b:45:bf:01       │
  │                                                            │
  └────────────────────────────────────────────────────────────┘

  ═══════════════════════════════════════════════════════════════
  SEITE: Config
  ═══════════════════════════════════════════════════════════════

  Zweck: Server-Einstellungen anpassen ohne SSH

  ┌────────────────────────────────────────────────────────────┐
  │  Konfiguration                          [↻ Neu laden]     │
  │                                                            │
  │  ┌─ Server ────────────────────────────────────────────┐  │
  │  │  Interface:   [ens18      ▾]                        │  │
  │  │  IP:          10.0.0.10 (auto-detected)             │  │
  │  │  Log-Level:   [info       ▾]                        │  │
  │  │  Log-Format:  [pretty     ▾]                        │  │
  │  └────────────────────────────────────────────────────┘  │
  │                                                            │
  │  ┌─ Dienste ───────────────────────────────────────────┐  │
  │  │  DHCP Proxy:  [✓] Port: [67  ] Proxy: [4011]       │  │
  │  │  TFTP:        [✓] Port: [69  ] Block:  [1468]      │  │
  │  │  HTTP:        [✓] Port: [8080] TLS:    [ ]         │  │
  │  └────────────────────────────────────────────────────┘  │
  │                                                            │
  │  ┌─ Health ────────────────────────────────────────────┐  │
  │  │  Interval:    [30s     ]                            │  │
  │  │  DHCP Probe:  [✓]  TFTP Read: [✓]  HTTP: [✓]      │  │
  │  │  File Check:  [✓]  Min Disk:  [1GB     ]           │  │
  │  └────────────────────────────────────────────────────┘  │
  │                                                            │
  │  ⚠ Änderungen erfordern "Neu laden" oder Server-Neustart │
  │                                                [Speichern] │
  └────────────────────────────────────────────────────────────┘

  ═══════════════════════════════════════════════════════════════
  SEITE: Files (Dateimanager)
  ═══════════════════════════════════════════════════════════════

  Zweck: Boot-Dateien verwalten ohne SSH/SCP

  ┌────────────────────────────────────────────────────────────┐
  │  Dateiverwaltung                          [↑ Upload]      │
  │                                                            │
  │  /etc/bootforge/data/                                      │
  │  ├── 📁 uefi-01/                                          │
  │  │   ├── 📁 tftp/                                         │
  │  │   │   └── 📄 ipxe.efi         984.2 KB   ✓ genutzt    │
  │  │   └── 📁 http/                                         │
  │  │       ├── 📄 boot.ipxe        1.2 KB     ✓ genutzt    │
  │  │       ├── 📄 vmlinuz          8.2 MB     ✓ genutzt    │
  │  │       └── 📄 initrd           52.1 MB    ✓ genutzt    │
  │  │                                                         │
  │  ├── 📁 rescue/                                            │
  │  │   └── 📁 http/                                         │
  │  │       └── ⚠ boot.ipxe        FEHLT                    │
  │  │           Benötigt von: legacy-box (aa:bb:cc:dd:ee:02) │
  │  │                                                         │
  │  └── 📁 ubuntu-24-PC/                                     │
  │      └── ...                                               │
  │                                                            │
  │  Speicherplatz: 1.2 GB belegt, 42.1 GB frei               │
  └────────────────────────────────────────────────────────────┘
```

---

## 8. Was CLI kann vs. Web-UI kann

```
┌──────────────────────────────┬──────┬────────┬──────────────────┐
│ Funktion                     │ CLI  │ Web-UI │ Anmerkung        │
├──────────────────────────────┼──────┼────────┼──────────────────┤
│ Server starten               │  ✓   │  ✗     │ Nur CLI/systemd  │
│ Init / Scaffold              │  ✓   │  ✗     │ Nur CLI          │
│ Download iPXE                │  ✓   │  ✓     │                  │
│ Config validieren            │  ✓   │  ✓     │                  │
│ Config anzeigen              │  ✓   │  ✓     │                  │
│ Config ändern                │  ✓   │  ✓     │                  │
│ Config reload                │  ✓   │  ✓     │                  │
│ Server restart               │  ✓   │  ✓     │                  │
│ Status anzeigen              │  ✓   │  ✓     │ Web: Dashboard   │
│ Client auflisten             │  ✓   │  ✓     │                  │
│ Client anlegen               │  ✓   │  ✓     │ CLI: interaktiv  │
│ Client kopieren              │  ✓   │  ✓     │                  │
│ Client bearbeiten            │  ✓   │  ✓     │ CLI: $EDITOR     │
│ Client verschieben           │  ✓   │  ✓     │ Drag & Drop?     │
│ Client aktivieren/deaktiv.   │  ✓   │  ✓     │ Toggle-Switch    │
│ Client löschen               │  ✓   │  ✓     │                  │
│ Wake-on-LAN                  │  ✓   │  ✓     │                  │
│ Self-Test auslösen           │  ✓   │  ✓     │                  │
│ Self-Test Historie           │  ✓   │  ✓     │                  │
│ Boot-Sessions live           │  ✓   │  ✓     │ Web: WebSocket   │
│ Boot-Historie pro Client     │  ✓   │  ✓     │ Web: Timeline    │
│ Logs live                    │  ✓   │  ✓     │ CLI: --follow    │
│ Logs filtern                 │  ✓   │  ✓     │                  │
│ Dateiverwaltung              │  ✗   │  ✓     │ Web: Upload/DL   │
│ Datei-Upload                 │  ✗   │  ✓     │ Drag & Drop      │
│ Boot-Fortschritt visuell     │  △   │  ✓     │ CLI: Text-only   │
│ Metriken / Grafiken          │  ✗   │  ✓     │ Oder Prometheus   │
│ Scriptbar / Automatisierung  │  ✓   │  ✗     │ CLI + --json     │
│ Pipe / Redirect              │  ✓   │  ✗     │                  │
│ Ohne Browser nutzbar         │  ✓   │  ✗     │                  │
└──────────────────────────────┴──────┴────────┴──────────────────┘

  Grundregel:
  ┌────────────────────────────────────────────────────────────────┐
  │  CLI  = Alles was automatisierbar sein muss + Server-Start    │
  │  Web  = Alles was visuell besser ist + Dateimanagement        │
  │  Beide = Der gesamte Rest (und das ist das meiste)            │
  └────────────────────────────────────────────────────────────────┘
```

---

## 9. API Design (für CLI + Web-UI gemeinsam)

```
  Beide nutzen dieselbe REST API:

  GET    /api/v1/status                    Server-Status
  POST   /api/v1/reload                    Config neu laden
  POST   /api/v1/restart                   Server neustarten

  GET    /api/v1/clients                   Alle Clients
  GET    /api/v1/clients/{mac}             Ein Client
  POST   /api/v1/clients                   Client anlegen
  PUT    /api/v1/clients/{mac}             Client ändern
  DELETE /api/v1/clients/{mac}             Client löschen
  POST   /api/v1/clients/{mac}/copy        Client kopieren
  POST   /api/v1/clients/{mac}/move        Client verschieben
  POST   /api/v1/clients/{mac}/enable      Client aktivieren
  POST   /api/v1/clients/{mac}/disable     Client deaktivieren
  POST   /api/v1/clients/{mac}/wake        Wake-on-LAN

  GET    /api/v1/sessions                  Aktive Sessions
  GET    /api/v1/sessions/{mac}            Session-Detail
  GET    /api/v1/sessions/{mac}/history    Boot-Historie

  POST   /api/v1/test                      Self-Test auslösen
  GET    /api/v1/test/history              Letzte Ergebnisse

  GET    /api/v1/logs                      Logs (Query-Params zum Filtern)
  WS     /api/v1/logs/stream              Live-Logs via WebSocket
  WS     /api/v1/sessions/stream          Live-Sessions via WebSocket

  GET    /api/v1/config                    Config anzeigen
  PUT    /api/v1/config                    Config ändern

  GET    /api/v1/files/{path}              Datei-Listing / Download
  POST   /api/v1/files/{path}             Datei-Upload
  DELETE /api/v1/files/{path}             Datei löschen

  GET    /api/v1/metrics                   Prometheus Metriken
```

---

## 10. Zusammenfassung: Was wir geplant haben

```
  ┌────────────────────────────────────────────────────────────────┐
  │                                                                │
  │  Dokument 1: Architektur + Boot-Flow + Diagnostics            │
  │  ✓ Drei Dienste, ein Prozess                                  │
  │  ✓ Boot-Flow Tracking mit State-Machine                       │
  │  ✓ Self-Test System                                           │
  │  ✓ Startup-Validierung                                        │
  │                                                                │
  │  Dokument 2: Modulare Config-Struktur                         │
  │  ✓ bootforge.toml (Server) + computers/*.toml (Clients)       │
  │  ✓ Intern/Extern wählbar pro Client pro Dienst               │
  │  ✓ Defaults, Gruppen, Einzelprofile                           │
  │  ✓ UEFI/BIOS Auto-Detect                                     │
  │                                                                │
  │  Dokument 3 (dieses): CLI + Web-UI + API                     │
  │  ✓ Ein Binary: bootforge serve + bootforge <command>          │
  │  ✓ Offline-Aktionen (direkt auf TOML) + Online (via API)     │
  │  ✓ Interaktiver CLI-Modus + Flag-Modus                       │
  │  ✓ Web-UI: Dashboard, Clients, Sessions, Tests, Logs, Files  │
  │  ✓ REST API als gemeinsame Grundlage                         │
  │                                                                │
  │  ─── Was FEHLT noch an Planung? ────────────────────────────  │
  │                                                                │
  │  ? iPXE Script-Generierung: Wie genau sieht das              │
  │    dynamische boot.ipxe aus? Template-Sprache?                │
  │                                                                │
  │  ? Preseed/Kickstart Templates: Go text/template              │
  │    oder eigene Syntax?                                        │
  │                                                                │
  │  ? Authentifizierung Web-UI: Token? Basic Auth?               │
  │    Kein Auth auf localhost?                                    │
  │                                                                │
  │  ? Persistenz: Sessions in SQLite, bbolt,                     │
  │    oder einfach JSON-Dateien?                                 │
  │                                                                │
  │  ? Phased Rollout: Was ist MVP (Phase 1)?                    │
  │    Was kommt erst in Phase 2/3?                               │
  │                                                                │
  └────────────────────────────────────────────────────────────────┘
```
