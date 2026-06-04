package clock

import (
	"sync"
	"time"
)

// Clock makes time-dependent policy behavior testable.
type Clock interface {
	Now() time.Time
}

// Manual is a concurrency-safe controllable clock for tests and simulations.
type Manual struct {
	mu  sync.RWMutex
	now time.Time
}

func NewManual(now time.Time) *Manual {
	return &Manual{now: now}
}

func (m *Manual) Now() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.now
}

func (m *Manual) Set(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = now
}

func (m *Manual) Advance(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = m.now.Add(d)
}
