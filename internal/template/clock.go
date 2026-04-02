package template

import "time"

// Clock provides time-related functions that can be mocked in tests.
type Clock interface {
	Now() time.Time
	Since(t time.Time) time.Duration
	After(d time.Duration) <-chan time.Time
}

// RealClock uses the real system time.
type RealClock struct{}

// Now returns the current time.
func (RealClock) Now() time.Time {
	return time.Now()
}

// Since returns the time elapsed since t.
func (RealClock) Since(t time.Time) time.Duration {
	return time.Since(t)
}

// After waits for the duration to elapse and then sends the current time on the channel.
func (RealClock) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}
