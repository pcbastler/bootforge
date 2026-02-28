package server

import (
	"fmt"
	"sync"

	"bootforge/internal/domain"
)

// ConfigManager holds the current configuration and supports hot-reload.
type ConfigManager struct {
	mu     sync.RWMutex
	config *domain.FullConfig
	loader func(dir string) (*domain.FullConfig, error)
	dir    string
}

// NewConfigManager creates a new config manager.
func NewConfigManager(loader func(dir string) (*domain.FullConfig, error)) *ConfigManager {
	return &ConfigManager{
		loader: loader,
	}
}

// Load reads and validates the configuration from the given directory.
func (m *ConfigManager) Load(dir string) error {
	cfg, err := m.loader(dir)
	if err != nil {
		return fmt.Errorf("loading config from %s: %w", dir, err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validating config: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg
	m.dir = dir
	return nil
}

// Reload re-reads the configuration from the same directory.
func (m *ConfigManager) Reload() (*domain.FullConfig, error) {
	m.mu.RLock()
	dir := m.dir
	m.mu.RUnlock()

	if dir == "" {
		return nil, fmt.Errorf("config not loaded yet")
	}

	cfg, err := m.loader(dir)
	if err != nil {
		return nil, fmt.Errorf("reloading config from %s: %w", dir, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating reloaded config: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg
	return cfg, nil
}

// Config returns the current configuration. Returns nil if not loaded.
func (m *ConfigManager) Config() *domain.FullConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}
