// Package httpboot serves boot files (kernel, initrd, iPXE scripts) over HTTP.
package httpboot

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"bootforge/internal/domain"
)

// HTTPServer serves boot files and iPXE scripts over HTTP.
type HTTPServer struct {
	cfg    domain.HTTPConfig
	server *http.Server
	mux    *http.ServeMux
	logger *slog.Logger
}

// NewHTTPServer creates a new HTTP boot server.
func NewHTTPServer(cfg domain.HTTPConfig, mux *http.ServeMux, logger *slog.Logger) *HTTPServer {
	readTimeout := cfg.ReadTimeout
	if readTimeout == 0 {
		readTimeout = 30 * time.Second
	}

	srv := &http.Server{
		Addr:        fmt.Sprintf(":%d", cfg.Port),
		Handler:     mux,
		ReadTimeout: readTimeout,
	}

	return &HTTPServer{
		cfg:    cfg,
		server: srv,
		mux:    mux,
		logger: logger,
	}
}

// Start begins serving HTTP requests. It blocks until the server is stopped.
func (s *HTTPServer) Start() error {
	s.logger.Info("HTTP boot server starting", "addr", s.server.Addr)
	var err error
	if s.cfg.TLS.Enabled {
		err = s.server.ListenAndServeTLS(s.cfg.TLS.Cert, s.cfg.TLS.Key)
	} else {
		err = s.server.ListenAndServe()
	}
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// StartOnListener begins serving on the given listener. Used for testing
// with ephemeral ports.
func (s *HTTPServer) StartOnListener(ln net.Listener) error {
	s.logger.Info("HTTP boot server starting", "addr", ln.Addr())
	err := s.server.Serve(ln)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Stop gracefully shuts down the HTTP server.
func (s *HTTPServer) Stop() error {
	s.logger.Info("HTTP boot server stopping")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}
