package health

import (
	"fmt"
	"time"

	"bootforge/internal/domain"

	"github.com/pin/tftp/v3"
)

// TFTPProbe checks that the TFTP server can serve a bootloader file.
type TFTPProbe struct {
	addr     string
	filename string
	timeout  time.Duration
}

// NewTFTPProbe creates a probe that reads a file from the TFTP server.
func NewTFTPProbe(addr, filename string, timeout time.Duration) *TFTPProbe {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &TFTPProbe{addr: addr, filename: filename, timeout: timeout}
}

func (p *TFTPProbe) Name() string { return "tftp" }

func (p *TFTPProbe) Check() domain.CheckResult {
	start := time.Now()

	client, err := tftp.NewClient(p.addr)
	result := domain.CheckResult{
		Name:     "tftp",
		Duration: time.Since(start),
		At:       start,
	}

	if err != nil {
		result.Status = domain.StatusFail
		result.Message = fmt.Sprintf("TFTP client %s: %v", p.addr, err)
		return result
	}
	client.SetTimeout(p.timeout)

	_, err = client.Receive(p.filename, "octet")
	result.Duration = time.Since(start)

	if err != nil {
		result.Status = domain.StatusFail
		result.Message = fmt.Sprintf("TFTP read %s from %s: %v", p.filename, p.addr, err)
	} else {
		result.Status = domain.StatusOK
		result.Message = fmt.Sprintf("TFTP read %s from %s: OK (%s)", p.filename, p.addr, result.Duration.Round(time.Millisecond))
	}

	return result
}
