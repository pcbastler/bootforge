package httpboot

import (
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
)

// SetupRoutes registers all boot-related HTTP routes on the mux.
func SetupRoutes(mux *http.ServeMux, handler *MenuScriptHandler, dataDir string, logger *slog.Logger) {
	// iPXE menu script per client.
	mux.HandleFunc("GET /boot/{mac}/menu.ipxe", handler.ServeHTTP)

	// Static boot files from data directory.
	// Serves files at /data/* from the data directory.
	fs := &safeFileServer{
		root:   http.Dir(dataDir),
		prefix: "/data/",
		logger: logger,
	}
	mux.Handle("GET /data/", http.StripPrefix("/data/", fs))

	// Health endpoint.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
}

// safeFileServer is a file server that rejects path traversal attempts.
type safeFileServer struct {
	root   http.FileSystem
	prefix string
	logger *slog.Logger
}

func (s *safeFileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Clean and validate the path.
	cleaned := filepath.Clean(r.URL.Path)
	if strings.Contains(cleaned, "..") {
		s.logger.Warn("HTTP path traversal rejected", "path", r.URL.Path)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	http.FileServer(s.root).ServeHTTP(w, r)
}
