// Package svggen provides the SVG generation cache implementation.
package svggen

import (
	"container/list"
	"encoding/json"
	"fmt"
	"hash"
	"hash/fnv"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// Clock provides time-related functions that can be replaced in tests.
type Clock interface {
	Now() time.Time
}

// realClock uses the real system time.
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// CacheEntry represents a cached render result with metadata.
type CacheEntry struct {
	Result    *RenderResult
	CreatedAt time.Time
	ExpiresAt time.Time
	HitCount  uint64
	key       string // Cache key for this entry (used by eviction list)
}

// isExpiredAt returns true if the entry has exceeded its TTL relative to the given time.
func (e *CacheEntry) isExpiredAt(now time.Time) bool {
	return now.After(e.ExpiresAt)
}

// IsExpired returns true if the entry has exceeded its TTL.
func (e *CacheEntry) IsExpired() bool {
	return e.isExpiredAt(time.Now())
}

// CacheConfig holds configuration for the render cache.
type CacheConfig struct {
	// TTL is the time-to-live for cache entries.
	// Default: 5 minutes.
	TTL time.Duration

	// MaxEntries is the maximum number of entries to keep.
	// When exceeded, oldest entries are evicted.
	// Default: 1000.
	MaxEntries int

	// CleanupInterval is how often to run background cleanup.
	// Default: 1 minute.
	CleanupInterval time.Duration
}

// DefaultCacheConfig returns the default cache configuration.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		TTL:             5 * time.Minute,
		MaxEntries:      1000,
		CleanupInterval: 1 * time.Minute,
	}
}

// CacheStats provides statistics about cache performance.
type CacheStats struct {
	// Hits is the total number of cache hits.
	Hits uint64 `json:"hits"`

	// Misses is the total number of cache misses.
	Misses uint64 `json:"misses"`

	// Entries is the current number of entries in the cache.
	Entries int `json:"entries"`

	// Evictions is the total number of entries evicted.
	Evictions uint64 `json:"evictions"`

	// TotalBytes is the approximate total size of cached data.
	TotalBytes int64 `json:"total_bytes"`
}

// HitRate returns the cache hit rate as a percentage (0-100).
func (s CacheStats) HitRate() float64 {
	total := s.Hits + s.Misses
	if total == 0 {
		return 0
	}
	return float64(s.Hits) / float64(total) * 100
}

// RenderCache is a thread-safe in-memory cache for render results.
type RenderCache struct {
	mu       sync.RWMutex
	entries  map[string]*CacheEntry
	config   CacheConfig
	clock    Clock
	stopChan chan struct{}
	stopped  bool

	// Eviction ordering: oldest entries at back of list, newest at front.
	// This provides O(1) eviction instead of O(n) linear scan.
	evictList  *list.List               // Doubly-linked list for insertion-order tracking
	evictIndex map[string]*list.Element // Map cache key -> list element for O(1) lookup

	// Atomic counters for lock-free stats tracking
	hits      atomic.Uint64
	misses    atomic.Uint64
	evictions atomic.Uint64
}

// NewRenderCache creates a new render cache with the given configuration.
func NewRenderCache(cfg CacheConfig) *RenderCache {
	return NewRenderCacheWithClock(cfg, realClock{})
}

// NewRenderCacheWithClock creates a new render cache with a custom clock for testing.
func NewRenderCacheWithClock(cfg CacheConfig, clock Clock) *RenderCache {
	if cfg.TTL <= 0 {
		cfg.TTL = DefaultCacheConfig().TTL
	}
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = DefaultCacheConfig().MaxEntries
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = DefaultCacheConfig().CleanupInterval
	}

	cache := &RenderCache{
		entries:    make(map[string]*CacheEntry),
		config:     cfg,
		clock:      clock,
		stopChan:   make(chan struct{}),
		evictList:  list.New(),
		evictIndex: make(map[string]*list.Element),
	}

	// Start background cleanup goroutine
	go cache.cleanupLoop()

	return cache
}

