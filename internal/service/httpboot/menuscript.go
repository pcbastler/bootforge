package httpboot

import (
	"log/slog"
	"net"
	"net/http"

	"bootforge/internal/server"
)

// MenuScriptHandler serves iPXE boot scripts for clients based on their MAC address.
type MenuScriptHandler struct {
	registry *server.Registry
	menus    *server.MenuResolver
	ipxe     *server.IPXEGenerator
	vars     server.IPXEVars
	logger   *slog.Logger
}

// NewMenuScriptHandler creates a handler that generates iPXE scripts per client.
func NewMenuScriptHandler(
	registry *server.Registry,
	menus *server.MenuResolver,
	ipxe *server.IPXEGenerator,
	vars server.IPXEVars,
	logger *slog.Logger,
) *MenuScriptHandler {
	return &MenuScriptHandler{
		registry: registry,
		menus:    menus,
		ipxe:     ipxe,
		vars:     vars,
		logger:   logger,
	}
}

// ServeHTTP handles GET /boot/{mac}/menu.ipxe requests.
func (h *MenuScriptHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	macStr := r.PathValue("mac")

	mac, err := net.ParseMAC(macStr)
	if err != nil {
		h.logger.Warn("invalid MAC in boot request", "mac", macStr, "error", err)
		http.Error(w, "Bad Request: invalid MAC address", http.StatusBadRequest)
		return
	}

	client, err := h.registry.FindClient(mac)
	if err != nil {
		h.logger.Info("no client config for MAC", "mac", macStr)
		http.Error(w, "Not Found: no configuration for this MAC", http.StatusNotFound)
		return
	}

	if !client.Enabled {
		h.logger.Info("client is disabled", "mac", macStr, "name", client.Name)
		http.Error(w, "Forbidden: client is disabled", http.StatusForbidden)
		return
	}

	// Per-request vars with the client's MAC and custom vars.
	vars := h.vars
	vars.MAC = mac.String()
	if client.Vars != nil {
		if vars.Custom == nil {
			vars.Custom = make(map[string]string)
		}
		for k, v := range client.Vars {
			vars.Custom[k] = v
		}
	}

	// Check for one-time boot override.
	if overrideName := h.registry.ConsumeBootOverride(mac); overrideName != "" {
		entry, err := h.menus.FindByName(overrideName)
		if err != nil {
			h.logger.Error("boot override references unknown menu entry",
				"mac", macStr, "entry", overrideName, "error", err)
			// Fall through to normal menu.
		} else {
			h.logger.Info("serving boot override",
				"mac", macStr, "entry", overrideName)
			script := h.ipxe.GenerateOverride(entry, vars)
			serveScript(w, script)
			return
		}
	}

	// Resolve menu entries for this client.
	entries, err := h.menus.Resolve(client.Menu.Entries)
	if err != nil {
		h.logger.Error("resolving menu entries",
			"mac", macStr, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	script := h.ipxe.Generate(entries, client.Menu, vars)
	h.logger.Info("serving iPXE script",
		"mac", macStr,
		"client", client.Name,
		"entries", len(entries),
	)
	serveScript(w, script)
}

func serveScript(w http.ResponseWriter, script string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(script))
}

// UpdateVars replaces the base iPXE variables (e.g. after config reload).
func (h *MenuScriptHandler) UpdateVars(vars server.IPXEVars) {
	h.vars = vars
}

// UpdateComponents replaces the registry and menu resolver (e.g. after config reload).
func (h *MenuScriptHandler) UpdateComponents(registry *server.Registry, menus *server.MenuResolver) {
	h.registry = registry
	h.menus = menus
}
