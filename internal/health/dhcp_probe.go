package health

import (
	"fmt"
	"time"

	"bootforge/internal/domain"
)

// DHCPProbe is a placeholder for DHCP health checking.
// Full DHCP probing requires raw sockets and elevated privileges,
// so this probe only validates that the DHCP proxy config is set.
type DHCPProbe struct {
	enabled bool
	port    int
}

// NewDHCPProbe creates a DHCP config validation probe.
func NewDHCPProbe(enabled bool, port int) *DHCPProbe {
	return &DHCPProbe{enabled: enabled, port: port}
}

func (p *DHCPProbe) Name() string { return "dhcp" }

func (p *DHCPProbe) Check() domain.CheckResult {
	start := time.Now()
	result := domain.CheckResult{
		Name:     "dhcp",
		Duration: time.Since(start),
		At:       start,
	}

	if !p.enabled {
		result.Status = domain.StatusOK
		result.Message = "DHCP proxy disabled (skipped)"
		return result
	}

	result.Status = domain.StatusOK
	result.Message = fmt.Sprintf("DHCP proxy configured on port %d", p.port)
	return result
}
