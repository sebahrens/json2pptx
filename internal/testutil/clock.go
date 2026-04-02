// Package testutil provides testing utilities for the go-slide-creator project.
package testutil

import (
	"sync"
	"time"
)

// MockClock is a controllable clock for testing.
// It allows tests to advance time without actual waits.
// MockClock satisfies the template.Clock interface via structural typing.
type MockClock struct {
	mu      sync.RWMutex
	current time.Time
	waiters []waiter
}

type waiter struct {
	deadline time.Time
	ch       chan time.Time
}

// NewMockClock creates a new mock clock set to the given time.
// If no time is provided, uses a fixed reference time.
func NewMockClock(t ...time.Time) *MockClock {
	var current time.Time
	if len(t) > 0 {
		current = t[0]
	} else {
		// Fixed reference time for reproducibility
		current = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	}
	return &MockClock{current: current}
}

// Now returns the mock clock's current time.
func (m *MockClock) Now() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

// Since returns the duration since t using the mock clock's current time.
func (m *MockClock) Since(t time.Time) time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current.Sub(t)
}

// After returns a channel that receives the time after the duration.
// The channel only sends when Advance is called.
func (m *MockClock) After(d time.Duration) <-chan time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan time.Time, 1)
	deadline := m.current.Add(d)

	// If deadline already passed, send immediately
	if !deadline.After(m.current) {
		ch <- m.current
		return ch
	}

	m.waiters = append(m.waiters, waiter{deadline: deadline, ch: ch})
	return ch
}

// Advance moves the mock clock forward by the given duration.
// Any waiters whose deadline has passed will be notified.
func (m *MockClock) Advance(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.current = m.current.Add(d)

	// Notify any waiters whose deadline has passed
	remaining := make([]waiter, 0, len(m.waiters))
	for _, w := range m.waiters {
		if !w.deadline.After(m.current) {
			select {
			case w.ch <- m.current:
			default:
			}
		} else {
			remaining = append(remaining, w)
		}
	}
	m.waiters = remaining
}

// Set sets the mock clock to a specific time.
func (m *MockClock) Set(t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current = t
}
