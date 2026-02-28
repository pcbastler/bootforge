package health

import (
	"log/slog"
	"testing"
	"time"

	"bootforge/internal/domain"
)

// mockProbe is a test probe that returns a configurable result.
type mockProbe struct {
	name   string
	status domain.CheckStatus
	msg    string
}

func (p *mockProbe) Name() string { return p.name }
func (p *mockProbe) Check() domain.CheckResult {
	return domain.CheckResult{
		Name:    p.name,
		Status:  p.status,
		Message: p.msg,
		At:      time.Now(),
	}
}

func TestCheckerAllOK(t *testing.T) {
	probes := []Probe{
		&mockProbe{name: "probe-a", status: domain.StatusOK, msg: "ok"},
		&mockProbe{name: "probe-b", status: domain.StatusOK, msg: "ok"},
	}
	store := NewResultStore(10)
	checker := NewChecker(probes, store, time.Minute, slog.Default())

	result := checker.RunOnce()

	if result.Status != domain.StatusOK {
		t.Errorf("overall status = %v, want OK", result.Status)
	}
	if len(result.Checks) != 2 {
		t.Errorf("checks = %d, want 2", len(result.Checks))
	}
}

func TestCheckerOneFail(t *testing.T) {
	probes := []Probe{
		&mockProbe{name: "probe-ok", status: domain.StatusOK, msg: "ok"},
		&mockProbe{name: "probe-fail", status: domain.StatusFail, msg: "broken"},
	}
	store := NewResultStore(10)
	checker := NewChecker(probes, store, time.Minute, slog.Default())

	result := checker.RunOnce()

	if result.Status != domain.StatusFail {
		t.Errorf("overall status = %v, want Fail", result.Status)
	}
}

func TestCheckerOneWarn(t *testing.T) {
	probes := []Probe{
		&mockProbe{name: "probe-ok", status: domain.StatusOK, msg: "ok"},
		&mockProbe{name: "probe-warn", status: domain.StatusWarn, msg: "low disk"},
	}
	store := NewResultStore(10)
	checker := NewChecker(probes, store, time.Minute, slog.Default())

	result := checker.RunOnce()

	if result.Status != domain.StatusWarn {
		t.Errorf("overall status = %v, want Warn", result.Status)
	}
}

func TestCheckerStoresResult(t *testing.T) {
	probes := []Probe{
		&mockProbe{name: "test", status: domain.StatusOK, msg: "ok"},
	}
	store := NewResultStore(10)
	checker := NewChecker(probes, store, time.Minute, slog.Default())

	if checker.Latest() != nil {
		t.Error("should have no results initially")
	}

	checker.RunOnce()
	latest := checker.Latest()
	if latest == nil {
		t.Fatal("should have a result after RunOnce")
	}
	if latest.Status != domain.StatusOK {
		t.Errorf("status = %v, want OK", latest.Status)
	}
}

func TestResultStoreCapacity(t *testing.T) {
	store := NewResultStore(3)

	for i := 0; i < 5; i++ {
		store.Add(domain.HealthResult{Status: domain.StatusOK, At: time.Now()})
	}

	recent := store.Recent(10)
	if len(recent) != 3 {
		t.Errorf("recent = %d, want 3 (capacity)", len(recent))
	}
}

func TestResultStoreRecent(t *testing.T) {
	store := NewResultStore(10)

	for i := 0; i < 3; i++ {
		store.Add(domain.HealthResult{
			Status: domain.CheckStatus(i),
			At:     time.Now(),
		})
	}

	recent := store.Recent(2)
	if len(recent) != 2 {
		t.Fatalf("recent = %d, want 2", len(recent))
	}
	// Newest first.
	if recent[0].Status != domain.StatusFail { // StatusFail = 2
		t.Errorf("first result status = %v, want Fail (newest)", recent[0].Status)
	}
}
