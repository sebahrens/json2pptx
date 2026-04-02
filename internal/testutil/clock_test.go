package testutil

import (
	"testing"
	"time"
)

func TestMockClock_Now(t *testing.T) {
	ref := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	clock := NewMockClock(ref)

	got := clock.Now()
	if !got.Equal(ref) {
		t.Errorf("MockClock.Now() = %v, want %v", got, ref)
	}
}

func TestMockClock_DefaultTime(t *testing.T) {
	clock := NewMockClock()
	got := clock.Now()

	expected := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	if !got.Equal(expected) {
		t.Errorf("MockClock default Now() = %v, want %v", got, expected)
	}
}

func TestMockClock_Since(t *testing.T) {
	ref := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	clock := NewMockClock(ref)

	past := ref.Add(-5 * time.Minute)
	got := clock.Since(past)
	want := 5 * time.Minute

	if got != want {
		t.Errorf("MockClock.Since() = %v, want %v", got, want)
	}
}

func TestMockClock_Advance(t *testing.T) {
	ref := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	clock := NewMockClock(ref)

	clock.Advance(1 * time.Hour)

	got := clock.Now()
	want := ref.Add(1 * time.Hour)

	if !got.Equal(want) {
		t.Errorf("After Advance(1h), Now() = %v, want %v", got, want)
	}
}

func TestMockClock_Set(t *testing.T) {
	clock := NewMockClock()
	target := time.Date(2025, 12, 25, 0, 0, 0, 0, time.UTC)

	clock.Set(target)

	got := clock.Now()
	if !got.Equal(target) {
		t.Errorf("After Set(), Now() = %v, want %v", got, target)
	}
}

func TestMockClock_After_ImmediatelyPast(t *testing.T) {
	clock := NewMockClock()

	// Request a timer for 0 or negative duration
	ch := clock.After(0)

	select {
	case <-ch:
		// Expected
	default:
		t.Error("After(0) should send immediately")
	}
}

func TestMockClock_After_AdvanceTriggersWaiter(t *testing.T) {
	clock := NewMockClock()

	ch := clock.After(100 * time.Millisecond)

	// Should not be ready yet
	select {
	case <-ch:
		t.Error("After() should not trigger before Advance()")
	default:
		// Expected
	}

	// Advance past the deadline
	clock.Advance(100 * time.Millisecond)

	select {
	case <-ch:
		// Expected
	default:
		t.Error("After() should trigger after Advance() past deadline")
	}
}

func TestMockClock_After_MultipleWaiters(t *testing.T) {
	clock := NewMockClock()

	ch1 := clock.After(50 * time.Millisecond)
	ch2 := clock.After(100 * time.Millisecond)
	ch3 := clock.After(150 * time.Millisecond)

	// Advance to 75ms - only ch1 should trigger
	clock.Advance(75 * time.Millisecond)

	select {
	case <-ch1:
		// Expected
	default:
		t.Error("ch1 should trigger at 75ms")
	}

	select {
	case <-ch2:
		t.Error("ch2 should not trigger at 75ms")
	default:
		// Expected
	}

	select {
	case <-ch3:
		t.Error("ch3 should not trigger at 75ms")
	default:
		// Expected
	}

	// Advance to 125ms - ch2 should trigger
	clock.Advance(50 * time.Millisecond)

	select {
	case <-ch2:
		// Expected
	default:
		t.Error("ch2 should trigger at 125ms")
	}

	select {
	case <-ch3:
		t.Error("ch3 should not trigger at 125ms")
	default:
		// Expected
	}

	// Advance to 175ms - ch3 should trigger
	clock.Advance(50 * time.Millisecond)

	select {
	case <-ch3:
		// Expected
	default:
		t.Error("ch3 should trigger at 175ms")
	}
}

func TestMockClock_Concurrency(t *testing.T) {
	clock := NewMockClock()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			clock.Advance(1 * time.Millisecond)
		}
		close(done)
	}()

	for i := 0; i < 100; i++ {
		_ = clock.Now()
		_ = clock.Since(time.Time{})
	}

	<-done
}
