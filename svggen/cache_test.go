package svggen

import (
	"testing"
	"time"
)

func TestRenderCache_GetSet(t *testing.T) {
	cache := NewRenderCache(CacheConfig{
		TTL:             1 * time.Minute,
		MaxEntries:      100,
		CleanupInterval: 1 * time.Minute,
	})
	defer cache.Stop()

	req := &RequestEnvelope{
		Type: "bar_chart",
		Data: map[string]any{
			"values": []any{1.0, 2.0, 3.0},
		},
	}

	result := &RenderResult{
		SVG: &SVGDocument{
			Content: []byte("<svg>test</svg>"),
			Width:   800,
			Height:  600,
		},
		Format: "svg",
		Width:  800,
		Height: 600,
	}

	// Initially not in cache
	if got := cache.Get(req); got != nil {
		t.Errorf("Get() = %v, want nil for missing key", got)
	}

	// Set and retrieve
	cache.Set(req, result)
	got := cache.Get(req)
	if got == nil {
		t.Fatal("Get() = nil, want result")
	}
	if got.Format != result.Format {
		t.Errorf("Get().Format = %v, want %v", got.Format, result.Format)
	}
	if got.Width != result.Width {
		t.Errorf("Get().Width = %v, want %v", got.Width, result.Width)
	}
}

func TestRenderCache_DifferentRequests(t *testing.T) {
	cache := NewRenderCache(CacheConfig{
		TTL:             1 * time.Minute,
		MaxEntries:      100,
		CleanupInterval: 1 * time.Minute,
	})
	defer cache.Stop()

	req1 := &RequestEnvelope{
		Type: "bar_chart",
		Data: map[string]any{"values": []any{1.0, 2.0}},
	}
	req2 := &RequestEnvelope{
		Type: "pie_chart",
		Data: map[string]any{"values": []any{1.0, 2.0}},
	}

	result1 := &RenderResult{Format: "svg", Width: 800, Height: 600}
	result2 := &RenderResult{Format: "svg", Width: 400, Height: 300}

	cache.Set(req1, result1)
	cache.Set(req2, result2)

	got1 := cache.Get(req1)
	got2 := cache.Get(req2)

	if got1.Width != 800 {
		t.Errorf("req1 result Width = %v, want 800", got1.Width)
	}
	if got2.Width != 400 {
		t.Errorf("req2 result Width = %v, want 400", got2.Width)
	}
}

func TestRenderCache_TTLExpiry(t *testing.T) {
	cache := NewRenderCache(CacheConfig{
		TTL:             50 * time.Millisecond,
		MaxEntries:      100,
		CleanupInterval: 10 * time.Millisecond,
	})
	defer cache.Stop()

	req := &RequestEnvelope{
		Type: "bar_chart",
		Data: map[string]any{"values": []any{1.0}},
	}
	result := &RenderResult{Format: "svg", Width: 800, Height: 600}

	cache.Set(req, result)

	// Should be in cache initially
	if got := cache.Get(req); got == nil {
		t.Error("Get() = nil immediately after Set, want result")
	}

	// Wait for TTL to expire
	time.Sleep(100 * time.Millisecond)

	// Should be expired
	if got := cache.Get(req); got != nil {
		t.Errorf("Get() = %v after TTL, want nil", got)
	}
}

func TestRenderCache_MaxEntries(t *testing.T) {
	cache := NewRenderCache(CacheConfig{
		TTL:             1 * time.Minute,
		MaxEntries:      3,
		CleanupInterval: 1 * time.Minute,
	})
	defer cache.Stop()

	// Add 5 entries to a cache with max 3
	for i := 0; i < 5; i++ {
		req := &RequestEnvelope{
			Type: "chart",
			Data: map[string]any{"id": float64(i)},
		}
		result := &RenderResult{Format: "svg", Width: i}
		cache.Set(req, result)
	}

	stats := cache.Stats()
	if stats.Entries > 3 {
		t.Errorf("Stats().Entries = %d, want <= 3", stats.Entries)
	}
	if stats.Evictions < 2 {
		t.Errorf("Stats().Evictions = %d, want >= 2", stats.Evictions)
	}
}

