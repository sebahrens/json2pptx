package template

import (
	"testing"
	"time"
)

func TestRealClock_Now(t *testing.T) {
	clock := RealClock{}
	before := time.Now()
	now := clock.Now()
	after := time.Now()

	if now.Before(before) || now.After(after) {
		t.Errorf("RealClock.Now() returned time outside expected range")
	}
}

func TestRealClock_Since(t *testing.T) {
	clock := RealClock{}
	start := time.Now()
	// Small busy wait to ensure some time passes
	for i := 0; i < 1000; i++ {
		_ = i * i
	}
	elapsed := clock.Since(start)

	if elapsed < 0 {
		t.Errorf("RealClock.Since() returned negative duration: %v", elapsed)
	}
}
