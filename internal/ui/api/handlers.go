package api

import (
	"log/slog"
	"net/http"
	"time"

	"bootforge/internal/buildinfo"
	"bootforge/internal/domain"
	"bootforge/internal/health"
	"bootforge/internal/server"
)

// JSON response types.

// StatusJSON is the response for GET /api/v1/status.
type StatusJSON struct {
	Version string      `json:"version"`
	Uptime  string      `json:"uptime"`
	Health  *HealthJSON `json:"health,omitempty"`
}

// ClientJSON is the JSON representation of a client.
type ClientJSON struct {
	MAC     string            `json:"mac"`
	Name    string            `json:"name"`
	Enabled bool              `json:"enabled"`
	Entries []string          `json:"entries"`
	Default string            `json:"default,omitempty"`
	Timeout int               `json:"timeout"`
	Vars    map[string]string `json:"vars,omitempty"`
}

// MenuJSON is the JSON representation of a menu entry.
type MenuJSON struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Kernel      string `json:"kernel,omitempty"`
	Initrd      string `json:"initrd,omitempty"`
}

// SessionJSON is the JSON representation of a boot session.
type SessionJSON struct {
	ID    string `json:"id"`
	MAC   string `json:"mac"`
	State string `json:"state"`
	Arch  string `json:"arch"`
	At    string `json:"started_at"`
}

// HealthJSON is the JSON representation of a health result.
type HealthJSON struct {
	Status string           `json:"status"`
	Checks []CheckJSON      `json:"checks"`
	At     string           `json:"checked_at"`
}

// CheckJSON is the JSON representation of a single health check.
type CheckJSON struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Message  string `json:"message"`
	Duration string `json:"duration"`
}

// APIDeps holds all dependencies needed by the API handlers.
type APIDeps struct {
	Registry *server.Registry
	Menus    *server.MenuResolver
	Sessions *server.SessionManager
	Health   *health.Checker
	StartedAt time.Time
	ReloadFn func() error
}

// handlers is the internal handler collection.
type handlers struct {
	deps   *APIDeps
	logger *slog.Logger
}

func (h *handlers) handleStatus(w http.ResponseWriter, r *http.Request) {
	resp := StatusJSON{
		Version: buildinfo.String(),
		Uptime:  time.Since(h.deps.StartedAt).Round(time.Second).String(),
	}

	if h.deps.Health != nil {
		if latest := h.deps.Health.Latest(); latest != nil {
			hj := healthResultToJSON(latest)
			resp.Health = &hj
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *handlers) handleClients(w http.ResponseWriter, r *http.Request) {
	clients := h.deps.Registry.ListClients()
	result := make([]ClientJSON, 0, len(clients))
	for _, c := range clients {
		result = append(result, clientToJSON(c))
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handlers) handleClientShow(w http.ResponseWriter, r *http.Request) {
	macStr := r.PathValue("mac")
	mac, err := domain.ParseMAC(macStr)
	if err != nil {
		errorJSON(w, http.StatusBadRequest, "invalid MAC address")
		return
	}

	client, err := h.deps.Registry.FindClient(mac)
	if err != nil {
		errorJSON(w, http.StatusNotFound, "client not found")
		return
	}

	writeJSON(w, http.StatusOK, clientToJSON(client))
}

func (h *handlers) handleClientWake(w http.ResponseWriter, r *http.Request) {
	macStr := r.PathValue("mac")
	mac, err := domain.ParseMAC(macStr)
	if err != nil {
		errorJSON(w, http.StatusBadRequest, "invalid MAC address")
		return
	}

	_, err = h.deps.Registry.FindClient(mac)
	if err != nil {
		errorJSON(w, http.StatusNotFound, "client not found")
		return
	}

	// WoL send would go here in full integration.
	h.logger.Info("WoL requested via API", "mac", macStr)
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"mac":    mac.String(),
	})
}

func (h *handlers) handleMenus(w http.ResponseWriter, r *http.Request) {
	menus := h.deps.Menus.ListAll()
	result := make([]MenuJSON, 0, len(menus))
	for _, m := range menus {
		result = append(result, MenuJSON{
			Name:        m.Name,
			Label:       m.Label,
			Type:        m.Type.String(),
			Description: m.Description,
			Kernel:      m.Boot.Kernel,
			Initrd:      m.Boot.Initrd,
		})
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handlers) handleSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.deps.Sessions.Active()
	if err != nil {
		errorJSON(w, http.StatusInternalServerError, "failed to list sessions")
		return
	}

	result := make([]SessionJSON, 0, len(sessions))
	for _, s := range sessions {
		result = append(result, SessionJSON{
			ID:    s.ID,
			MAC:   s.MAC.String(),
			State: s.State.String(),
			Arch:  s.Arch.String(),
			At:    s.StartedAt.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handlers) handleReload(w http.ResponseWriter, r *http.Request) {
	if h.deps.ReloadFn == nil {
		errorJSON(w, http.StatusNotImplemented, "reload not configured")
		return
	}

	if err := h.deps.ReloadFn(); err != nil {
		h.logger.Error("config reload failed", "error", err)
		errorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.logger.Info("configuration reloaded via API")
	writeJSON(w, http.StatusOK, map[string]string{"status": "reloaded"})
}

func (h *handlers) handleTest(w http.ResponseWriter, r *http.Request) {
	if h.deps.Health == nil {
		errorJSON(w, http.StatusNotImplemented, "health checker not configured")
		return
	}

	result := h.deps.Health.RunOnce()
	writeJSON(w, http.StatusOK, healthResultToJSON(&result))
}

func clientToJSON(c *domain.Client) ClientJSON {
	mac := c.MAC.String()
	if c.IsWildcard() {
		mac = "*"
	}
	return ClientJSON{
		MAC:     mac,
		Name:    c.Name,
		Enabled: c.Enabled,
		Entries: c.Menu.Entries,
		Default: c.Menu.Default,
		Timeout: c.Menu.Timeout,
		Vars:    c.Vars,
	}
}

func healthResultToJSON(hr *domain.HealthResult) HealthJSON {
	checks := make([]CheckJSON, 0, len(hr.Checks))
	for _, c := range hr.Checks {
		checks = append(checks, CheckJSON{
			Name:     c.Name,
			Status:   c.Status.String(),
			Message:  c.Message,
			Duration: c.Duration.Round(time.Millisecond).String(),
		})
	}
	return HealthJSON{
		Status: hr.Status.String(),
		Checks: checks,
		At:     hr.At.Format(time.RFC3339),
	}
}