func TestRenderCache_Stats(t *testing.T) {
	cache := NewRenderCache(CacheConfig{
		TTL:             1 * time.Minute,
		MaxEntries:      100,
		CleanupInterval: 1 * time.Minute,
	})
	defer cache.Stop()

	req := &RequestEnvelope{
		Type: "bar_chart",
		Data: map[string]any{"values": []any{1.0}},
	}
	result := &RenderResult{
		SVG: &SVGDocument{
			Content: []byte("<svg>test</svg>"),
		},
		Format: "svg",
	}

	// Initial stats
	stats := cache.Stats()
	if stats.Hits != 0 || stats.Misses != 0 {
		t.Errorf("Initial stats: hits=%d, misses=%d, want 0, 0", stats.Hits, stats.Misses)
	}

	// Cache miss
	cache.Get(req)
	stats = cache.Stats()
	if stats.Misses != 1 {
		t.Errorf("After miss: Misses = %d, want 1", stats.Misses)
	}

	// Set and hit
	cache.Set(req, result)
	cache.Get(req)
	cache.Get(req)
	stats = cache.Stats()
	if stats.Hits != 2 {
		t.Errorf("After hits: Hits = %d, want 2", stats.Hits)
	}
	if stats.Entries != 1 {
		t.Errorf("After set: Entries = %d, want 1", stats.Entries)
	}
}

