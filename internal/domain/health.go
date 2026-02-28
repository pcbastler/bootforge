package domain

import (
	"fmt"
	"time"
)

// CheckStatus represents the result status of a health check.
type CheckStatus int

const (
	StatusOK   CheckStatus = iota // Check passed
	StatusWarn                    // Check passed with warnings
	StatusFail                    // Check failed
)

var checkStatusNames = map[CheckStatus]string{
	StatusOK:   "ok",
	StatusWarn: "warn",
	StatusFail: "fail",
}

func (s CheckStatus) String() string {
	if n, ok := checkStatusNames[s]; ok {
		return n
	}
	return fmt.Sprintf("CheckStatus(%d)", int(s))
}

// CheckResult is the outcome of a single health check probe.
type CheckResult struct {
	Name     string
	Status   CheckStatus
	Message  string
	Duration time.Duration
	At       time.Time
}

// HealthResult aggregates all probe results into a single health report.
type HealthResult struct {
	Status CheckStatus   // overall status (worst of all checks)
	Checks []CheckResult // individual check results
	At     time.Time
}