// Get retrieves a cached result by request key.
// Returns nil if not found or expired.
func (c *RenderCache) Get(req *RequestEnvelope) *RenderResult {
	key := c.computeKey(req)

	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		c.misses.Add(1)
		return nil
	}

	if entry.isExpiredAt(c.clock.Now()) {
		c.mu.Lock()
		delete(c.entries, key)
		// Remove from eviction list
		if elem, ok := c.evictIndex[key]; ok {
			c.evictList.Remove(elem)
			delete(c.evictIndex, key)
		}
		c.mu.Unlock()
		c.misses.Add(1)
		return nil
	}

	// Increment hit count atomically on the entry
	atomic.AddUint64(&entry.HitCount, 1)
	c.hits.Add(1)

	return entry.Result
}

// Set stores a render result in the cache.
func (c *RenderCache) Set(req *RequestEnvelope, result *RenderResult) {
	key := c.computeKey(req)
	now := c.clock.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if entry already exists (update case)
	if _, exists := c.entries[key]; exists {
		// Remove old entry from eviction list, will re-add at front
		if elem, ok := c.evictIndex[key]; ok {
			c.evictList.Remove(elem)
			delete(c.evictIndex, key)
		}
	}

	// Evict if at capacity
	if len(c.entries) >= c.config.MaxEntries {
		c.evictOldestLocked()
	}

	entry := &CacheEntry{
		Result:    result,
		CreatedAt: now,
		ExpiresAt: now.Add(c.config.TTL),
		HitCount:  0,
		key:       key,
	}
	c.entries[key] = entry

	// Add to front of eviction list (newest entries at front)
	elem := c.evictList.PushFront(entry)
	c.evictIndex[key] = elem
}

// Invalidate removes a specific entry from the cache.
func (c *RenderCache) Invalidate(req *RequestEnvelope) {
	key := c.computeKey(req)

	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, key)

	// Remove from eviction list
	if elem, ok := c.evictIndex[key]; ok {
		c.evictList.Remove(elem)
		delete(c.evictIndex, key)
	}
}

// Clear removes all entries from the cache.
func (c *RenderCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
	c.evictList = list.New()
	c.evictIndex = make(map[string]*list.Element)
}

// Stats returns current cache statistics.
func (c *RenderCache) Stats() CacheStats {
	// Read atomic counters without holding lock
	stats := CacheStats{
		Hits:      c.hits.Load(),
		Misses:    c.misses.Load(),
		Evictions: c.evictions.Load(),
	}

	// Only hold lock briefly to count entries and calculate bytes
	c.mu.RLock()
	stats.Entries = len(c.entries)

	// Calculate approximate total bytes
	var totalBytes int64
	for _, entry := range c.entries {
		if entry.Result.SVG != nil {
			totalBytes += int64(len(entry.Result.SVG.Content))
		}
		totalBytes += int64(len(entry.Result.PNG))
		totalBytes += int64(len(entry.Result.PDF))
	}
	stats.TotalBytes = totalBytes
	c.mu.RUnlock()

	return stats
}

// Stop gracefully stops the background cleanup goroutine.
func (c *RenderCache) Stop() {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return
	}
	c.stopped = true
	c.mu.Unlock()

	close(c.stopChan)
}

// computeKey generates a cache key from a request envelope using FNV-1a hash.
// This is much faster than SHA256 and sufficient for cache key purposes.
// The key is computed by hashing field values directly rather than JSON marshaling.
func (c *RenderCache) computeKey(req *RequestEnvelope) string {
	h := fnv.New64a()

	// Hash simple string fields directly (no allocation)
	h.Write([]byte(req.Type))
	h.Write([]byte{0}) // Field separator
	h.Write([]byte(req.Title))
	h.Write([]byte{0})
	h.Write([]byte(req.Subtitle))
	h.Write([]byte{0})

	// Hash output spec fields
	h.Write([]byte(req.Output.Format))
	h.Write([]byte{0})
	h.Write([]byte(strconv.Itoa(req.Output.Width)))
	h.Write([]byte{0})
	h.Write([]byte(strconv.Itoa(req.Output.Height)))
	h.Write([]byte{0})
	h.Write([]byte(strconv.FormatFloat(req.Output.Scale, 'f', -1, 64)))
	h.Write([]byte{0})

	// Hash style spec fields
	hashAny(h, req.Style.Palette) // May be string or []string
	h.Write([]byte{0})
	h.Write([]byte(req.Style.FontFamily))
	h.Write([]byte{0})
	h.Write([]byte(req.Style.Background))
	h.Write([]byte{0})
	if req.Style.ShowLegend {
		h.Write([]byte{'1'})
	} else {
		h.Write([]byte{'0'})
	}
	if req.Style.ShowValues {
		h.Write([]byte{'1'})
	} else {
		h.Write([]byte{'0'})
	}
	if req.Style.ShowGrid {
		h.Write([]byte{'1'})
	} else {
		h.Write([]byte{'0'})
	}
	h.Write([]byte{0})

	// Hash data map - use JSON for complex nested data (sorted keys for consistency)
	// This is the only allocation, but it's necessary for map ordering consistency
	hashMapData(h, req.Data)

	return strconv.FormatUint(h.Sum64(), 16)
}

