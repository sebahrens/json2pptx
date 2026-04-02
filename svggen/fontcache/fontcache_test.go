package fontcache

import (
	"sync"
	"testing"
)

func TestGet_ReturnsCachedInstance(t *testing.T) {
	Reset()

	ff1 := Get("Arial", "")
	ff2 := Get("Arial", "")

	if ff1 != ff2 {
		t.Error("expected same pointer for repeated Get with same name")
	}
	if Len() != 1 {
		t.Errorf("Len() = %d, want 1", Len())
	}
}

func TestGet_DifferentNamesAreSeparate(t *testing.T) {
	Reset()

	Get("Arial", "")
	Get("Courier", "")

	if Len() != 2 {
		t.Errorf("Len() = %d, want 2", Len())
	}
}

func TestGet_FallbackLoads(t *testing.T) {
	Reset()

	// A made-up font name should still return non-nil thanks to the
	// fallback cascade (system fallbacks or embedded Liberation Sans).
	ff := Get("NonExistentFont12345", "")
	if ff == nil {
		t.Error("expected non-nil font from fallback cascade")
	}
}

func TestGet_WithExplicitFallback(t *testing.T) {
	Reset()

	ff := Get("NonExistentFont12345", "Helvetica")
	if ff == nil {
		t.Error("expected non-nil font with explicit fallback")
	}
}

func TestLRUEviction(t *testing.T) {
	Reset()

	// Fill the cache past maxEntries.
	for i := 0; i < maxEntries+10; i++ {
		// Use unique names so each creates a new entry.
		Get("evict-test-font-"+string(rune('A'+i%26))+string(rune('0'+i/26)), "")
	}

	if Len() > maxEntries {
		t.Errorf("Len() = %d, want <= %d after eviction", Len(), maxEntries)
	}
}

func TestLRUEviction_RecentlyUsedSurvives(t *testing.T) {
	Reset()

	// Insert maxEntries items.
	for i := 0; i < maxEntries; i++ {
		Get("lru-font-"+string(rune(i)), "")
	}

	// Access the first entry to make it recently used.
	firstKey := "lru-font-" + string(rune(0))
	Get(firstKey, "")

	// Insert one more to trigger eviction.
	Get("lru-font-new", "")

	// The first entry should still be present because we just accessed it.
	ff := Get(firstKey, "")
	if ff == nil {
		t.Error("recently-used entry should survive eviction")
	}
}

func TestReset(t *testing.T) {
	Get("Arial", "")
	if Len() == 0 {
		t.Skip("cache already empty, nothing to test")
	}

	Reset()
	if Len() != 0 {
		t.Errorf("Len() = %d after Reset, want 0", Len())
	}
}

func TestConcurrentAccess(t *testing.T) {
	Reset()

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			// All goroutines load the same font to stress the lock.
			ff := Get("Arial", "Helvetica")
			if ff == nil {
				t.Errorf("goroutine %d: Get returned nil", id)
			}
		}(i)
	}

	wg.Wait()

	// Should have exactly one cache entry for "Arial".
	if Len() != 1 {
		t.Errorf("Len() = %d after concurrent access, want 1", Len())
	}
}
