// Package health implements self-tests and diagnostics for Bootforge.
// It runs periodic probes against DHCP, TFTP, HTTP, files, and disk.
package health

import (
	"context"
	"log/slog"
	"time"

	"bootforge/internal/domain"
)

// Checker runs health probes periodically and on demand.
type Checker struct {
	probes   []Probe
	store    *ResultStore
	interval time.Duration
	logger   *slog.Logger
}

// NewChecker creates a new health checker.
func NewChecker(probes []Probe, store *ResultStore, interval time.Duration, logger *slog.Logger) *Checker {
	if interval == 0 {
		interval = 30 * time.Second
	}
	return &Checker{
		probes:   probes,
		store:    store,
		interval: interval,
		logger:   logger,
	}
}

// RunOnce executes all probes and returns the aggregated result.
func (c *Checker) RunOnce() domain.HealthResult {
	checks := make([]domain.CheckResult, 0, len(c.probes))
	for _, p := range c.probes {
		result := p.Check()
		checks = append(checks, result)

		if result.Status == domain.StatusFail {
			c.logger.Warn("health check failed",
				"probe", result.Name,
				"message", result.Message,
				"duration", result.Duration,
			)
		}
	}

	hr := aggregate(checks)
	c.store.Add(hr)
	return hr
}

// Start runs the checker periodically until the context is cancelled.
func (c *Checker) Start(ctx context.Context) {
	c.logger.Info("health checker starting", "interval", c.interval, "probes", len(c.probes))

	// Run immediately on start.
	c.RunOnce()

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("health checker stopping")
			return
		case <-ticker.C:
			c.RunOnce()
		}
	}
}

// Latest returns the most recent health result.
func (c *Checker) Latest() *domain.HealthResult {
	return c.store.Latest()
}