func TestRenderCache_HitRate(t *testing.T) {
	tests := []struct {
		name   string
		hits   uint64
		misses uint64
		want   float64
	}{
		{"no requests", 0, 0, 0},
		{"all hits", 10, 0, 100},
		{"all misses", 0, 10, 0},
		{"50/50", 5, 5, 50},
		{"75% hits", 75, 25, 75},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := CacheStats{Hits: tt.hits, Misses: tt.misses}
			got := stats.HitRate()
			if got != tt.want {
				t.Errorf("HitRate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRenderCache_Invalidate(t *testing.T) {
	cache := NewRenderCache(CacheConfig{
		TTL:             1 * time.Minute,
		MaxEntries:      100,
		CleanupInterval: 1 * time.Minute,
	})
	defer cache.Stop()

	req := &RequestEnvelope{
		Type: "bar_chart",
		Data: map[string]any{"values": []any{1.0}},
	}
	result := &RenderResult{Format: "svg"}

	cache.Set(req, result)
	if got := cache.Get(req); got == nil {
		t.Fatal("Get() = nil after Set, want result")
	}

	cache.Invalidate(req)
	if got := cache.Get(req); got != nil {
		t.Errorf("Get() = %v after Invalidate, want nil", got)
	}
}

func TestRenderCache_Clear(t *testing.T) {
	cache := NewRenderCache(CacheConfig{
		TTL:             1 * time.Minute,
		MaxEntries:      100,
		CleanupInterval: 1 * time.Minute,
	})
	defer cache.Stop()

	// Add several entries
	for i := 0; i < 5; i++ {
		req := &RequestEnvelope{
			Type: "chart",
			Data: map[string]any{"id": float64(i)},
		}
		cache.Set(req, &RenderResult{Format: "svg"})
	}

	stats := cache.Stats()
	if stats.Entries != 5 {
		t.Errorf("Entries before Clear = %d, want 5", stats.Entries)
	}

	cache.Clear()
	stats = cache.Stats()
	if stats.Entries != 0 {
		t.Errorf("Entries after Clear = %d, want 0", stats.Entries)
	}
}

func TestRenderCache_ConcurrentAccess(t *testing.T) {
	cache := NewRenderCache(CacheConfig{
		TTL:             1 * time.Minute,
		MaxEntries:      1000,
		CleanupInterval: 1 * time.Minute,
	})
	defer cache.Stop()

	// Concurrent reads and writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				req := &RequestEnvelope{
					Type: "chart",
					Data: map[string]any{"id": float64(id*100 + j)},
				}
				result := &RenderResult{Format: "svg", Width: id*100 + j}

				cache.Set(req, result)
				cache.Get(req)
				cache.Stats()
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// No panics means success
	stats := cache.Stats()
	if stats.Entries == 0 {
		t.Error("Expected some entries after concurrent access")
	}
}

func TestRenderCache_KeyNormalization(t *testing.T) {
	cache := NewRenderCache(CacheConfig{
		TTL:             1 * time.Minute,
		MaxEntries:      100,
		CleanupInterval: 1 * time.Minute,
	})
	defer cache.Stop()

	// Two requests with same data but different map iteration order
	// should produce the same cache key
	req1 := &RequestEnvelope{
		Type: "bar_chart",
		Data: map[string]any{
			"a": 1.0,
			"b": 2.0,
			"c": 3.0,
		},
		Output: OutputSpec{Width: 800, Height: 600},
	}
	req2 := &RequestEnvelope{
		Type: "bar_chart",
		Data: map[string]any{
			"c": 3.0,
			"b": 2.0,
			"a": 1.0,
		},
		Output: OutputSpec{Width: 800, Height: 600},
	}

	result := &RenderResult{Format: "svg", Width: 800}
	cache.Set(req1, result)

	// req2 should hit the same cache entry
	got := cache.Get(req2)
	if got == nil {
		t.Error("Get(req2) = nil, want cache hit for equivalent request")
	}
}

func TestRenderCache_DefaultConfig(t *testing.T) {
	cfg := DefaultCacheConfig()

	if cfg.TTL != 5*time.Minute {
		t.Errorf("TTL = %v, want 5m", cfg.TTL)
	}
	if cfg.MaxEntries != 1000 {
		t.Errorf("MaxEntries = %d, want 1000", cfg.MaxEntries)
	}
	if cfg.CleanupInterval != 1*time.Minute {
		t.Errorf("CleanupInterval = %v, want 1m", cfg.CleanupInterval)
	}
}

func TestRenderCache_TotalBytes(t *testing.T) {
	cache := NewRenderCache(CacheConfig{
		TTL:             1 * time.Minute,
		MaxEntries:      100,
		CleanupInterval: 1 * time.Minute,
	})
	defer cache.Stop()

	req := &RequestEnvelope{
		Type: "bar_chart",
		Data: map[string]any{"values": []any{1.0}},
	}
	result := &RenderResult{
		SVG: &SVGDocument{
			Content: []byte("<svg>some content here</svg>"),
		},
		PNG:    []byte{0x89, 0x50, 0x4E, 0x47}, // PNG header
		Format: "svg",
	}

	cache.Set(req, result)
	stats := cache.Stats()

	expectedBytes := int64(len(result.SVG.Content) + len(result.PNG))
	if stats.TotalBytes != expectedBytes {
		t.Errorf("TotalBytes = %d, want %d", stats.TotalBytes, expectedBytes)
	}
}

func TestCacheEntry_IsExpired(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{"not expired", now.Add(1 * time.Hour), false},
		{"expired", now.Add(-1 * time.Hour), true},
		{"just expired", now.Add(-1 * time.Millisecond), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &CacheEntry{ExpiresAt: tt.expiresAt}
			if got := entry.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestRenderCache_EvictionOrder tests that eviction removes oldest entries first (FIFO order).
// This verifies the O(1) linked-list based eviction works correctly.
func TestRenderCache_EvictionOrder(t *testing.T) {
	cache := NewRenderCache(CacheConfig{
		TTL:             1 * time.Hour, // Long TTL to prevent expiry
		MaxEntries:      3,
		CleanupInterval: 1 * time.Hour,
	})
	defer cache.Stop()

	// Helper to create request with unique ID
	makeReq := func(id int) *RequestEnvelope {
		return &RequestEnvelope{
			Type: "chart",
			Data: map[string]any{"id": float64(id)},
		}
	}
	makeResult := func(id int) *RenderResult {
		return &RenderResult{Format: "svg", Width: id}
	}

	// Add entries 1, 2, 3 (cache full)
	cache.Set(makeReq(1), makeResult(1))
	cache.Set(makeReq(2), makeResult(2))
	cache.Set(makeReq(3), makeResult(3))

	// Verify all exist
	if cache.Get(makeReq(1)) == nil {
		t.Error("Entry 1 should exist before eviction")
	}
	if cache.Get(makeReq(2)) == nil {
		t.Error("Entry 2 should exist before eviction")
	}
	if cache.Get(makeReq(3)) == nil {
		t.Error("Entry 3 should exist before eviction")
	}

	// Add entry 4 - should evict entry 1 (oldest)
	cache.Set(makeReq(4), makeResult(4))

	// Entry 1 should be evicted (oldest)
	if cache.Get(makeReq(1)) != nil {
		t.Error("Entry 1 should have been evicted (it was oldest)")
	}

	// Entries 2, 3, 4 should still exist
	if cache.Get(makeReq(2)) == nil {
		t.Error("Entry 2 should still exist after eviction")
	}
	if cache.Get(makeReq(3)) == nil {
		t.Error("Entry 3 should still exist after eviction")
	}
	if cache.Get(makeReq(4)) == nil {
		t.Error("Entry 4 should exist (just added)")
	}

	// Add entry 5 - should evict entry 2 (now oldest)
	cache.Set(makeReq(5), makeResult(5))

	if cache.Get(makeReq(2)) != nil {
		t.Error("Entry 2 should have been evicted (it was oldest)")
	}
	if cache.Get(makeReq(3)) == nil {
		t.Error("Entry 3 should still exist")
	}
	if cache.Get(makeReq(4)) == nil {
		t.Error("Entry 4 should still exist")
	}
	if cache.Get(makeReq(5)) == nil {
		t.Error("Entry 5 should exist (just added)")
	}

	stats := cache.Stats()
	if stats.Entries != 3 {
		t.Errorf("Expected 3 entries, got %d", stats.Entries)
	}
	if stats.Evictions != 2 {
		t.Errorf("Expected 2 evictions, got %d", stats.Evictions)
	}
}

// TestComputeKey_Consistency ensures the same request always produces the same key.
func TestComputeKey_Consistency(t *testing.T) {
	cache := NewRenderCache(DefaultCacheConfig())
	defer cache.Stop()

	req := &RequestEnvelope{
		Type:     "bar_chart",
		Title:    "Sales Report",
		Subtitle: "Q4 2024",
		Data: map[string]any{
			"labels": []any{"Jan", "Feb", "Mar"},
			"values": []any{100.5, 200.0, 150.75},
			"nested": map[string]any{
				"key1": "value1",
				"key2": 42.0,
			},
		},
		Output: OutputSpec{
			Format: "svg",
			Width:  800,
			Height: 600,
			Scale:  1.5,
		},
		Style: StyleSpec{
			Palette:    PaletteSpec{Colors: []string{"#ff0000", "#00ff00"}},
			FontFamily: "Arial",
			Background: "#ffffff",
			ShowLegend: true,
			ShowValues: false,
			ShowGrid:   true,
		},
	}

	key1 := cache.computeKey(req)
	key2 := cache.computeKey(req)
	key3 := cache.computeKey(req)

	if key1 != key2 || key2 != key3 {
		t.Errorf("computeKey produced inconsistent keys: %s, %s, %s", key1, key2, key3)
	}
}

// TestComputeKey_DifferentRequests ensures different requests produce different keys.
func TestComputeKey_DifferentRequests(t *testing.T) {
	cache := NewRenderCache(DefaultCacheConfig())
	defer cache.Stop()

	tests := []struct {
		name string
		req1 *RequestEnvelope
		req2 *RequestEnvelope
	}{
		{
			name: "different type",
			req1: &RequestEnvelope{Type: "bar_chart"},
			req2: &RequestEnvelope{Type: "pie_chart"},
		},
		{
			name: "different title",
			req1: &RequestEnvelope{Type: "bar_chart", Title: "Report A"},
			req2: &RequestEnvelope{Type: "bar_chart", Title: "Report B"},
		},
		{
			name: "different data values",
			req1: &RequestEnvelope{Type: "bar_chart", Data: map[string]any{"x": 1.0}},
			req2: &RequestEnvelope{Type: "bar_chart", Data: map[string]any{"x": 2.0}},
		},
		{
			name: "different output size",
			req1: &RequestEnvelope{Type: "bar_chart", Output: OutputSpec{Width: 800}},
			req2: &RequestEnvelope{Type: "bar_chart", Output: OutputSpec{Width: 1024}},
		},
		{
			name: "different style boolean",
			req1: &RequestEnvelope{Type: "bar_chart", Style: StyleSpec{ShowLegend: true}},
			req2: &RequestEnvelope{Type: "bar_chart", Style: StyleSpec{ShowLegend: false}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key1 := cache.computeKey(tt.req1)
			key2 := cache.computeKey(tt.req2)
			if key1 == key2 {
				t.Errorf("Different requests produced same key: %s", key1)
			}
		})
	}
}

// BenchmarkComputeKey measures the performance of cache key computation.
func BenchmarkComputeKey(b *testing.B) {
	cache := NewRenderCache(DefaultCacheConfig())
	defer cache.Stop()

	// Create a realistic request envelope
	req := &RequestEnvelope{
		Type:     "bar_chart",
		Title:    "Quarterly Sales Report",
		Subtitle: "Fiscal Year 2024",
		Data: map[string]any{
			"labels": []any{"Q1", "Q2", "Q3", "Q4"},
			"values": []any{125000.50, 150000.75, 175000.25, 200000.00},
			"colors": []any{"#3498db", "#2ecc71", "#e74c3c", "#9b59b6"},
			"metadata": map[string]any{
				"department": "Sales",
				"region":     "North America",
				"currency":   "USD",
			},
		},
		Output: OutputSpec{
			Format: "svg",
			Width:  1200,
			Height: 800,
			Scale:  2.0,
		},
		Style: StyleSpec{
			Palette:    PaletteSpec{Colors: []string{"corporate", "professional"}},
			FontFamily: "Inter",
			Background: "#ffffff",
			ShowLegend: true,
			ShowValues: true,
			ShowGrid:   true,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.computeKey(req)
	}
}

// BenchmarkComputeKey_Simple measures key computation for a minimal request.
func BenchmarkComputeKey_Simple(b *testing.B) {
	cache := NewRenderCache(DefaultCacheConfig())
	defer cache.Stop()

	req := &RequestEnvelope{
		Type: "bar_chart",
		Data: map[string]any{"values": []any{1.0, 2.0, 3.0}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.computeKey(req)
	}
}

// BenchmarkComputeKey_LargeData measures key computation for requests with large data.
func BenchmarkComputeKey_LargeData(b *testing.B) {
	cache := NewRenderCache(DefaultCacheConfig())
	defer cache.Stop()

	// Create large data array
	values := make([]any, 1000)
	labels := make([]any, 1000)
	for i := 0; i < 1000; i++ {
		values[i] = float64(i) * 1.5
		labels[i] = "Label" + string(rune('A'+i%26))
	}

	req := &RequestEnvelope{
		Type: "bar_chart",
		Data: map[string]any{
			"values": values,
			"labels": labels,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.computeKey(req)
	}
}

// TestRenderCache_EvictListConsistency verifies that the eviction list stays consistent
// with the entries map after various operations.
func TestRenderCache_EvictListConsistency(t *testing.T) {
	cache := NewRenderCache(CacheConfig{
		TTL:             1 * time.Hour,
		MaxEntries:      10,
		CleanupInterval: 1 * time.Hour,
	})
	defer cache.Stop()

	makeReq := func(id int) *RequestEnvelope {
		return &RequestEnvelope{Type: "chart", Data: map[string]any{"id": float64(id)}}
	}
	makeResult := func(id int) *RenderResult {
		return &RenderResult{Format: "svg", Width: id}
	}

	// Add entries
	for i := 0; i < 5; i++ {
		cache.Set(makeReq(i), makeResult(i))
	}

	// Invalidate some entries
	cache.Invalidate(makeReq(2))
	cache.Invalidate(makeReq(4))

	// Verify list and map sizes match
	cache.mu.RLock()
	mapSize := len(cache.entries)
	listSize := cache.evictList.Len()
	indexSize := len(cache.evictIndex)
	cache.mu.RUnlock()

	if mapSize != listSize {
		t.Errorf("entries map size (%d) != evictList size (%d)", mapSize, listSize)
	}
	if mapSize != indexSize {
		t.Errorf("entries map size (%d) != evictIndex size (%d)", mapSize, indexSize)
	}

	// Clear and verify all structures are empty
	cache.Clear()

	cache.mu.RLock()
	mapSize = len(cache.entries)
	listSize = cache.evictList.Len()
	indexSize = len(cache.evictIndex)
	cache.mu.RUnlock()

	if mapSize != 0 || listSize != 0 || indexSize != 0 {
		t.Errorf("After Clear: entries=%d, evictList=%d, evictIndex=%d (all should be 0)",
			mapSize, listSize, indexSize)
	}
}