// hashAny writes a value of any type to the hash.
// Used for fields like Palette which can be string or []string.
func hashAny(h hash.Hash64, v any) {
	switch val := v.(type) {
	case string:
		h.Write([]byte(val))
	case []string:
		for _, s := range val {
			h.Write([]byte(s))
			h.Write([]byte{','})
		}
	case []any:
		for _, item := range val {
			hashAny(h, item)
			h.Write([]byte{','})
		}
	case nil:
		h.Write([]byte("nil"))
	default:
		// Fallback: use fmt for unknown types (error ignored for hash writes)
		_, _ = fmt.Fprintf(h, "%v", v)
	}
}

// hashMapData writes a map to the hash with sorted keys for consistency.
// For complex nested data, this marshals to JSON (compact form).
func hashMapData(h hash.Hash64, data map[string]any) {
	if len(data) == 0 {
		h.Write([]byte("{}"))
		return
	}

	// Sort keys for consistent ordering
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	// Write key-value pairs
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{':'})
		v := data[k]
		switch val := v.(type) {
		case string:
			h.Write([]byte(val))
		case int:
			h.Write([]byte(strconv.Itoa(val)))
		case int64:
			h.Write([]byte(strconv.FormatInt(val, 10)))
		case float64:
			h.Write([]byte(strconv.FormatFloat(val, 'f', -1, 64)))
		case bool:
			if val {
				h.Write([]byte{'t'})
			} else {
				h.Write([]byte{'f'})
			}
		case map[string]any:
			// Recursive call for nested maps
			hashMapData(h, val)
		case []any:
			// Hash array elements
			for i, item := range val {
				if i > 0 {
					h.Write([]byte{','})
				}
				hashAny(h, item)
			}
		default:
			// Fallback: marshal to JSON for complex types
			if b, err := json.Marshal(v); err == nil {
				h.Write(b)
			} else {
				_, _ = fmt.Fprintf(h, "%v", v)
			}
		}
		h.Write([]byte{0})
	}
}

// evictOldestLocked removes the oldest entry. Must be called with lock held.
// Uses O(1) list-based eviction instead of O(n) linear scan.
func (c *RenderCache) evictOldestLocked() {
	// Oldest entry is at back of list
	elem := c.evictList.Back()
	if elem == nil {
		return
	}

	entry, ok := elem.Value.(*CacheEntry)
	if !ok {
		return
	}

	// Remove from all data structures
	delete(c.entries, entry.key)
	c.evictList.Remove(elem)
	delete(c.evictIndex, entry.key)
	c.evictions.Add(1)
}

// cleanupLoop runs periodic cleanup of expired entries.
func (c *RenderCache) cleanupLoop() {
	ticker := time.NewTicker(c.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanupExpired()
		case <-c.stopChan:
			return
		}
	}
}

// cleanupExpired removes all expired entries.
func (c *RenderCache) cleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.clock.Now()
	for key, entry := range c.entries {
		if now.After(entry.ExpiresAt) {
			delete(c.entries, key)

			// Remove from eviction list
			if elem, ok := c.evictIndex[key]; ok {
				c.evictList.Remove(elem)
				delete(c.evictIndex, key)
			}

			c.evictions.Add(1)
		}
	}
}
