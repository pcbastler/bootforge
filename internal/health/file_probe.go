package health

import (
	"fmt"
	"os"
	"time"

	"bootforge/internal/domain"
)

// FileProbe checks that required files exist and are readable.
type FileProbe struct {
	name  string
	paths []string
}

// NewFileProbe creates a probe that checks the given files exist.
func NewFileProbe(name string, paths []string) *FileProbe {
	return &FileProbe{name: name, paths: paths}
}

func (p *FileProbe) Name() string { return p.name }

func (p *FileProbe) Check() domain.CheckResult {
	start := time.Now()

	var missing []string
	for _, path := range p.paths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			missing = append(missing, path)
		}
	}

	result := domain.CheckResult{
		Name:     p.name,
		Duration: time.Since(start),
		At:       start,
	}

	if len(missing) > 0 {
		result.Status = domain.StatusFail
		result.Message = fmt.Sprintf("%d of %d files missing: %v", len(missing), len(p.paths), missing)
	} else {
		result.Status = domain.StatusOK
		result.Message = fmt.Sprintf("all %d files present", len(p.paths))
	}

	return result
}
