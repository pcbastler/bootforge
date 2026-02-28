// Package api implements the REST API and WebSocket endpoints.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// NewRouter creates a new HTTP mux with all API routes registered.
func NewRouter(deps *APIDeps, logger *slog.Logger) *http.ServeMux {
	h := &handlers{deps: deps, logger: logger}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/status", h.handleStatus)
	mux.HandleFunc("GET /api/v1/clients", h.handleClients)
	mux.HandleFunc("GET /api/v1/clients/{mac}", h.handleClientShow)
	mux.HandleFunc("POST /api/v1/clients/{mac}/wake", h.handleClientWake)
	mux.HandleFunc("GET /api/v1/menus", h.handleMenus)
	mux.HandleFunc("GET /api/v1/sessions", h.handleSessions)
	mux.HandleFunc("POST /api/v1/reload", h.handleReload)
	mux.HandleFunc("POST /api/v1/test", h.handleTest)
	mux.HandleFunc("GET /api/v1/logs", h.handleLogs)

	return mux
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// errorJSON writes a JSON error response.
func errorJSON(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
