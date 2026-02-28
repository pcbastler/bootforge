// Package server orchestrates Bootforge's lifecycle, config, registry,
// sessions, and event bus. It contains no network I/O.
package server

import (
	"context"
	"fmt"
	"log/slog"

	"bootforge/internal/domain"
)

// Service is the interface that network services (DHCP, TFTP, HTTP) implement.
type Service interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// Server is the main Bootforge server that orchestrates all components.
type Server struct {
	Config   *ConfigManager
	Registry *Registry
	Menus    *MenuResolver
	Sessions *SessionManager
	Events   *EventBus
	IPXE     *IPXEGenerator
	Logger   *slog.Logger

	services []Service
}

// ServerDeps holds the dependencies needed to construct a Server.
type ServerDeps struct {
	ConfigManager *ConfigManager
	SessionStore  domain.SessionStore
	Logger        *slog.Logger
}

// New creates a new Server from the loaded configuration and dependencies.
func New(deps ServerDeps) (*Server, error) {
	cfg := deps.ConfigManager.Config()
	if cfg == nil {
		return nil, fmt.Errorf("config not loaded")
	}

	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Server{
		Config:   deps.ConfigManager,
		Registry: NewRegistry(cfg.Clients),
		Menus:    NewMenuResolver(cfg.Menus),
		Sessions: NewSessionManager(deps.SessionStore),
		Events:   NewEventBus(64),
		IPXE:     NewIPXEGenerator(),
		Logger:   logger,
	}, nil
}

// RegisterService adds a network service to the server's lifecycle.
func (s *Server) RegisterService(svc Service) {
	s.services = append(s.services, svc)
}

// Start launches all registered services.
func (s *Server) Start(ctx context.Context) error {
	for _, svc := range s.services {
		s.Logger.Info("starting service", "service", svc.Name())
		if err := svc.Start(ctx); err != nil {
			return fmt.Errorf("starting %s: %w", svc.Name(), err)
		}
	}
	return nil
}

// Stop shuts down all registered services in reverse order.
func (s *Server) Stop(ctx context.Context) error {
	var firstErr error
	for i := len(s.services) - 1; i >= 0; i-- {
		svc := s.services[i]
		s.Logger.Info("stopping service", "service", svc.Name())
		if err := svc.Stop(ctx); err != nil {
			s.Logger.Error("error stopping service", "service", svc.Name(), "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// Reload re-reads the configuration and updates the registry and menu resolver.
func (s *Server) Reload() error {
	cfg, err := s.Config.Reload()
	if err != nil {
		return err
	}

	s.Registry.Reload(cfg.Clients)
	s.Menus.Reload(cfg.Menus)

	s.Logger.Info("configuration reloaded",
		"menus", len(cfg.Menus),
		"clients", len(cfg.Clients),
	)
	return nil
}
