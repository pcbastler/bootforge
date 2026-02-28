package health

import (
	"fmt"
	"net/http"
	"time"

	"bootforge/internal/domain"
)

// HTTPProbe checks that the HTTP server is responding.
type HTTPProbe struct {
	url     string
	timeout time.Duration
}

// NewHTTPProbe creates a probe that checks the HTTP health endpoint.
func NewHTTPProbe(url string, timeout time.Duration) *HTTPProbe {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &HTTPProbe{url: url, timeout: timeout}
}

func (p *HTTPProbe) Name() string { return "http" }

func (p *HTTPProbe) Check() domain.CheckResult {
	start := time.Now()

	client := &http.Client{Timeout: p.timeout}
	resp, err := client.Get(p.url)

	result := domain.CheckResult{
		Name:     "http",
		Duration: time.Since(start),
		At:       start,
	}

	if err != nil {
		result.Status = domain.StatusFail
		result.Message = fmt.Sprintf("GET %s: %v", p.url, err)
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		result.Status = domain.StatusFail
		result.Message = fmt.Sprintf("GET %s: status %d", p.url, resp.StatusCode)
	} else {
		result.Status = domain.StatusOK
		result.Message = fmt.Sprintf("GET %s: %d (%s)", p.url, resp.StatusCode, result.Duration.Round(time.Millisecond))
	}

	return result
}
