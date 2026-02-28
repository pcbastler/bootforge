package api

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bootforge/internal/domain"
	"bootforge/internal/infra/store"
	"bootforge/internal/server"
)

func testDeps() *APIDeps {
	menus := []*domain.MenuEntry{
		{Name: "rescue", Label: "Rescue System", Type: domain.MenuLive,
			Boot: domain.BootParams{Kernel: "vmlinuz", Initrd: "initrd"}},
		{Name: "local-disk", Label: "Boot from local disk", Type: domain.MenuExit},
	}
	clients := []*domain.Client{
		{
			MAC: net.HardwareAddr{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01},
			Name: "server-1", Enabled: true,
			Menu: domain.MenuConfig{Entries: []string{"rescue", "local-disk"}, Default: "rescue", Timeout: 30},
		},
		{
			MAC: domain.WildcardMAC, Name: "default", Enabled: true,
			Menu: domain.MenuConfig{Entries: []string{"local-disk"}},
		},
	}

	registry := server.NewRegistry(clients)
	resolver := server.NewMenuResolver(menus)
	sessions := server.NewSessionManager(store.NewMemorySessionStore())

	return &APIDeps{
		Registry:  registry,
		Menus:     resolver,
		Sessions:  sessions,
		StartedAt: time.Now(),
	}
}

func doRequest(t *testing.T, mux *http.ServeMux, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestStatusEndpoint(t *testing.T) {
	deps := testDeps()
	mux := NewRouter(deps, slog.Default())

	rec := doRequest(t, mux, "GET", "/api/v1/status")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var resp StatusJSON
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if resp.Version == "" {
		t.Error("version should not be empty")
	}
}

func TestClientsEndpoint(t *testing.T) {
	deps := testDeps()
	mux := NewRouter(deps, slog.Default())

	rec := doRequest(t, mux, "GET", "/api/v1/clients")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var clients []ClientJSON
	if err := json.NewDecoder(rec.Body).Decode(&clients); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(clients) < 2 {
		t.Errorf("expected at least 2 clients, got %d", len(clients))
	}
}

func TestClientShowEndpoint(t *testing.T) {
	deps := testDeps()
	mux := NewRouter(deps, slog.Default())

	rec := doRequest(t, mux, "GET", "/api/v1/clients/aa:bb:cc:dd:ee:01")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var client ClientJSON
	if err := json.NewDecoder(rec.Body).Decode(&client); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if client.Name != "server-1" {
		t.Errorf("name = %q, want %q", client.Name, "server-1")
	}
}

func TestClientShowNotFound(t *testing.T) {
	deps := testDeps()
	mux := NewRouter(deps, slog.Default())

	// With wildcard, unknown MACs fallback to default.
	rec := doRequest(t, mux, "GET", "/api/v1/clients/11:22:33:44:55:66")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (wildcard fallback)", rec.Code, http.StatusOK)
	}
}

func TestClientShowInvalidMAC(t *testing.T) {
	deps := testDeps()
	mux := NewRouter(deps, slog.Default())

	rec := doRequest(t, mux, "GET", "/api/v1/clients/not-a-mac")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestClientWakeEndpoint(t *testing.T) {
	deps := testDeps()
	mux := NewRouter(deps, slog.Default())

	rec := doRequest(t, mux, "POST", "/api/v1/clients/aa:bb:cc:dd:ee:01/wake")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestMenusEndpoint(t *testing.T) {
	deps := testDeps()
	mux := NewRouter(deps, slog.Default())

	rec := doRequest(t, mux, "GET", "/api/v1/menus")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var menus []MenuJSON
	if err := json.NewDecoder(rec.Body).Decode(&menus); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(menus) != 2 {
		t.Errorf("expected 2 menus, got %d", len(menus))
	}
}

func TestSessionsEndpoint(t *testing.T) {
	deps := testDeps()
	mux := NewRouter(deps, slog.Default())

	rec := doRequest(t, mux, "GET", "/api/v1/sessions")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var sessions []SessionJSON
	if err := json.NewDecoder(rec.Body).Decode(&sessions); err != nil {
		t.Fatalf("decoding: %v", err)
	}
}

func TestReloadEndpointNoHandler(t *testing.T) {
	deps := testDeps()
	// No ReloadFn set.
	mux := NewRouter(deps, slog.Default())

	rec := doRequest(t, mux, "POST", "/api/v1/reload")

	if rec.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotImplemented)
	}
}

func TestReloadEndpointWithHandler(t *testing.T) {
	deps := testDeps()
	deps.ReloadFn = func() error { return nil }
	mux := NewRouter(deps, slog.Default())

	rec := doRequest(t, mux, "POST", "/api/v1/reload")

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestTestEndpointNoHealth(t *testing.T) {
	deps := testDeps()
	// No Health checker set.
	mux := NewRouter(deps, slog.Default())

	rec := doRequest(t, mux, "POST", "/api/v1/test")

	if rec.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotImplemented)
	}
}

func TestJSONContentType(t *testing.T) {
	deps := testDeps()
	mux := NewRouter(deps, slog.Default())

	paths := []string{
		"/api/v1/status",
		"/api/v1/clients",
		"/api/v1/menus",
		"/api/v1/sessions",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			rec := doRequest(t, mux, "GET", path)
			ct := rec.Header().Get("Content-Type")
			if !strings.HasPrefix(ct, "application/json") {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
		})
	}
}
