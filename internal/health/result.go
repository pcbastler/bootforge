package health

import (
	"sync"
	"time"

	"bootforge/internal/domain"
)

// ResultStore keeps the most recent health check results.
type ResultStore struct {
	mu      sync.RWMutex
	results []domain.HealthResult
	maxSize int
}

// NewResultStore creates a result store with the given capacity.
func NewResultStore(maxSize int) *ResultStore {
	if maxSize <= 0 {
		maxSize = 100
	}
	return &ResultStore{
		results: make([]domain.HealthResult, 0, maxSize),
		maxSize: maxSize,
	}
}

// Add stores a new health result, evicting the oldest if at capacity.
func (s *ResultStore) Add(result domain.HealthResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.results) >= s.maxSize {
		s.results = s.results[1:]
	}
	s.results = append(s.results, result)
}

// Latest returns the most recent health result, or nil if no results.
func (s *ResultStore) Latest() *domain.HealthResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.results) == 0 {
		return nil
	}
	r := s.results[len(s.results)-1]
	return &r
}

// Recent returns the N most recent results, newest first.
func (s *ResultStore) Recent(n int) []domain.HealthResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if n <= 0 || len(s.results) == 0 {
		return nil
	}
	if n > len(s.results) {
		n = len(s.results)
	}
	result := make([]domain.HealthResult, n)
	for i := 0; i < n; i++ {
		result[i] = s.results[len(s.results)-1-i]
	}
	return result
}

// aggregate computes the overall status from individual check results.
func aggregate(checks []domain.CheckResult) domain.HealthResult {
	status := domain.StatusOK
	for _, c := range checks {
		if c.Status > status {
			status = c.Status
		}
	}
	return domain.HealthResult{
		Status: status,
		Checks: checks,
		At:     time.Now(),
	}
}
