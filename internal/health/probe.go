package health

import "bootforge/internal/domain"

// Probe runs a single health check and returns a result.
type Probe interface {
	// Name returns the probe's display name.
	Name() string
	// Check runs the probe and returns a result.
	Check() domain.CheckResult
}
