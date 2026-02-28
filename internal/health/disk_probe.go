package health

import (
	"fmt"
	"time"

	"bootforge/internal/domain"

	"golang.org/x/sys/unix"
)

// DiskProbe checks available disk space on the data directory.
type DiskProbe struct {
	path         string
	minFreeBytes uint64
}

// NewDiskProbe creates a probe that checks disk space.
// minFreeBytes is the minimum free space required (default: 100MB).
func NewDiskProbe(path string, minFreeBytes uint64) *DiskProbe {
	if minFreeBytes == 0 {
		minFreeBytes = 100 * 1024 * 1024 // 100MB
	}
	return &DiskProbe{path: path, minFreeBytes: minFreeBytes}
}

func (p *DiskProbe) Name() string { return "disk" }

func (p *DiskProbe) Check() domain.CheckResult {
	start := time.Now()

	var stat unix.Statfs_t
	err := unix.Statfs(p.path, &stat)

	result := domain.CheckResult{
		Name:     "disk",
		Duration: time.Since(start),
		At:       start,
	}

	if err != nil {
		result.Status = domain.StatusFail
		result.Message = fmt.Sprintf("statfs %s: %v", p.path, err)
		return result
	}

	freeBytes := stat.Bavail * uint64(stat.Bsize)
	freeMB := freeBytes / (1024 * 1024)

	if freeBytes < p.minFreeBytes {
		result.Status = domain.StatusWarn
		result.Message = fmt.Sprintf("%s: %d MB free (minimum: %d MB)", p.path, freeMB, p.minFreeBytes/(1024*1024))
	} else {
		result.Status = domain.StatusOK
		result.Message = fmt.Sprintf("%s: %d MB free", p.path, freeMB)
	}

	return result
}
