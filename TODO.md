# TODO

## CLI

- [ ] `client add <mac>` — add a new client via CLI
- [ ] `client edit <mac>` — edit existing client
- [ ] `client remove <mac>` — remove a client
- [ ] `client copy <mac> <new-mac>` — duplicate client config
- [ ] `client move <mac> <new-mac>` — change client MAC
- [ ] `client enable/disable <mac>` — toggle client
- [ ] `client wake <mac>` — send Wake-on-LAN from CLI
- [ ] `client menu <mac>` — show resolved menu for a client
- [ ] `client menu-add <mac> <entry>` — add menu entry to client
- [ ] `client menu-remove <mac> <entry>` — remove menu entry from client
- [ ] `menu validate` — validate all menu entries against available files
- [ ] `menu used-by <name>` — show which clients reference a menu entry
- [ ] `bootloader download` — download iPXE binaries
- [ ] `session history [<mac>]` — show boot history
- [ ] `config get <key>` — get a single config value
- [ ] `config set <key> <value>` — set a config value
- [ ] `config diff` — show difference between running and on-disk config
- [ ] `reload` — trigger config reload from CLI (via API)
- [ ] `test` — add filter flags (`--dhcp`, `--tftp`, `--http`, `--history`)
- [ ] `logs --follow` — verify live streaming works (requires WebSocket)

## REST API

- [ ] `GET /api/v1/menus/{name}` — single menu entry
- [ ] `GET /api/v1/clients/{mac}/menu` — resolved menu for a client
- [ ] `POST /api/v1/clients` — create client
- [ ] `PUT /api/v1/clients/{mac}` — update client
- [ ] `DELETE /api/v1/clients/{mac}` — delete client
- [ ] `POST /api/v1/clients/{mac}/enable` — enable client
- [ ] `POST /api/v1/clients/{mac}/disable` — disable client
- [ ] `POST /api/v1/clients/{mac}/wake` — actually send WoL packet (currently stub)
- [ ] `GET /api/v1/sessions/{mac}/history` — boot history for a client
- [ ] `GET /api/v1/test/history` — health check history
- [ ] `GET /api/v1/config` — read current config
- [ ] `PUT /api/v1/config` — update config
- [ ] `GET/POST/DELETE /api/v1/files/{path}` — file management
- [ ] `GET /api/v1/metrics` — Prometheus-style metrics
- [ ] `WS /api/v1/logs/stream` — real-time log streaming
- [ ] `WS /api/v1/sessions/stream` — real-time session updates

## Future Phases

- [ ] Web UI (Phase 3)
- [ ] HTTP caching proxy for upstream boot files (Phase 3)
- [ ] File upload via API (Phase 3)
- [ ] iSCSI target service (Phase 2)
- [ ] Diskless boot with base+overlay (Phase 2)
- [ ] API authentication/authorization
